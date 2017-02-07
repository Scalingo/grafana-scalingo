package api

import (
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

// GET /api/user  (current authenticated user)
func GetSignedInUser(c *middleware.Context) Response {
	return getUserUserProfile(c.UserId)
}

// GET /api/user/:id
func GetUserById(c *middleware.Context) Response {
	return getUserUserProfile(c.ParamsInt64(":id"))
}

func getUserUserProfile(userId int64) Response {
	query := m.GetUserProfileQuery{UserId: userId}

	if err := bus.Dispatch(&query); err != nil {
		return ApiError(500, "Failed to get user", err)
	}

	return Json(200, query.Result)
}

// POST /api/user
func UpdateSignedInUser(c *middleware.Context, cmd m.UpdateUserCommand) Response {
	if setting.AuthProxyEnabled {
		if setting.AuthProxyHeaderProperty == "email" && cmd.Email != c.Email {
			return ApiError(400, "Not allowed to change email when auth proxy is using email property", nil)
		}
		if setting.AuthProxyHeaderProperty == "username" && cmd.Login != c.Login {
			return ApiError(400, "Not allowed to change username when auth proxy is using username property", nil)
		}
	}
	cmd.UserId = c.UserId
	return handleUpdateUser(cmd)
}

// POST /api/users/:id
func UpdateUser(c *middleware.Context, cmd m.UpdateUserCommand) Response {
	cmd.UserId = c.ParamsInt64(":id")
	return handleUpdateUser(cmd)
}

//POST /api/users/:id/using/:orgId
func UpdateUserActiveOrg(c *middleware.Context) Response {
	userId := c.ParamsInt64(":id")
	orgId := c.ParamsInt64(":orgId")

	if !validateUsingOrg(userId, orgId) {
		return ApiError(401, "Not a valid organization", nil)
	}

	cmd := m.SetUsingOrgCommand{UserId: userId, OrgId: orgId}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed change active organization", err)
	}

	return ApiSuccess("Active organization changed")
}

func handleUpdateUser(cmd m.UpdateUserCommand) Response {
	if len(cmd.Login) == 0 {
		cmd.Login = cmd.Email
		if len(cmd.Login) == 0 {
			return ApiError(400, "Validation error, need specify either username or email", nil)
		}
	}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "failed to update user", err)
	}

	return ApiSuccess("User updated")
}

// GET /api/user/orgs
func GetSignedInUserOrgList(c *middleware.Context) Response {
	return getUserOrgList(c.UserId)
}

// GET /api/user/:id/orgs
func GetUserOrgList(c *middleware.Context) Response {
	return getUserOrgList(c.ParamsInt64(":id"))
}

func getUserOrgList(userId int64) Response {
	query := m.GetUserOrgListQuery{UserId: userId}

	if err := bus.Dispatch(&query); err != nil {
		return ApiError(500, "Faile to get user organziations", err)
	}

	return Json(200, query.Result)
}

func validateUsingOrg(userId int64, orgId int64) bool {
	query := m.GetUserOrgListQuery{UserId: userId}

	if err := bus.Dispatch(&query); err != nil {
		return false
	}

	// validate that the org id in the list
	valid := false
	for _, other := range query.Result {
		if other.OrgId == orgId {
			valid = true
		}
	}

	return valid
}

// POST /api/user/using/:id
func UserSetUsingOrg(c *middleware.Context) Response {
	orgId := c.ParamsInt64(":id")

	if !validateUsingOrg(c.UserId, orgId) {
		return ApiError(401, "Not a valid organization", nil)
	}

	cmd := m.SetUsingOrgCommand{UserId: c.UserId, OrgId: orgId}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed change active organization", err)
	}

	return ApiSuccess("Active organization changed")
}

// GET /profile/switch-org/:id
func ChangeActiveOrgAndRedirectToHome(c *middleware.Context) {
	orgId := c.ParamsInt64(":id")

	if !validateUsingOrg(c.UserId, orgId) {
		NotFoundHandler(c)
	}

	cmd := m.SetUsingOrgCommand{UserId: c.UserId, OrgId: orgId}

	if err := bus.Dispatch(&cmd); err != nil {
		NotFoundHandler(c)
	}

	c.Redirect(setting.AppSubUrl + "/")
}

func ChangeUserPassword(c *middleware.Context, cmd m.ChangeUserPasswordCommand) Response {
	if setting.LdapEnabled || setting.AuthProxyEnabled {
		return ApiError(400, "Not allowed to change password when LDAP or Auth Proxy is enabled", nil)
	}

	userQuery := m.GetUserByIdQuery{Id: c.UserId}

	if err := bus.Dispatch(&userQuery); err != nil {
		return ApiError(500, "Could not read user from database", err)
	}

	passwordHashed := util.EncodePassword(cmd.OldPassword, userQuery.Result.Salt)
	if passwordHashed != userQuery.Result.Password {
		return ApiError(401, "Invalid old password", nil)
	}

	password := m.Password(cmd.NewPassword)
	if password.IsWeak() {
		return ApiError(400, "New password is too short", nil)
	}

	cmd.UserId = c.UserId
	cmd.NewPassword = util.EncodePassword(cmd.NewPassword, userQuery.Result.Salt)

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed to change user password", err)
	}

	return ApiSuccess("User password changed")
}

// GET /api/users
func SearchUsers(c *middleware.Context) Response {
	query := m.SearchUsersQuery{Query: "", Page: 0, Limit: 1000}
	if err := bus.Dispatch(&query); err != nil {
		return ApiError(500, "Failed to fetch users", err)
	}

	return Json(200, query.Result)
}

func SetHelpFlag(c *middleware.Context) Response {
	flag := c.ParamsInt64(":id")

	bitmask := &c.HelpFlags1
	bitmask.AddFlag(m.HelpFlags1(flag))

	cmd := m.SetUserHelpFlagCommand{
		UserId:     c.UserId,
		HelpFlags1: *bitmask,
	}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed to update help flag", err)
	}

	return Json(200, &util.DynMap{"message": "Help flag set", "helpFlags1": cmd.HelpFlags1})
}

func ClearHelpFlags(c *middleware.Context) Response {
	cmd := m.SetUserHelpFlagCommand{
		UserId:     c.UserId,
		HelpFlags1: m.HelpFlags1(0),
	}

	if err := bus.Dispatch(&cmd); err != nil {
		return ApiError(500, "Failed to update help flag", err)
	}

	return Json(200, &util.DynMap{"message": "Help flag set", "helpFlags1": cmd.HelpFlags1})
}
