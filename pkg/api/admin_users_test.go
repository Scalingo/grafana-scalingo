package api

import (
	"testing"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/auth"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAdminApiEndpoint(t *testing.T) {
	role := m.ROLE_ADMIN
	Convey("Given a server admin attempts to remove themself as an admin", t, func() {

		updateCmd := dtos.AdminUpdateUserPermissionsForm{
			IsGrafanaAdmin: false,
		}

		bus.AddHandler("test", func(cmd *m.UpdateUserPermissionsCommand) error {
			return m.ErrLastGrafanaAdmin
		})

		putAdminScenario("When calling PUT on", "/api/admin/users/1/permissions", "/api/admin/users/:id/permissions", role, updateCmd, func(sc *scenarioContext) {
			sc.fakeReqWithParams("PUT", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 400)
		})
	})

	Convey("When a server admin attempts to logout himself from all devices", t, func() {
		bus.AddHandler("test", func(cmd *m.GetUserByIdQuery) error {
			cmd.Result = &m.User{Id: TestUserID}
			return nil
		})

		adminLogoutUserScenario("Should not be allowed when calling POST on", "/api/admin/users/1/logout", "/api/admin/users/:id/logout", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 400)
		})
	})

	Convey("When a server admin attempts to logout a non-existing user from all devices", t, func() {
		userId := int64(0)
		bus.AddHandler("test", func(cmd *m.GetUserByIdQuery) error {
			userId = cmd.Id
			return m.ErrUserNotFound
		})

		adminLogoutUserScenario("Should return not found when calling POST on", "/api/admin/users/200/logout", "/api/admin/users/:id/logout", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 404)
			So(userId, ShouldEqual, 200)
		})
	})

	Convey("When a server admin attempts to revoke an auth token for a non-existing user", t, func() {
		userId := int64(0)
		bus.AddHandler("test", func(cmd *m.GetUserByIdQuery) error {
			userId = cmd.Id
			return m.ErrUserNotFound
		})

		cmd := m.RevokeAuthTokenCmd{AuthTokenId: 2}

		adminRevokeUserAuthTokenScenario("Should return not found when calling POST on", "/api/admin/users/200/revoke-auth-token", "/api/admin/users/:id/revoke-auth-token", cmd, func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 404)
			So(userId, ShouldEqual, 200)
		})
	})

	Convey("When a server admin gets auth tokens for a non-existing user", t, func() {
		userId := int64(0)
		bus.AddHandler("test", func(cmd *m.GetUserByIdQuery) error {
			userId = cmd.Id
			return m.ErrUserNotFound
		})

		adminGetUserAuthTokensScenario("Should return not found when calling GET on", "/api/admin/users/200/auth-tokens", "/api/admin/users/:id/auth-tokens", func(sc *scenarioContext) {
			sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 404)
			So(userId, ShouldEqual, 200)
		})
	})

	Convey("When a server admin attempts to enable/disable a nonexistent user", t, func() {
		var userId int64
		isDisabled := false
		bus.AddHandler("test", func(cmd *m.GetAuthInfoQuery) error {
			return m.ErrUserNotFound
		})

		bus.AddHandler("test", func(cmd *m.DisableUserCommand) error {
			userId = cmd.UserId
			isDisabled = cmd.IsDisabled
			return m.ErrUserNotFound
		})

		adminDisableUserScenario("Should return user not found on a POST request", "enable", "/api/admin/users/42/enable", "/api/admin/users/:id/enable", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()

			So(sc.resp.Code, ShouldEqual, 404)
			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			So(err, ShouldBeNil)

			So(respJSON.Get("message").MustString(), ShouldEqual, "User not found")

			So(userId, ShouldEqual, 42)
			So(isDisabled, ShouldEqual, false)
		})

		adminDisableUserScenario("Should return user not found on a POST request", "disable", "/api/admin/users/42/disable", "/api/admin/users/:id/disable", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()

			So(sc.resp.Code, ShouldEqual, 404)
			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			So(err, ShouldBeNil)

			So(respJSON.Get("message").MustString(), ShouldEqual, "User not found")

			So(userId, ShouldEqual, 42)
			So(isDisabled, ShouldEqual, true)
		})
	})

	Convey("When a server admin attempts to disable/enable external user", t, func() {
		userId := int64(0)
		bus.AddHandler("test", func(cmd *m.GetAuthInfoQuery) error {
			userId = cmd.UserId
			return nil
		})

		adminDisableUserScenario("Should return Could not disable external user error", "disable", "/api/admin/users/42/disable", "/api/admin/users/:id/disable", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 500)

			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			So(err, ShouldBeNil)
			So(respJSON.Get("message").MustString(), ShouldEqual, "Could not disable external user")

			So(userId, ShouldEqual, 42)
		})

		adminDisableUserScenario("Should return Could not enable external user error", "enable", "/api/admin/users/42/enable", "/api/admin/users/:id/enable", func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			So(sc.resp.Code, ShouldEqual, 500)

			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			So(err, ShouldBeNil)
			So(respJSON.Get("message").MustString(), ShouldEqual, "Could not enable external user")

			So(userId, ShouldEqual, 42)
		})
	})

	Convey("When a server admin attempts to delete a nonexistent user", t, func() {
		var userId int64
		bus.AddHandler("test", func(cmd *m.DeleteUserCommand) error {
			userId = cmd.UserId
			return m.ErrUserNotFound
		})

		adminDeleteUserScenario("Should return user not found error", "/api/admin/users/42", "/api/admin/users/:id", func(sc *scenarioContext) {
			sc.fakeReqWithParams("DELETE", sc.url, map[string]string{}).exec()

			So(sc.resp.Code, ShouldEqual, 404)

			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			So(err, ShouldBeNil)
			So(respJSON.Get("message").MustString(), ShouldEqual, "User not found")

			So(userId, ShouldEqual, 42)
		})
	})
}

