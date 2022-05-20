package api

import (
	"fmt"
	"testing"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/auth"
	"github.com/grafana/grafana/pkg/services/login/loginservice"
	"github.com/grafana/grafana/pkg/services/login/logintest"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLogin         = "test@example.com"
	testPassword      = "password"
	nonExistingOrgID  = 1000
	existingTestLogin = "existing@example.com"
)

func TestAdminAPIEndpoint(t *testing.T) {
	const role = models.ROLE_ADMIN

	t.Run("Given a server admin attempts to remove themselves as an admin", func(t *testing.T) {
		updateCmd := dtos.AdminUpdateUserPermissionsForm{
			IsGrafanaAdmin: false,
		}
		mock := &mockstore.SQLStoreMock{
			ExpectedError: models.ErrLastGrafanaAdmin,
		}
		putAdminScenario(t, "When calling PUT on", "/api/admin/users/1/permissions",
			"/api/admin/users/:id/permissions", role, updateCmd, func(sc *scenarioContext) {
				sc.fakeReqWithParams("PUT", sc.url, map[string]string{}).exec()
				assert.Equal(t, 400, sc.resp.Code)
			}, mock)
	})

	t.Run("When a server admin attempts to logout himself from all devices", func(t *testing.T) {
		mock := mockstore.NewSQLStoreMock()
		adminLogoutUserScenario(t, "Should not be allowed when calling POST on",
			"/api/admin/users/1/logout", "/api/admin/users/:id/logout", func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 400, sc.resp.Code)
			}, mock)
	})

	t.Run("When a server admin attempts to logout a non-existing user from all devices", func(t *testing.T) {
		mock := &mockstore.SQLStoreMock{
			ExpectedError: models.ErrUserNotFound,
		}
		adminLogoutUserScenario(t, "Should return not found when calling POST on", "/api/admin/users/200/logout",
			"/api/admin/users/:id/logout", func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 404, sc.resp.Code)
			}, mock)
	})

	t.Run("When a server admin attempts to revoke an auth token for a non-existing user", func(t *testing.T) {
		cmd := models.RevokeAuthTokenCmd{AuthTokenId: 2}
		mock := &mockstore.SQLStoreMock{
			ExpectedError: models.ErrUserNotFound,
		}
		adminRevokeUserAuthTokenScenario(t, "Should return not found when calling POST on",
			"/api/admin/users/200/revoke-auth-token", "/api/admin/users/:id/revoke-auth-token", cmd, func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 404, sc.resp.Code)
			}, mock)
	})

	t.Run("When a server admin gets auth tokens for a non-existing user", func(t *testing.T) {
		mock := &mockstore.SQLStoreMock{
			ExpectedError: models.ErrUserNotFound,
		}
		adminGetUserAuthTokensScenario(t, "Should return not found when calling GET on",
			"/api/admin/users/200/auth-tokens", "/api/admin/users/:id/auth-tokens", func(sc *scenarioContext) {
				sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()
				assert.Equal(t, 404, sc.resp.Code)
			}, mock)
	})

	t.Run("When a server admin attempts to enable/disable a nonexistent user", func(t *testing.T) {
		adminDisableUserScenario(t, "Should return user not found on a POST request", "enable",
			"/api/admin/users/42/enable", "/api/admin/users/:id/enable", func(sc *scenarioContext) {
				store := sc.sqlStore.(*mockstore.SQLStoreMock)
				sc.authInfoService.ExpectedError = models.ErrUserNotFound
				store.ExpectedError = models.ErrUserNotFound

				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()

				assert.Equal(t, 404, sc.resp.Code)
				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)

				assert.Equal(t, "user not found", respJSON.Get("message").MustString())

				assert.Equal(t, int64(42), store.LatestUserId)
			})

		adminDisableUserScenario(t, "Should return user not found on a POST request", "disable",
			"/api/admin/users/42/disable", "/api/admin/users/:id/disable", func(sc *scenarioContext) {
				store := sc.sqlStore.(*mockstore.SQLStoreMock)
				sc.authInfoService.ExpectedError = models.ErrUserNotFound
				store.ExpectedError = models.ErrUserNotFound

				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()

				assert.Equal(t, 404, sc.resp.Code)
				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)

				assert.Equal(t, "user not found", respJSON.Get("message").MustString())

				assert.Equal(t, int64(42), store.LatestUserId)
			})
	})

	t.Run("When a server admin attempts to disable/enable external user", func(t *testing.T) {
		adminDisableUserScenario(t, "Should return Could not disable external user error", "disable",
			"/api/admin/users/42/disable", "/api/admin/users/:id/disable", func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 500, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, "Could not disable external user", respJSON.Get("message").MustString())

				assert.Equal(t, int64(42), sc.authInfoService.LatestUserID)
			})

		adminDisableUserScenario(t, "Should return Could not enable external user error", "enable",
			"/api/admin/users/42/enable", "/api/admin/users/:id/enable", func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 500, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, "Could not enable external user", respJSON.Get("message").MustString())

				userID := sc.authInfoService.LatestUserID
				assert.Equal(t, int64(42), userID)
			})
	})

	t.Run("When a server admin attempts to delete a nonexistent user", func(t *testing.T) {
		adminDeleteUserScenario(t, "Should return user not found error", "/api/admin/users/42",
			"/api/admin/users/:id", func(sc *scenarioContext) {
				sc.sqlStore.(*mockstore.SQLStoreMock).ExpectedError = models.ErrUserNotFound
				sc.authInfoService.ExpectedError = models.ErrUserNotFound
				sc.fakeReqWithParams("DELETE", sc.url, map[string]string{}).exec()
				userID := sc.sqlStore.(*mockstore.SQLStoreMock).LatestUserId

				assert.Equal(t, 404, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, "user not found", respJSON.Get("message").MustString())

				assert.Equal(t, int64(42), userID)
			})
	})

	t.Run("When a server admin attempts to create a user", func(t *testing.T) {
		t.Run("Without an organization", func(t *testing.T) {
			createCmd := dtos.AdminCreateUserForm{
				Login:    testLogin,
				Password: testPassword,
			}

			adminCreateUserScenario(t, "Should create the user", "/api/admin/users", "/api/admin/users", createCmd, func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 200, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, testUserID, respJSON.Get("id").MustInt64())
				assert.Equal(t, "User created", respJSON.Get("message").MustString())
			})
		})

		t.Run("With an organization", func(t *testing.T) {
			createCmd := dtos.AdminCreateUserForm{
				Login:    testLogin,
				Password: testPassword,
				OrgId:    testOrgID,
			}

			adminCreateUserScenario(t, "Should create the user", "/api/admin/users", "/api/admin/users", createCmd, func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 200, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, testUserID, respJSON.Get("id").MustInt64())
				assert.Equal(t, "User created", respJSON.Get("message").MustString())
			})
		})

		t.Run("With a nonexistent organization", func(t *testing.T) {
			createCmd := dtos.AdminCreateUserForm{
				Login:    testLogin,
				Password: testPassword,
				OrgId:    nonExistingOrgID,
			}

			adminCreateUserScenario(t, "Should create the user", "/api/admin/users", "/api/admin/users", createCmd, func(sc *scenarioContext) {
				sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
				assert.Equal(t, 400, sc.resp.Code)

				respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
				require.NoError(t, err)
				assert.Equal(t, "organization not found", respJSON.Get("message").MustString())
			})
		})
	})

	t.Run("When a server admin attempts to create a user with an already existing email/login", func(t *testing.T) {
		createCmd := dtos.AdminCreateUserForm{
			Login:    existingTestLogin,
			Password: testPassword,
		}

		adminCreateUserScenario(t, "Should return an error", "/api/admin/users", "/api/admin/users", createCmd, func(sc *scenarioContext) {
			sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
			assert.Equal(t, 412, sc.resp.Code)

			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			require.NoError(t, err)
			assert.Equal(t, "user already exists", respJSON.Get("error").MustString())
		})
	})
}

