package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

// POST /api/org/users
func (hs *HTTPServer) AddOrgUserToCurrentOrg(c *models.ReqContext) response.Response {
	cmd := models.AddOrgUserCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.OrgId
	return hs.addOrgUserHelper(c.Req.Context(), cmd)
}

// POST /api/orgs/:orgId/users
func (hs *HTTPServer) AddOrgUser(c *models.ReqContext) response.Response {
	cmd := models.AddOrgUserCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}

	var err error
	cmd.OrgId, err = strconv.ParseInt(web.Params(c.Req)[":orgId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "orgId is invalid", err)
	}
	return hs.addOrgUserHelper(c.Req.Context(), cmd)
}

func (hs *HTTPServer) addOrgUserHelper(ctx context.Context, cmd models.AddOrgUserCommand) response.Response {
	if !cmd.Role.IsValid() {
		return response.Error(400, "Invalid role specified", nil)
	}

	userQuery := models.GetUserByLoginQuery{LoginOrEmail: cmd.LoginOrEmail}
	err := hs.SQLStore.GetUserByLogin(ctx, &userQuery)
	if err != nil {
		return response.Error(404, "User not found", nil)
	}

	userToAdd := userQuery.Result

	cmd.UserId = userToAdd.Id

	if err := hs.SQLStore.AddOrgUser(ctx, &cmd); err != nil {
		if errors.Is(err, models.ErrOrgUserAlreadyAdded) {
			return response.JSON(409, util.DynMap{
				"message": "User is already member of this organization",
				"userId":  cmd.UserId,
			})
		}
		return response.Error(500, "Could not add user to organization", err)
	}

	return response.JSON(200, util.DynMap{
		"message": "User added to organization",
		"userId":  cmd.UserId,
	})
}

// GET /api/org/users
func (hs *HTTPServer) GetOrgUsersForCurrentOrg(c *models.ReqContext) response.Response {
	result, err := hs.getOrgUsersHelper(c, &models.GetOrgUsersQuery{
		OrgId: c.OrgId,
		Query: c.Query("query"),
		Limit: c.QueryInt("limit"),
		User:  c.SignedInUser,
	}, c.SignedInUser)

	if err != nil {
		return response.Error(500, "Failed to get users for current organization", err)
	}

	return response.JSON(200, result)
}

// GET /api/org/users/lookup
func (hs *HTTPServer) GetOrgUsersForCurrentOrgLookup(c *models.ReqContext) response.Response {
	orgUsers, err := hs.getOrgUsersHelper(c, &models.GetOrgUsersQuery{
		OrgId: c.OrgId,
		Query: c.Query("query"),
		Limit: c.QueryInt("limit"),
		User:  c.SignedInUser,
	}, c.SignedInUser)

	if err != nil {
		return response.Error(500, "Failed to get users for current organization", err)
	}

	result := make([]*dtos.UserLookupDTO, 0)

	for _, u := range orgUsers {
		result = append(result, &dtos.UserLookupDTO{
			UserID:    u.UserId,
			Login:     u.Login,
			AvatarURL: u.AvatarUrl,
		})
	}

	return response.JSON(200, result)
}

