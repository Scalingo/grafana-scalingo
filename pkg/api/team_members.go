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
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

// GET /api/teams/:teamId/members
func (hs *HTTPServer) GetTeamMembers(c *models.ReqContext) response.Response {
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	query := models.GetTeamMembersQuery{OrgId: c.OrgId, TeamId: teamId, SignedInUser: c.SignedInUser}

	// With accesscontrol the permission check has been done at middleware layer
	// and the membership filtering will be done at DB layer based on user permissions
	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), query.OrgId, query.TeamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to list team members", err)
		}
	}

	if err := hs.SQLStore.GetTeamMembers(c.Req.Context(), &query); err != nil {
		return response.Error(500, "Failed to get Team Members", err)
	}

	filteredMembers := make([]*models.TeamMemberDTO, 0, len(query.Result))
	for _, member := range query.Result {
		if dtos.IsHiddenUser(member.Login, c.SignedInUser, hs.Cfg) {
			continue
		}

		member.AvatarUrl = dtos.GetGravatarUrl(member.Email)
		member.Labels = []string{}

		if hs.License.FeatureEnabled("teamgroupsync") && member.External {
			authProvider := GetAuthProviderLabel(member.AuthModule)
			member.Labels = append(member.Labels, authProvider)
		}

		filteredMembers = append(filteredMembers, member)
	}

	return response.JSON(http.StatusOK, filteredMembers)
}

// POST /api/teams/:teamId/members
func (hs *HTTPServer) AddTeamMember(c *models.ReqContext) response.Response {
	cmd := models.AddTeamMemberCommand{}
	var err error
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	cmd.OrgId = c.OrgId
	cmd.TeamId, err = strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), cmd.OrgId, cmd.TeamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to add team member", err)
		}
	}

	isTeamMember, err := hs.SQLStore.IsTeamMember(c.OrgId, cmd.TeamId, cmd.UserId)
	if err != nil {
		return response.Error(500, "Failed to add team member.", err)
	}
	if isTeamMember {
		return response.Error(400, "User is already added to this team", nil)
	}

	err = addOrUpdateTeamMember(c.Req.Context(), hs.teamPermissionsService, cmd.UserId, cmd.OrgId, cmd.TeamId, getPermissionName(cmd.Permission))
	if err != nil {
		return response.Error(500, "Failed to add Member to Team", err)
	}

	return response.JSON(http.StatusOK, &util.DynMap{
		"message": "Member added to Team",
	})
}

// PUT /:teamId/members/:userId
func (hs *HTTPServer) UpdateTeamMember(c *models.ReqContext) response.Response {
	cmd := models.UpdateTeamMemberCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}
	userId, err := strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}
	orgId := c.OrgId

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), orgId, teamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to update team member", err)
		}
	}

	isTeamMember, err := hs.SQLStore.IsTeamMember(orgId, teamId, userId)
	if err != nil {
		return response.Error(500, "Failed to update team member.", err)
	}
	if !isTeamMember {
		return response.Error(404, "Team member not found.", nil)
	}

	err = addOrUpdateTeamMember(c.Req.Context(), hs.teamPermissionsService, userId, orgId, teamId, getPermissionName(cmd.Permission))
	if err != nil {
		return response.Error(500, "Failed to update team member.", err)
	}
	return response.Success("Team member updated")
}

func getPermissionName(permission models.PermissionType) string {
	permissionName := permission.String()
	// Team member permission is 0, which maps to an empty string.
	// However, we want the team permission service to display "Member" for team members. This is a hack to make it work.
	if permissionName == "" {
		permissionName = "Member"
	}
	return permissionName
}

// DELETE /api/teams/:teamId/members/:userId
func (hs *HTTPServer) RemoveTeamMember(c *models.ReqContext) response.Response {
	orgId := c.OrgId
	teamId, err := strconv.ParseInt(web.Params(c.Req)[":teamId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "teamId is invalid", err)
	}
	userId, err := strconv.ParseInt(web.Params(c.Req)[":userId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "userId is invalid", err)
	}

	if hs.AccessControl.IsDisabled() {
		if err := hs.teamGuardian.CanAdmin(c.Req.Context(), orgId, teamId, c.SignedInUser); err != nil {
			return response.Error(403, "Not allowed to remove team member", err)
		}
	}

	teamIDString := strconv.FormatInt(teamId, 10)
	if _, err := hs.teamPermissionsService.SetUserPermission(c.Req.Context(), orgId, accesscontrol.User{ID: userId}, teamIDString, ""); err != nil {
		if errors.Is(err, models.ErrTeamNotFound) {
			return response.Error(404, "Team not found", nil)
		}

		if errors.Is(err, models.ErrTeamMemberNotFound) {
			return response.Error(404, "Team member not found", nil)
		}

		return response.Error(500, "Failed to remove Member from Team", err)
	}
	return response.Success("Team Member removed")
}

// addOrUpdateTeamMember adds or updates a team member.
//
// Stubbable by tests.
var addOrUpdateTeamMember = func(ctx context.Context, resourcePermissionService accesscontrol.TeamPermissionsService, userID, orgID, teamID int64, permission string) error {
	teamIDString := strconv.FormatInt(teamID, 10)
	if _, err := resourcePermissionService.SetUserPermission(ctx, orgID, accesscontrol.User{ID: userID}, teamIDString, permission); err != nil {
		return fmt.Errorf("failed setting permissions for user %d in team %d: %w", userID, teamID, err)
	}
	return nil
}