func putAdminScenario(t *testing.T, desc string, url string, routePattern string, role models.RoleType,
	cmd dtos.AdminUpdateUserPermissionsForm, fn scenarioFunc, sqlStore sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		hs := &HTTPServer{
			Cfg:             setting.NewCfg(),
			SQLStore:        sqlStore,
			authInfoService: &logintest.AuthInfoServiceFake{},
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.UserId = testUserID
			sc.context.OrgId = testOrgID
			sc.context.OrgRole = role

			return hs.AdminUpdateUserPermissions(c)
		})

		sc.m.Put(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminLogoutUserScenario(t *testing.T, desc string, url string, routePattern string, fn scenarioFunc, sqlStore sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		hs := HTTPServer{
			AuthTokenService: auth.NewFakeUserAuthTokenService(),
			SQLStore:         sqlStore,
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			t.Log("Route handler invoked", "url", c.Req.URL)

			sc.context = c
			sc.context.UserId = testUserID
			sc.context.OrgId = testOrgID
			sc.context.OrgRole = models.ROLE_ADMIN

			return hs.AdminLogoutUser(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminRevokeUserAuthTokenScenario(t *testing.T, desc string, url string, routePattern string, cmd models.RevokeAuthTokenCmd, fn scenarioFunc, sqlStore sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		hs := HTTPServer{
			AuthTokenService: fakeAuthTokenService,
			SQLStore:         sqlStore,
		}

		sc := setupScenarioContext(t, url)
		sc.userAuthTokenService = fakeAuthTokenService
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.UserId = testUserID
			sc.context.OrgId = testOrgID
			sc.context.OrgRole = models.ROLE_ADMIN

			return hs.AdminRevokeUserAuthToken(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminGetUserAuthTokensScenario(t *testing.T, desc string, url string, routePattern string, fn scenarioFunc, sqlStore sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		hs := HTTPServer{
			AuthTokenService: fakeAuthTokenService,
			SQLStore:         sqlStore,
		}

		sc := setupScenarioContext(t, url)
		sc.userAuthTokenService = fakeAuthTokenService
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			sc.context = c
			sc.context.UserId = testUserID
			sc.context.OrgId = testOrgID
			sc.context.OrgRole = models.ROLE_ADMIN

			return hs.AdminGetUserAuthTokens(c)
		})

		sc.m.Get(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminDisableUserScenario(t *testing.T, desc string, action string, url string, routePattern string, fn scenarioFunc) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		fakeAuthTokenService := auth.NewFakeUserAuthTokenService()

		authInfoService := &logintest.AuthInfoServiceFake{}

		hs := HTTPServer{
			SQLStore:         mockstore.NewSQLStoreMock(),
			AuthTokenService: fakeAuthTokenService,
			authInfoService:  authInfoService,
		}

		sc := setupScenarioContext(t, url)
		sc.sqlStore = hs.SQLStore
		sc.authInfoService = authInfoService
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			sc.context = c
			sc.context.UserId = testUserID

			if action == "enable" {
				return hs.AdminEnableUser(c)
			}

			return hs.AdminDisableUser(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminDeleteUserScenario(t *testing.T, desc string, url string, routePattern string, fn scenarioFunc) {
	hs := HTTPServer{
		SQLStore: mockstore.NewSQLStoreMock(),
	}
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		sc := setupScenarioContext(t, url)
		sc.sqlStore = hs.SQLStore
		sc.authInfoService = &logintest.AuthInfoServiceFake{}
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			sc.context = c
			sc.context.UserId = testUserID

			return hs.AdminDeleteUser(c)
		})

		sc.m.Delete(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func adminCreateUserScenario(t *testing.T, desc string, url string, routePattern string, cmd dtos.AdminCreateUserForm, fn scenarioFunc) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		hs := HTTPServer{
			Login: loginservice.LoginServiceMock{
				ExpectedUserForm:    cmd,
				NoExistingOrgId:     nonExistingOrgID,
				AlreadyExitingLogin: existingTestLogin,
				GeneratedUserId:     testUserID,
			},
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.UserId = testUserID

			return hs.AdminCreateUser(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}