func putAdminScenario(desc string, url string, routePattern string, role m.RoleType, cmd dtos.AdminUpdateUserPermissionsForm, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		sc := setupScenarioContext(url)
		sc.defaultHandler = Wrap(func(c *m.ReqContext) {
			sc.context = c
			sc.context.UserId = TestUserID
			sc.context.OrgId = TestOrgID
			sc.context.OrgRole = role

			AdminUpdateUserPermissions(c, cmd)
		})

		sc.m.Put(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminLogoutUserScenario(desc string, url string, routePattern string, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		hs := HTTPServer{
			Bus:              bus.GetBus(),
			AuthTokenService: auth.NewFakeUserAuthTokenService(),
		}

		sc := setupScenarioContext(url)
		sc.defaultHandler = Wrap(func(c *m.ReqContext) Response {
			sc.context = c
			sc.context.UserId = TestUserID
			sc.context.OrgId = TestOrgID
			sc.context.OrgRole = m.ROLE_ADMIN

			return hs.AdminLogoutUser(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminRevokeUserAuthTokenScenario(desc string, url string, routePattern string, cmd m.RevokeAuthTokenCmd, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		hs := HTTPServer{
			Bus:              bus.GetBus(),
			AuthTokenService: fakeAuthTokenService,
		}

		sc := setupScenarioContext(url)
		sc.userAuthTokenService = fakeAuthTokenService
		sc.defaultHandler = Wrap(func(c *m.ReqContext) Response {
			sc.context = c
			sc.context.UserId = TestUserID
			sc.context.OrgId = TestOrgID
			sc.context.OrgRole = m.ROLE_ADMIN

			return hs.AdminRevokeUserAuthToken(c, cmd)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminGetUserAuthTokensScenario(desc string, url string, routePattern string, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		hs := HTTPServer{
			Bus:              bus.GetBus(),
			AuthTokenService: fakeAuthTokenService,
		}

		sc := setupScenarioContext(url)
		sc.userAuthTokenService = fakeAuthTokenService
		sc.defaultHandler = Wrap(func(c *m.ReqContext) Response {
			sc.context = c
			sc.context.UserId = TestUserID
			sc.context.OrgId = TestOrgID
			sc.context.OrgRole = m.ROLE_ADMIN

			return hs.AdminGetUserAuthTokens(c)
		})

		sc.m.Get(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminDisableUserScenario(desc string, action string, url string, routePattern string, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		hs := HTTPServer{
			Bus:              bus.GetBus(),
			AuthTokenService: fakeAuthTokenService,
		}

		sc := setupScenarioContext(url)
		sc.defaultHandler = Wrap(func(c *m.ReqContext) Response {
			sc.context = c
			sc.context.UserId = TestUserID

			if action == "enable" {
				return AdminEnableUser(c)
			}

			return hs.AdminDisableUser(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminDeleteUserScenario(desc string, url string, routePattern string, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		sc := setupScenarioContext(url)
		sc.defaultHandler = Wrap(func(c *m.ReqContext) {
			sc.context = c
			sc.context.UserId = TestUserID

			AdminDeleteUser(c)
		})

		sc.m.Delete(routePattern, sc.defaultHandler)

		fn(sc)
	})
}
