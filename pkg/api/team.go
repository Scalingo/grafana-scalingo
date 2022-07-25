package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

// POST /api/teams
func (hs *HTTPServer) CreateTeam(c *models.ReqContext) response.Response {
	cmd := models.CreateTeamCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	accessControlEnabled := !hs.AccessControl.IsDisabled()
	if !accessControlEnabled && c.OrgRole == models.ROLE_VIEWER {
		return response.Error(403, "Not allowed to create team.", nil)
	}

	team, err := hs.SQLStore.CreateTeam(cmd.Name, cmd.Email, c.OrgId)
	if err != nil {
		if errors.Is(err, models.ErrTeamNameTaken) {
			return response.Error(409, "Team name taken", err)
		}
		return response.Error(500, "Failed to create Team", err)
	}

	if accessControlEnabled || (c.OrgRole == models.ROLE_EDITOR && hs.Cfg.EditorsCanAdmin) {
		// if the request is authenticated using API tokens
		// the SignedInUser is an empty struct therefore
		// an additional check whether it is an actual user is required
		if c.SignedInUser.IsRealUser() {
			if err := addOrUpdateTeamMember(c.Req.Context(), hs.teamPermissionsService, c.SignedInUser.UserId, c.OrgId, team.Id, models.PERMISSION_ADMIN.String()); err != nil {
				c.Logger.Error("Could not add creator to team", "error", err)
			}
		} else {
			c.Logger.Warn("Could not add creator to team because is not a real user")
		}
	}
	return response.JSON(http.StatusOK, &util.DynMap{
		"teamId":  team.Id,
		"message": "Team created",
	})
}

// PUT /api/teams/:teamId
func (hs *HTTPServer) UpdateTeam(c *models.ReqContext) response.Response {
	cmd := models.UpdateTeamCommand{}
	var err error
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.OrgId
	cmd.Id, err = strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), cmd.OrgId, cmd.Id, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to update team", err)
		}
	}

	if err := hs.SQLStore.UpdateTeam(c.Req.Context(), &cmd); err != nil {
		if errors.Is(err, models.ErrTeamNameTaken) {
			return response.Error(400, "Team name taken", err)
		}
		return response.Error(500, "Failed to update Team", err)
	}

	return response.Success("Team updated")
}

// DELETE /api/teams/:teamId
func (hs *HTTPServer) DeleteTeamByID(c *models.ReqContext) response.Response {
	orgId := c.OrgId
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}
	user := c.SignedInUser

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), orgId, teamId, user); err != nil {
			return response.Error(403, "Not allowed to delete team", err)
		}
	}

	if err := hs.SQLStore.DeleteTeam(c.Req.Context(), &models.DeleteTeamCommand{OrgId: orgId, Id: teamId}); err != nil {
		if errors.Is(err, models.ErrTeamNotFound) {
			return response.Error(404, "Failed to delete Team. ID not found", nil)
		}
		return response.Error(500, "Failed to delete Team", err)
	}
	return response.Success("Team deleted")
}

// GET /api/teams/search
func (hs *HTTPServer) SearchTeams(c *models.ReqContext) response.Response {
	perPage := c.QueryInt("perpage")
	if perPage <= 0 {
		perPage = 1000
	}
	page := c.QueryInt("page")
	if page < 1 {
		page = 1
	}

	// Using accesscontrol the filtering is done based on user permissions
	userIdFilter := models.FilterIgnoreUser
	if hs.AccessControl.IsDisabled() {
		userIdFilter = userFilter(c)
	}

	query := models.SearchTeamsQuery{
		OrgId:        c.OrgId,
		Query:        c.Query("query"),
		Name:         c.Query("name"),
		UserIdFilter: userIdFilter,
		Page:         page,
		Limit:        perPage,
		SignedInUser: c.SignedInUser,
		HiddenUsers:  hs.Cfg.HiddenUsers,
	}

	if err := hs.SQLStore.SearchTeams(c.Req.Context(), &query); err != nil {
		return response.Error(500, "Failed to search Teams", err)
	}

	teamIDs := map[string]bool{}
	for _, team := range query.Result.Teams {
		team.AvatarUrl = dtos.GetGravatarUrlWithDefault(team.Email, team.Name)
		teamIDs[strconv.FormatInt(team.Id, 10)] = true
	}

	metadata := hs.getMultiAccessControlMetadata(c, c.OrgId, "teams:id:", teamIDs)
	if len(metadata) > 0 {
		for _, team := range query.Result.Teams {
			team.AccessControl = metadata[strconv.FormatInt(team.Id, 10)]
		}
	}

	query.Result.Page = page
	query.Result.PerPage = perPage

	return response.JSON(http.StatusOK, query.Result)
}

// UserFilter returns the user ID used in a filter when querying a team
// 1. If the user is a viewer or editor, this will return the user's ID.
// 2. If the user is an admin, this will return models.FilterIgnoreUser (0)
func userFilter(c *models.ReqContext) int64 {
	userIdFilter := c.SignedInUser.UserId
	if c.OrgRole == models.ROLE_ADMIN {
		userIdFilter = models.FilterIgnoreUser
	}
	return userIdFilter
}

// GET /api/teams/:teamId
func (hs *HTTPServer) GetTeamByID(c *models.ReqContext) response.Response {
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	// Using accesscontrol the filtering has already been performed at middleware layer
	userIdFilter := models.FilterIgnoreUser
	if hs.AccessControl.IsDisabled() {
		userIdFilter = userFilter(c)
	}

	query := models.GetTeamByIdQuery{
		OrgId:        c.OrgId,
		Id:           teamId,
		SignedInUser: c.SignedInUser,
		HiddenUsers:  hs.Cfg.HiddenUsers,
		UserIdFilter: userIdFilter,
	}

	if err := hs.SQLStore.GetTeamById(c.Req.Context(), &query); err != nil {
		if errors.Is(err, models.ErrTeamNotFound) {
			return response.Error(404, "Team not found", err)
		}

		return response.Error(500, "Failed to get Team", err)
	}

	// Add accesscontrol metadata
	query.Result.AccessControl = hs.getAccessControlMetadata(c, c.OrgId, "teams:id:", strconv.FormatInt(query.Result.Id, 10))

	query.Result.AvatarUrl = dtos.GetGravatarUrlWithDefault(query.Result.Email, query.Result.Name)
	return response.JSON(http.StatusOK, &query.Result)
}

// GET /api/teams/:teamId/preferences
func (hs *HTTPServer) GetTeamPreferences(c *models.ReqContext) response.Response {
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	orgId := c.OrgId

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), orgId, teamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to view team preferences.", err)
		}
	}

	return hs.getPreferencesFor(c.Req.Context(), orgId, 0, teamId)
}

// PUT /api/teams/:teamId/preferences
func (hs *HTTPServer) UpdateTeamPreferences(c *models.ReqContext) response.Response {
	dtoCmd := dtos.UpdatePrefsCmd{}
	if err := web.Bind(c.Req, &dtoCmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}

	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	orgId := c.OrgId

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), orgId, teamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to update team preferences.", err)
		}
	}

	return hs.updatePreferencesFor(c.Req.Context(), orgId, 0, teamId, &dtoCmd)
}
