package api

import (
	"github.com/go-macaron/binding"
	"github.com/grafana/grafana/pkg/api/avatar"
	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
)

func (hs *HTTPServer) registerRoutes() {
	reqSignedIn := middleware.ReqSignedIn
	reqGrafanaAdmin := middleware.ReqGrafanaAdmin
	reqEditorRole := middleware.ReqEditorRole
	reqOrgAdmin := middleware.ReqOrgAdmin
	redirectFromLegacyDashboardURL := middleware.RedirectFromLegacyDashboardURL()
	redirectFromLegacyDashboardSoloURL := middleware.RedirectFromLegacyDashboardSoloURL()
	quota := middleware.Quota
	bind := binding.Bind

	r := hs.RouteRegister

	// not logged in views
	r.Get("/", reqSignedIn, hs.Index)
	r.Get("/logout", Logout)
	r.Post("/login", quota("session"), bind(dtos.LoginCommand{}), Wrap(LoginPost))
	r.Get("/login/:name", quota("session"), OAuthLogin)
	r.Get("/login", hs.LoginView)
	r.Get("/invite/:code", hs.Index)

	// authed views
	r.Get("/profile/", reqSignedIn, hs.Index)
	r.Get("/profile/password", reqSignedIn, hs.Index)
	r.Get("/profile/switch-org/:id", reqSignedIn, hs.ChangeActiveOrgAndRedirectToHome)
	r.Get("/org/", reqSignedIn, hs.Index)
	r.Get("/org/new", reqSignedIn, hs.Index)
	r.Get("/datasources/", reqSignedIn, hs.Index)
	r.Get("/datasources/new", reqSignedIn, hs.Index)
	r.Get("/datasources/edit/*", reqSignedIn, hs.Index)
	r.Get("/org/users", reqSignedIn, hs.Index)
	r.Get("/org/users/new", reqSignedIn, hs.Index)
	r.Get("/org/users/invite", reqSignedIn, hs.Index)
	r.Get("/org/teams", reqSignedIn, hs.Index)
	r.Get("/org/teams/*", reqSignedIn, hs.Index)
	r.Get("/org/apikeys/", reqSignedIn, hs.Index)
	r.Get("/dashboard/import/", reqSignedIn, hs.Index)
	r.Get("/configuration", reqGrafanaAdmin, hs.Index)
	r.Get("/admin", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/settings", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/users", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/users/create", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/users/edit/:id", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/orgs", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/orgs/edit/:id", reqGrafanaAdmin, hs.Index)
	r.Get("/admin/stats", reqGrafanaAdmin, hs.Index)

	r.Get("/styleguide", reqSignedIn, hs.Index)

	r.Get("/plugins", reqSignedIn, hs.Index)
	r.Get("/plugins/:id/edit", reqSignedIn, hs.Index)
	r.Get("/plugins/:id/page/:page", reqSignedIn, hs.Index)

	r.Get("/d/:uid/:slug", reqSignedIn, hs.Index)
	r.Get("/d/:uid", reqSignedIn, hs.Index)
	r.Get("/dashboard/db/:slug", reqSignedIn, redirectFromLegacyDashboardURL, hs.Index)
	r.Get("/dashboard/script/*", reqSignedIn, hs.Index)
	r.Get("/dashboard-solo/snapshot/*", hs.Index)
	r.Get("/d-solo/:uid/:slug", reqSignedIn, hs.Index)
	r.Get("/dashboard-solo/db/:slug", reqSignedIn, redirectFromLegacyDashboardSoloURL, hs.Index)
	r.Get("/dashboard-solo/script/*", reqSignedIn, hs.Index)
	r.Get("/import/dashboard", reqSignedIn, hs.Index)
	r.Get("/dashboards/", reqSignedIn, hs.Index)
	r.Get("/dashboards/*", reqSignedIn, hs.Index)

	r.Get("/explore", reqEditorRole, hs.Index)

	r.Get("/playlists/", reqSignedIn, hs.Index)
	r.Get("/playlists/*", reqSignedIn, hs.Index)
	r.Get("/alerting/", reqSignedIn, hs.Index)
	r.Get("/alerting/*", reqSignedIn, hs.Index)

	// sign up
	r.Get("/signup", hs.Index)
	r.Get("/api/user/signup/options", Wrap(GetSignUpOptions))
	r.Post("/api/user/signup", quota("user"), bind(dtos.SignUpForm{}), Wrap(SignUp))
	r.Post("/api/user/signup/step2", bind(dtos.SignUpStep2Form{}), Wrap(SignUpStep2))

	// invited
	r.Get("/api/user/invite/:code", Wrap(GetInviteInfoByCode))
	r.Post("/api/user/invite/complete", bind(dtos.CompleteInviteForm{}), Wrap(CompleteInvite))

	// reset password
	r.Get("/user/password/send-reset-email", hs.Index)
	r.Get("/user/password/reset", hs.Index)

	r.Post("/api/user/password/send-reset-email", bind(dtos.SendResetPasswordEmailForm{}), Wrap(SendResetPasswordEmail))
	r.Post("/api/user/password/reset", bind(dtos.ResetUserPasswordForm{}), Wrap(ResetPassword))

	// dashboard snapshots
	r.Get("/dashboard/snapshot/*", hs.Index)
	r.Get("/dashboard/snapshots/", reqSignedIn, hs.Index)

	// api for dashboard snapshots
	r.Post("/api/snapshots/", bind(m.CreateDashboardSnapshotCommand{}), CreateDashboardSnapshot)
	r.Get("/api/snapshot/shared-options/", GetSharingOptions)
	r.Get("/api/snapshots/:key", GetDashboardSnapshot)
	r.Get("/api/snapshots-delete/:deleteKey", Wrap(DeleteDashboardSnapshotByDeleteKey))
	r.Delete("/api/snapshots/:key", reqEditorRole, Wrap(DeleteDashboardSnapshot))

	// api renew session based on remember cookie
	r.Get("/api/login/ping", quota("session"), LoginAPIPing)

	// authed api
	r.Group("/api", func(apiRoute routing.RouteRegister) {

		// user (signed in)
		apiRoute.Group("/user", func(userRoute routing.RouteRegister) {
			userRoute.Get("/", Wrap(GetSignedInUser))
			userRoute.Put("/", bind(m.UpdateUserCommand{}), Wrap(UpdateSignedInUser))
			userRoute.Post("/using/:id", Wrap(UserSetUsingOrg))
			userRoute.Get("/orgs", Wrap(GetSignedInUserOrgList))
			userRoute.Get("/teams", Wrap(GetSignedInUserTeamList))

			userRoute.Post("/stars/dashboard/:id", Wrap(StarDashboard))
			userRoute.Delete("/stars/dashboard/:id", Wrap(UnstarDashboard))

			userRoute.Put("/password", bind(m.ChangeUserPasswordCommand{}), Wrap(ChangeUserPassword))
			userRoute.Get("/quotas", Wrap(GetUserQuotas))
			userRoute.Put("/helpflags/:id", Wrap(SetHelpFlag))
			// For dev purpose
			userRoute.Get("/helpflags/clear", Wrap(ClearHelpFlags))

			userRoute.Get("/preferences", Wrap(GetUserPreferences))
			userRoute.Put("/preferences", bind(dtos.UpdatePrefsCmd{}), Wrap(UpdateUserPreferences))
		})

		// users (admin permission required)
		apiRoute.Group("/users", func(usersRoute routing.RouteRegister) {
			usersRoute.Get("/", Wrap(SearchUsers))
			usersRoute.Get("/search", Wrap(SearchUsersWithPaging))
			usersRoute.Get("/:id", Wrap(GetUserByID))
			usersRoute.Get("/:id/teams", Wrap(GetUserTeams))
			usersRoute.Get("/:id/orgs", Wrap(GetUserOrgList))
			// query parameters /users/lookup?loginOrEmail=admin@example.com
			usersRoute.Get("/lookup", Wrap(GetUserByLoginOrEmail))
			usersRoute.Put("/:id", bind(m.UpdateUserCommand{}), Wrap(UpdateUser))
			usersRoute.Post("/:id/using/:orgId", Wrap(UpdateUserActiveOrg))
		}, reqGrafanaAdmin)

		// team (admin permission required)
		apiRoute.Group("/teams", func(teamsRoute routing.RouteRegister) {
			teamsRoute.Post("/", bind(m.CreateTeamCommand{}), Wrap(CreateTeam))
			teamsRoute.Put("/:teamId", bind(m.UpdateTeamCommand{}), Wrap(UpdateTeam))
			teamsRoute.Delete("/:teamId", Wrap(DeleteTeamByID))
			teamsRoute.Get("/:teamId/members", Wrap(GetTeamMembers))
			teamsRoute.Post("/:teamId/members", bind(m.AddTeamMemberCommand{}), Wrap(AddTeamMember))
			teamsRoute.Delete("/:teamId/members/:userId", Wrap(RemoveTeamMember))
			teamsRoute.Get("/:teamId/preferences", Wrap(GetTeamPreferences))
			teamsRoute.Put("/:teamId/preferences", bind(dtos.UpdatePrefsCmd{}), Wrap(UpdateTeamPreferences))
		}, reqOrgAdmin)

		// team without requirement of user to be org admin
		apiRoute.Group("/teams", func(teamsRoute routing.RouteRegister) {
			teamsRoute.Get("/:teamId", Wrap(GetTeamByID))
			teamsRoute.Get("/search", Wrap(SearchTeams))
		})

		// org information available to all users.
		apiRoute.Group("/org", func(orgRoute routing.RouteRegister) {
			orgRoute.Get("/", Wrap(GetOrgCurrent))
			orgRoute.Get("/quotas", Wrap(GetOrgQuotas))
		})

		// current org
		apiRoute.Group("/org", func(orgRoute routing.RouteRegister) {
			orgRoute.Put("/", bind(dtos.UpdateOrgForm{}), Wrap(UpdateOrgCurrent))
			orgRoute.Put("/address", bind(dtos.UpdateOrgAddressForm{}), Wrap(UpdateOrgAddressCurrent))
			orgRoute.Post("/users", quota("user"), bind(m.AddOrgUserCommand{}), Wrap(AddOrgUserToCurrentOrg))
			orgRoute.Patch("/users/:userId", bind(m.UpdateOrgUserCommand{}), Wrap(UpdateOrgUserForCurrentOrg))
			orgRoute.Delete("/users/:userId", Wrap(RemoveOrgUserForCurrentOrg))

			// invites
			orgRoute.Get("/invites", Wrap(GetPendingOrgInvites))
			orgRoute.Post("/invites", quota("user"), bind(dtos.AddInviteForm{}), Wrap(AddOrgInvite))
			orgRoute.Patch("/invites/:code/revoke", Wrap(RevokeInvite))

			// prefs
			orgRoute.Get("/preferences", Wrap(GetOrgPreferences))
			orgRoute.Put("/preferences", bind(dtos.UpdatePrefsCmd{}), Wrap(UpdateOrgPreferences))
		}, reqOrgAdmin)

		// current org without requirement of user to be org admin
		apiRoute.Group("/org", func(orgRoute routing.RouteRegister) {
			orgRoute.Get("/users", Wrap(GetOrgUsersForCurrentOrg))
		})

		// create new org
		apiRoute.Post("/orgs", quota("org"), bind(m.CreateOrgCommand{}), Wrap(CreateOrg))

		// search all orgs
		apiRoute.Get("/orgs", reqGrafanaAdmin, Wrap(SearchOrgs))

		// orgs (admin routes)
		apiRoute.Group("/orgs/:orgId", func(orgsRoute routing.RouteRegister) {
			orgsRoute.Get("/", Wrap(GetOrgByID))
			orgsRoute.Put("/", bind(dtos.UpdateOrgForm{}), Wrap(UpdateOrg))
			orgsRoute.Put("/address", bind(dtos.UpdateOrgAddressForm{}), Wrap(UpdateOrgAddress))
			orgsRoute.Delete("/", Wrap(DeleteOrgByID))
			orgsRoute.Get("/users", Wrap(GetOrgUsers))
			orgsRoute.Post("/users", bind(m.AddOrgUserCommand{}), Wrap(AddOrgUser))
			orgsRoute.Patch("/users/:userId", bind(m.UpdateOrgUserCommand{}), Wrap(UpdateOrgUser))
			orgsRoute.Delete("/users/:userId", Wrap(RemoveOrgUser))
			orgsRoute.Get("/quotas", Wrap(GetOrgQuotas))
			orgsRoute.Put("/quotas/:target", bind(m.UpdateOrgQuotaCmd{}), Wrap(UpdateOrgQuota))
		}, reqGrafanaAdmin)

		// orgs (admin routes)
		apiRoute.Group("/orgs/name/:name", func(orgsRoute routing.RouteRegister) {
			orgsRoute.Get("/", Wrap(GetOrgByName))
		}, reqGrafanaAdmin)

		// auth api keys
		apiRoute.Group("/auth/keys", func(keysRoute routing.RouteRegister) {
			keysRoute.Get("/", Wrap(GetAPIKeys))
			keysRoute.Post("/", quota("api_key"), bind(m.AddApiKeyCommand{}), Wrap(AddAPIKey))
			keysRoute.Delete("/:id", Wrap(DeleteAPIKey))
		}, reqOrgAdmin)

		// Preferences
		apiRoute.Group("/preferences", func(prefRoute routing.RouteRegister) {
			prefRoute.Post("/set-home-dash", bind(m.SavePreferencesCommand{}), Wrap(SetHomeDashboard))
		})

		// Data sources
		apiRoute.Group("/datasources", func(datasourceRoute routing.RouteRegister) {
			datasourceRoute.Get("/", Wrap(GetDataSources))
			datasourceRoute.Post("/", quota("data_source"), bind(m.AddDataSourceCommand{}), Wrap(AddDataSource))
			datasourceRoute.Put("/:id", bind(m.UpdateDataSourceCommand{}), Wrap(UpdateDataSource))
			datasourceRoute.Delete("/:id", Wrap(DeleteDataSourceById))
			datasourceRoute.Delete("/name/:name", Wrap(DeleteDataSourceByName))
			datasourceRoute.Get("/:id", Wrap(GetDataSourceById))
			datasourceRoute.Get("/name/:name", Wrap(GetDataSourceByName))
		}, reqOrgAdmin)

		apiRoute.Get("/datasources/id/:name", Wrap(GetDataSourceIdByName), reqSignedIn)

		apiRoute.Get("/plugins", Wrap(hs.GetPluginList))
		apiRoute.Get("/plugins/:pluginId/settings", Wrap(GetPluginSettingByID))
		apiRoute.Get("/plugins/:pluginId/markdown/:name", Wrap(GetPluginMarkdown))

		apiRoute.Group("/plugins", func(pluginRoute routing.RouteRegister) {
			pluginRoute.Get("/:pluginId/dashboards/", Wrap(GetPluginDashboards))
			pluginRoute.Post("/:pluginId/settings", bind(m.UpdatePluginSettingCmd{}), Wrap(UpdatePluginSetting))
		}, reqOrgAdmin)

		apiRoute.Get("/frontend/settings/", hs.GetFrontendSettings)
		apiRoute.Any("/datasources/proxy/:id/*", reqSignedIn, hs.ProxyDataSourceRequest)
		apiRoute.Any("/datasources/proxy/:id", reqSignedIn, hs.ProxyDataSourceRequest)

		// Folders
		apiRoute.Group("/folders", func(folderRoute routing.RouteRegister) {
			folderRoute.Get("/", Wrap(GetFolders))
			folderRoute.Get("/id/:id", Wrap(GetFolderByID))
			folderRoute.Post("/", bind(m.CreateFolderCommand{}), Wrap(CreateFolder))

			folderRoute.Group("/:uid", func(folderUidRoute routing.RouteRegister) {
				folderUidRoute.Get("/", Wrap(GetFolderByUID))
				folderUidRoute.Put("/", bind(m.UpdateFolderCommand{}), Wrap(UpdateFolder))
				folderUidRoute.Delete("/", Wrap(DeleteFolder))

				folderUidRoute.Group("/permissions", func(folderPermissionRoute routing.RouteRegister) {
					folderPermissionRoute.Get("/", Wrap(GetFolderPermissionList))
					folderPermissionRoute.Post("/", bind(dtos.UpdateDashboardAclCommand{}), Wrap(UpdateFolderPermissions))
				})
			})
		})

		// Dashboard
		apiRoute.Group("/dashboards", func(dashboardRoute routing.RouteRegister) {
			dashboardRoute.Get("/uid/:uid", Wrap(GetDashboard))
			dashboardRoute.Delete("/uid/:uid", Wrap(DeleteDashboardByUID))

			dashboardRoute.Get("/db/:slug", Wrap(GetDashboard))
			dashboardRoute.Delete("/db/:slug", Wrap(DeleteDashboard))

			dashboardRoute.Post("/calculate-diff", bind(dtos.CalculateDiffOptions{}), Wrap(CalculateDashboardDiff))

			dashboardRoute.Post("/db", bind(m.SaveDashboardCommand{}), Wrap(PostDashboard))
			dashboardRoute.Get("/home", Wrap(GetHomeDashboard))
			dashboardRoute.Get("/tags", GetDashboardTags)
			dashboardRoute.Post("/import", bind(dtos.ImportDashboardCommand{}), Wrap(ImportDashboard))

			dashboardRoute.Group("/id/:dashboardId", func(dashIdRoute routing.RouteRegister) {
				dashIdRoute.Get("/versions", Wrap(GetDashboardVersions))
				dashIdRoute.Get("/versions/:id", Wrap(GetDashboardVersion))
				dashIdRoute.Post("/restore", bind(dtos.RestoreDashboardVersionCommand{}), Wrap(RestoreDashboardVersion))

				dashIdRoute.Group("/permissions", func(dashboardPermissionRoute routing.RouteRegister) {
					dashboardPermissionRoute.Get("/", Wrap(GetDashboardPermissionList))
					dashboardPermissionRoute.Post("/", bind(dtos.UpdateDashboardAclCommand{}), Wrap(UpdateDashboardPermissions))
				})
			})
		})

		// Dashboard snapshots
		apiRoute.Group("/dashboard/snapshots", func(dashboardRoute routing.RouteRegister) {
			dashboardRoute.Get("/", Wrap(SearchDashboardSnapshots))
		})

		// Playlist
		apiRoute.Group("/playlists", func(playlistRoute routing.RouteRegister) {
			playlistRoute.Get("/", Wrap(SearchPlaylists))
			playlistRoute.Get("/:id", ValidateOrgPlaylist, Wrap(GetPlaylist))
			playlistRoute.Get("/:id/items", ValidateOrgPlaylist, Wrap(GetPlaylistItems))
			playlistRoute.Get("/:id/dashboards", ValidateOrgPlaylist, Wrap(GetPlaylistDashboards))
			playlistRoute.Delete("/:id", reqEditorRole, ValidateOrgPlaylist, Wrap(DeletePlaylist))
			playlistRoute.Put("/:id", reqEditorRole, bind(m.UpdatePlaylistCommand{}), ValidateOrgPlaylist, Wrap(UpdatePlaylist))
			playlistRoute.Post("/", reqEditorRole, bind(m.CreatePlaylistCommand{}), Wrap(CreatePlaylist))
		})

		// Search
		apiRoute.Get("/search/", Search)

		// metrics
		apiRoute.Post("/tsdb/query", bind(dtos.MetricRequest{}), Wrap(hs.QueryMetrics))
		apiRoute.Get("/tsdb/testdata/scenarios", Wrap(GetTestDataScenarios))
		apiRoute.Get("/tsdb/testdata/gensql", reqGrafanaAdmin, Wrap(GenerateSQLTestData))
		apiRoute.Get("/tsdb/testdata/random-walk", Wrap(GetTestDataRandomWalk))

		apiRoute.Group("/alerts", func(alertsRoute routing.RouteRegister) {
			alertsRoute.Post("/test", bind(dtos.AlertTestCommand{}), Wrap(AlertTest))
			alertsRoute.Post("/:alertId/pause", reqEditorRole, bind(dtos.PauseAlertCommand{}), Wrap(PauseAlert))
			alertsRoute.Get("/:alertId", ValidateOrgAlert, Wrap(GetAlert))
			alertsRoute.Get("/", Wrap(GetAlerts))
			alertsRoute.Get("/states-for-dashboard", Wrap(GetAlertStatesForDashboard))
		})

		apiRoute.Get("/alert-notifications", Wrap(GetAlertNotifications))
		apiRoute.Get("/alert-notifiers", Wrap(GetAlertNotifiers))

		apiRoute.Group("/alert-notifications", func(alertNotifications routing.RouteRegister) {
			alertNotifications.Post("/test", bind(dtos.NotificationTestCommand{}), Wrap(NotificationTest))
			alertNotifications.Post("/", bind(m.CreateAlertNotificationCommand{}), Wrap(CreateAlertNotification))
			alertNotifications.Put("/:notificationId", bind(m.UpdateAlertNotificationCommand{}), Wrap(UpdateAlertNotification))
			alertNotifications.Get("/:notificationId", Wrap(GetAlertNotificationByID))
			alertNotifications.Delete("/:notificationId", Wrap(DeleteAlertNotification))
		}, reqEditorRole)

		apiRoute.Get("/annotations", Wrap(GetAnnotations))
		apiRoute.Post("/annotations/mass-delete", reqOrgAdmin, bind(dtos.DeleteAnnotationsCmd{}), Wrap(DeleteAnnotations))

		apiRoute.Group("/annotations", func(annotationsRoute routing.RouteRegister) {
			annotationsRoute.Post("/", bind(dtos.PostAnnotationsCmd{}), Wrap(PostAnnotation))
			annotationsRoute.Delete("/:annotationId", Wrap(DeleteAnnotationByID))
			annotationsRoute.Put("/:annotationId", bind(dtos.UpdateAnnotationsCmd{}), Wrap(UpdateAnnotation))
			annotationsRoute.Delete("/region/:regionId", Wrap(DeleteAnnotationRegion))
			annotationsRoute.Post("/graphite", reqEditorRole, bind(dtos.PostGraphiteAnnotationsCmd{}), Wrap(PostGraphiteAnnotation))
		})

		// error test
		r.Get("/metrics/error", Wrap(GenerateError))

	}, reqSignedIn)

	// admin api
	r.Group("/api/admin", func(adminRoute routing.RouteRegister) {
		adminRoute.Get("/settings", AdminGetSettings)
		adminRoute.Post("/users", bind(dtos.AdminCreateUserForm{}), AdminCreateUser)
		adminRoute.Put("/users/:id/password", bind(dtos.AdminUpdateUserPasswordForm{}), AdminUpdateUserPassword)
		adminRoute.Put("/users/:id/permissions", bind(dtos.AdminUpdateUserPermissionsForm{}), AdminUpdateUserPermissions)
		adminRoute.Delete("/users/:id", AdminDeleteUser)
		adminRoute.Get("/users/:id/quotas", Wrap(GetUserQuotas))
		adminRoute.Put("/users/:id/quotas/:target", bind(m.UpdateUserQuotaCmd{}), Wrap(UpdateUserQuota))
		adminRoute.Get("/stats", AdminGetStats)
		adminRoute.Post("/pause-all-alerts", bind(dtos.PauseAllAlertsCommand{}), Wrap(PauseAllAlerts))
	}, reqGrafanaAdmin)

	// rendering
	r.Get("/render/*", reqSignedIn, hs.RenderToPng)

	// grafana.net proxy
	r.Any("/api/gnet/*", reqSignedIn, ProxyGnetRequest)

	// Gravatar service.
	avatarCacheServer := avatar.NewCacheServer()
	r.Get("/avatar/:hash", avatarCacheServer.Handler)

	// Websocket
	r.Any("/ws", hs.streamManager.Serve)

	// streams
	//r.Post("/api/streams/push", reqSignedIn, bind(dtos.StreamMessage{}), liveConn.PushToStream)
}