// GET /api/orgs/:orgId/users
func (hs *HTTPServer) GetOrgUsers(c *models.ReqContext) response.Response {
	orgId, err := strconv.ParseInt(web.Params(c.Req)[":orgId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "orgId is invalid", err)
	}

	result, err := hs.getOrgUsersHelper(c, &models.GetOrgUsersQuery{
		OrgId: orgId,
		Query: "",
		Limit: 0,
		User:  c.SignedInUser,
	}, c.SignedInUser)

	if err != nil {
		return response.Error(500, "Failed to get users for organization", err)
	}

	return response.JSON(200, result)
}

func (hs *HTTPServer) getOrgUsersHelper(c *models.ReqContext, query *models.GetOrgUsersQuery, signedInUser *models.SignedInUser) ([]*models.OrgUserDTO, error) {
	if err := hs.SQLStore.GetOrgUsers(c.Req.Context(), query); err != nil {
		return nil, err
	}

	filteredUsers := make([]*models.OrgUserDTO, 0, len(query.Result))
	userIDs := map[string]bool{}
	for _, user := range query.Result {
		if dtos.IsHiddenUser(user.Login, signedInUser, hs.Cfg) {
			continue
		}
		user.AvatarUrl = dtos.GetGravatarUrl(user.Email)

		userIDs[fmt.Sprint(user.UserId)] = true
		filteredUsers = append(filteredUsers, user)
	}

	// Get accesscontrol metadata for users in the target org
	accessControlMetadata := hs.getMultiAccessControlMetadata(c, query.OrgId, "users:id:", userIDs)
	if len(accessControlMetadata) > 0 {
		for i := range filteredUsers {
			filteredUsers[i].AccessControl = accessControlMetadata[fmt.Sprint(filteredUsers[i].UserId)]
		}
	}

	return filteredUsers, nil
}

// SearchOrgUsersWithPaging is an HTTP handler to search for org users with paging.
// GET /api/org/users/search
func (hs *HTTPServer) SearchOrgUsersWithPaging(c *models.ReqContext) response.Response {
	ctx := c.Req.Context()
	perPage := c.QueryInt("perpage")
	if perPage <= 0 {
		perPage = 1000
	}
	page := c.QueryInt("page")

	if page < 1 {
		page = 1
	}

	query := &models.SearchOrgUsersQuery{
		OrgID: c.OrgId,
		Query: c.Query("query"),
		Page:  page,
		Limit: perPage,
		User:  c.SignedInUser,
	}

	if err := hs.SQLStore.SearchOrgUsers(ctx, query); err != nil {
		return response.Error(500, "Failed to get users for current organization", err)
	}

	filteredUsers := make([]*models.OrgUserDTO, 0, len(query.Result.OrgUsers))
	for _, user := range query.Result.OrgUsers {
		if dtos.IsHiddenUser(user.Login, c.SignedInUser, hs.Cfg) {
			continue
		}
		user.AvatarUrl = dtos.GetGravatarUrl(user.Email)

		filteredUsers = append(filteredUsers, user)
	}

	query.Result.OrgUsers = filteredUsers
	query.Result.Page = page
	query.Result.PerPage = perPage

	return response.JSON(200, query.Result)
}

// PATCH /api/org/users/:userId
func (hs *HTTPServer) UpdateOrgUserForCurrentOrg(c *models.ReqContext) response.Response {
	cmd := models.UpdateOrgUserCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.OrgId
	var err error
	cmd.UserId, err = strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}
	return hs.updateOrgUserHelper(c.Req.Context(), cmd)
}

// PATCH /api/orgs/:orgId/users/:userId
func (hs *HTTPServer) UpdateOrgUser(c *models.ReqContext) response.Response {
	cmd := models.UpdateOrgUserCommand{}
	var err error
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId, err = strconv.ParseInt(web.Params(c.Req)[":orgId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "orgId is invalid", err)
	}
	cmd.UserId, err = strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}
	return hs.updateOrgUserHelper(c.Req.Context(), cmd)
}

func (hs *HTTPServer) updateOrgUserHelper(ctx context.Context, cmd models.UpdateOrgUserCommand) response.Response {
	if !cmd.Role.IsValid() {
		return response.Error(400, "Invalid role specified", nil)
	}
	if err := hs.SQLStore.UpdateOrgUser(ctx, &cmd); err != nil {
		if errors.Is(err, models.ErrLastOrgAdmin) {
			return response.Error(400, "Cannot change role so that there is no organization admin left", nil)
		}
		return response.Error(500, "Failed update org user", err)
	}

	return response.Success("Organization user updated")
}

// DELETE /api/org/users/:userId
func (hs *HTTPServer) RemoveOrgUserForCurrentOrg(c *models.ReqContext) response.Response {
	userId, err := strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}

	return hs.removeOrgUserHelper(c.Req.Context(), &models.RemoveOrgUserCommand{
		UserId:                   userId,
		OrgId:                    c.OrgId,
		ShouldDeleteOrphanedUser: true,
	})
}

// DELETE /api/orgs/:orgId/users/:userId
func (hs *HTTPServer) RemoveOrgUser(c *models.ReqContext) response.Response {
	userId, err := strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}
	orgId, err := strconv.ParseInt(web.Params(c.Req)[":orgId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "orgId is invalid", err)
	}
	return hs.removeOrgUserHelper(c.Req.Context(), &models.RemoveOrgUserCommand{
		UserId: userId,
		OrgId:  orgId,
	})
}

func (hs *HTTPServer) removeOrgUserHelper(ctx context.Context, cmd *models.RemoveOrgUserCommand) response.Response {
	if err := hs.SQLStore.RemoveOrgUser(ctx, cmd); err != nil {
		if errors.Is(err, models.ErrLastOrgAdmin) {
			return response.Error(400, "Cannot remove last organization admin", nil)
		}
		return response.Error(500, "Failed to remove user from organization", err)
	}

	if cmd.UserWasDeleted {
		return response.Success("User deleted")
	}

	return response.Success("User removed from organization")
}
