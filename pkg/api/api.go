package api

import (
	"github.com/go-macaron/binding"
	"github.com/grafana/grafana/pkg/api/avatar"
	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
)

// Register adds http routes
func (hs *HTTPServer) registerRoutes() {
	macaronR := hs.macaron
	reqSignedIn := middleware.Auth(&middleware.AuthOptions{ReqSignedIn: true})
	reqGrafanaAdmin := middleware.Auth(&middleware.AuthOptions{ReqSignedIn: true, ReqGrafanaAdmin: true})
	reqEditorRole := middleware.RoleAuth(m.ROLE_EDITOR, m.ROLE_ADMIN)
	reqOrgAdmin := middleware.RoleAuth(m.ROLE_ADMIN)
	redirectFromLegacyDashboardURL := middleware.RedirectFromLegacyDashboardURL()
	redirectFromLegacyDashboardSoloURL := middleware.RedirectFromLegacyDashboardSoloURL()
	quota := middleware.Quota
	bind := binding.Bind

	// automatically set HEAD for every GET
	macaronR.SetAutoHead(true)

	r := hs.RouteRegister

	// not logged in views
	r.Get("/", reqSignedIn, Index)
	r.Get("/logout", Logout)
	r.Post("/login", quota("session"), bind(dtos.LoginCommand{}), wrap(LoginPost))
	r.Get("/login/:name", quota("session"), OAuthLogin)
	r.Get("/login", LoginView)
	r.Get("/invite/:code", Index)

	// authed views
	r.Get("/profile/", reqSignedIn, Index)
	r.Get("/profile/password", reqSignedIn, Index)
	r.Get("/profile/switch-org/:id", reqSignedIn, ChangeActiveOrgAndRedirectToHome)
	r.Get("/org/", reqSignedIn, Index)
	r.Get("/org/new", reqSignedIn, Index)
	r.Get("/datasources/", reqSignedIn, Index)
	r.Get("/datasources/new", reqSignedIn, Index)
	r.Get("/datasources/edit/*", reqSignedIn, Index)
	r.Get("/org/users", reqSignedIn, Index)
	r.Get("/org/users/new", reqSignedIn, Index)
	r.Get("/org/users/invite", reqSignedIn, Index)
	r.Get("/org/teams", reqSignedIn, Index)
	r.Get("/org/teams/*", reqSignedIn, Index)
	r.Get("/org/apikeys/", reqSignedIn, Index)
	r.Get("/dashboard/import/", reqSignedIn, Index)
	r.Get("/configuration", reqGrafanaAdmin, Index)
	r.Get("/admin", reqGrafanaAdmin, Index)
	r.Get("/admin/settings", reqGrafanaAdmin, Index)
	r.Get("/admin/users", reqGrafanaAdmin, Index)
	r.Get("/admin/users/create", reqGrafanaAdmin, Index)
	r.Get("/admin/users/edit/:id", reqGrafanaAdmin, Index)
	r.Get("/admin/orgs", reqGrafanaAdmin, Index)
	r.Get("/admin/orgs/edit/:id", reqGrafanaAdmin, Index)
	r.Get("/admin/stats", reqGrafanaAdmin, Index)

	r.Get("/styleguide", reqSignedIn, Index)

	r.Get("/plugins", reqSignedIn, Index)
	r.Get("/plugins/:id/edit", reqSignedIn, Index)
	r.Get("/plugins/:id/page/:page", reqSignedIn, Index)

	r.Get("/d/:uid/:slug", reqSignedIn, Index)
	r.Get("/d/:uid", reqSignedIn, Index)
	r.Get("/dashboard/db/:slug", reqSignedIn, redirectFromLegacyDashboardURL, Index)
	r.Get("/dashboard/script/*", reqSignedIn, Index)
	r.Get("/dashboard-solo/snapshot/*", Index)
	r.Get("/d-solo/:uid/:slug", reqSignedIn, Index)
	r.Get("/dashboard-solo/db/:slug", reqSignedIn, redirectFromLegacyDashboardSoloURL, Index)
	r.Get("/dashboard-solo/script/*", reqSignedIn, Index)
	r.Get("/import/dashboard", reqSignedIn, Index)
	r.Get("/dashboards/", reqSignedIn, Index)
	r.Get("/dashboards/*", reqSignedIn, Index)

	r.Get("/playlists/", reqSignedIn, Index)
	r.Get("/playlists/*", reqSignedIn, Index)
	r.Get("/alerting/", reqSignedIn, Index)
	r.Get("/alerting/*", reqSignedIn, Index)

	// sign up
	r.Get("/signup", Index)
	r.Get("/api/user/signup/options", wrap(GetSignUpOptions))
	r.Post("/api/user/signup", quota("user"), bind(dtos.SignUpForm{}), wrap(SignUp))
	r.Post("/api/user/signup/step2", bind(dtos.SignUpStep2Form{}), wrap(SignUpStep2))

	// invited
	r.Get("/api/user/invite/:code", wrap(GetInviteInfoByCode))
	r.Post("/api/user/invite/complete", bind(dtos.CompleteInviteForm{}), wrap(CompleteInvite))

	// reset password
	r.Get("/user/password/send-reset-email", Index)
	r.Get("/user/password/reset", Index)

	r.Post("/api/user/password/send-reset-email", bind(dtos.SendResetPasswordEmailForm{}), wrap(SendResetPasswordEmail))
	r.Post("/api/user/password/reset", bind(dtos.ResetUserPasswordForm{}), wrap(ResetPassword))

	// dashboard snapshots
	r.Get("/dashboard/snapshot/*", Index)
	r.Get("/dashboard/snapshots/", reqSignedIn, Index)

	// api for dashboard snapshots
	r.Post("/api/snapshots/", bind(m.CreateDashboardSnapshotCommand{}), CreateDashboardSnapshot)
	r.Get("/api/snapshot/shared-options/", GetSharingOptions)
	r.Get("/api/snapshots/:key", GetDashboardSnapshot)
	r.Get("/api/snapshots-delete/:key", reqEditorRole, wrap(DeleteDashboardSnapshot))

	// api renew session based on remember cookie
	r.Get("/api/login/ping", quota("session"), LoginAPIPing)

	// authed api
	r.Group("/api", func(apiRoute RouteRegister) {

		// user (signed in)
		apiRoute.Group("/user", func(userRoute RouteRegister) {
			userRoute.Get("/", wrap(GetSignedInUser))
			userRoute.Put("/", bind(m.UpdateUserCommand{}), wrap(UpdateSignedInUser))
			userRoute.Post("/using/:id", wrap(UserSetUsingOrg))
			userRoute.Get("/orgs", wrap(GetSignedInUserOrgList))

			userRoute.Post("/stars/dashboard/:id", wrap(StarDashboard))
			userRoute.Delete("/stars/dashboard/:id", wrap(UnstarDashboard))

			userRoute.Put("/password", bind(m.ChangeUserPasswordCommand{}), wrap(ChangeUserPassword))
			userRoute.Get("/quotas", wrap(GetUserQuotas))
			userRoute.Put("/helpflags/:id", wrap(SetHelpFlag))
			// For dev purpose
			userRoute.Get("/helpflags/clear", wrap(ClearHelpFlags))

			userRoute.Get("/preferences", wrap(GetUserPreferences))
			userRoute.Put("/preferences", bind(dtos.UpdatePrefsCmd{}), wrap(UpdateUserPreferences))
		})

		// users (admin permission required)
		apiRoute.Group("/users", func(usersRoute RouteRegister) {
			usersRoute.Get("/", wrap(SearchUsers))
			usersRoute.Get("/search", wrap(SearchUsersWithPaging))
			usersRoute.Get("/:id", wrap(GetUserByID))
			usersRoute.Get("/:id/orgs", wrap(GetUserOrgList))
			// query parameters /users/lookup?loginOrEmail=admin@example.com
			usersRoute.Get("/lookup", wrap(GetUserByLoginOrEmail))
			usersRoute.Put("/:id", bind(m.UpdateUserCommand{}), wrap(UpdateUser))
			usersRoute.Post("/:id/using/:orgId", wrap(UpdateUserActiveOrg))
		}, reqGrafanaAdmin)

		// team (admin permission required)
		apiRoute.Group("/teams", func(teamsRoute RouteRegister) {
			teamsRoute.Post("/", bind(m.CreateTeamCommand{}), wrap(CreateTeam))
			teamsRoute.Put("/:teamId", bind(m.UpdateTeamCommand{}), wrap(UpdateTeam))
			teamsRoute.Delete("/:teamId", wrap(DeleteTeamByID))
			teamsRoute.Get("/:teamId/members", wrap(GetTeamMembers))
			teamsRoute.Post("/:teamId/members", bind(m.AddTeamMemberCommand{}), wrap(AddTeamMember))
			teamsRoute.Delete("/:teamId/members/:userId", wrap(RemoveTeamMember))
		}, reqOrgAdmin)

		// team without requirement of user to be org admin
		apiRoute.Group("/teams", func(teamsRoute RouteRegister) {
			teamsRoute.Get("/:teamId", wrap(GetTeamByID))
			teamsRoute.Get("/search", wrap(SearchTeams))
		})

		// org information available to all users.
		apiRoute.Group("/org", func(orgRoute RouteRegister) {
			orgRoute.Get("/", wrap(GetOrgCurrent))
			orgRoute.Get("/quotas", wrap(GetOrgQuotas))
		})

		// current org
		apiRoute.Group("/org", func(orgRoute RouteRegister) {
			orgRoute.Put("/", bind(dtos.UpdateOrgForm{}), wrap(UpdateOrgCurrent))
			orgRoute.Put("/address", bind(dtos.UpdateOrgAddressForm{}), wrap(UpdateOrgAddressCurrent))
			orgRoute.Post("/users", quota("user"), bind(m.AddOrgUserCommand{}), wrap(AddOrgUserToCurrentOrg))
			orgRoute.Patch("/users/:userId", bind(m.UpdateOrgUserCommand{}), wrap(UpdateOrgUserForCurrentOrg))
			orgRoute.Delete("/users/:userId", wrap(RemoveOrgUserForCurrentOrg))

			// invites
			orgRoute.Get("/invites", wrap(GetPendingOrgInvites))
			orgRoute.Post("/invites", quota("user"), bind(dtos.AddInviteForm{}), wrap(AddOrgInvite))
			orgRoute.Patch("/invites/:code/revoke", wrap(RevokeInvite))

			// prefs
			orgRoute.Get("/preferences", wrap(GetOrgPreferences))
			orgRoute.Put("/preferences", bind(dtos.UpdatePrefsCmd{}), wrap(UpdateOrgPreferences))
		}, reqOrgAdmin)

		// current org without requirement of user to be org admin
		apiRoute.Group("/org", func(orgRoute RouteRegister) {
			orgRoute.Get("/users", wrap(GetOrgUsersForCurrentOrg))
		})

		// create new org
		apiRoute.Post("/orgs", quota("org"), bind(m.CreateOrgCommand{}), wrap(CreateOrg))

		// search all orgs
		apiRoute.Get("/orgs", reqGrafanaAdmin, wrap(SearchOrgs))

		// orgs (admin routes)
		apiRoute.Group("/orgs/:orgId", func(orgsRoute RouteRegister) {
			orgsRoute.Get("/", wrap(GetOrgByID))
			orgsRoute.Put("/", bind(dtos.UpdateOrgForm{}), wrap(UpdateOrg))
			orgsRoute.Put("/address", bind(dtos.UpdateOrgAddressForm{}), wrap(UpdateOrgAddress))
			orgsRoute.Delete("/", wrap(DeleteOrgByID))
			orgsRoute.Get("/users", wrap(GetOrgUsers))
			orgsRoute.Post("/users", bind(m.AddOrgUserCommand{}), wrap(AddOrgUser))
			orgsRoute.Patch("/users/:userId", bind(m.UpdateOrgUserCommand{}), wrap(UpdateOrgUser))
			orgsRoute.Delete("/users/:userId", wrap(RemoveOrgUser))
			orgsRoute.Get("/quotas", wrap(GetOrgQuotas))
			orgsRoute.Put("/quotas/:target", bind(m.UpdateOrgQuotaCmd{}), wrap(UpdateOrgQuota))
		}, reqGrafanaAdmin)

		// orgs (admin routes)
		apiRoute.Group("/orgs/name/:name", func(orgsRoute RouteRegister) {
			orgsRoute.Get("/", wrap(GetOrgByName))
		}, reqGrafanaAdmin)

		// auth api keys
		apiRoute.Group("/auth/keys", func(keysRoute RouteRegister) {
			keysRoute.Get("/", wrap(GetAPIKeys))
			keysRoute.Post("/", quota("api_key"), bind(m.AddApiKeyCommand{}), wrap(AddAPIKey))
			keysRoute.Delete("/:id", wrap(DeleteAPIKey))
		}, reqOrgAdmin)

		// Preferences
		apiRoute.Group("/preferences", func(prefRoute RouteRegister) {
			prefRoute.Post("/set-home-dash", bind(m.SavePreferencesCommand{}), wrap(SetHomeDashboard))
		})

		// Data sources
		apiRoute.Group("/datasources", func(datasourceRoute RouteRegister) {
			datasourceRoute.Get("/", wrap(GetDataSources))
			datasourceRoute.Post("/", quota("data_source"), bind(m.AddDataSourceCommand{}), wrap(AddDataSource))
			datasourceRoute.Put("/:id", bind(m.UpdateDataSourceCommand{}), wrap(UpdateDataSource))
			datasourceRoute.Delete("/:id", wrap(DeleteDataSourceByID))
			datasourceRoute.Delete("/name/:name", wrap(DeleteDataSourceByName))
			datasourceRoute.Get("/:id", wrap(GetDataSourceByID))
			datasourceRoute.Get("/name/:name", wrap(GetDataSourceByName))
		}, reqOrgAdmin)

		apiRoute.Get("/datasources/id/:name", wrap(GetDataSourceIDByName), reqSignedIn)

		apiRoute.Get("/plugins", wrap(GetPluginList))
		apiRoute.Get("/plugins/:pluginId/settings", wrap(GetPluginSettingByID))
		apiRoute.Get("/plugins/:pluginId/markdown/:name", wrap(GetPluginMarkdown))

		apiRoute.Group("/plugins", func(pluginRoute RouteRegister) {
			pluginRoute.Get("/:pluginId/dashboards/", wrap(GetPluginDashboards))
			pluginRoute.Post("/:pluginId/settings", bind(m.UpdatePluginSettingCmd{}), wrap(UpdatePluginSetting))
		}, reqOrgAdmin)

		apiRoute.Get("/frontend/settings/", GetFrontendSettings)
		apiRoute.Any("/datasources/proxy/:id/*", reqSignedIn, hs.ProxyDataSourceRequest)
		apiRoute.Any("/datasources/proxy/:id", reqSignedIn, hs.ProxyDataSourceRequest)

		// Folders
		apiRoute.Group("/folders", func(folderRoute RouteRegister) {
			folderRoute.Get("/", wrap(GetFolders))
			folderRoute.Get("/id/:id", wrap(GetFolderByID))
			folderRoute.Post("/", bind(m.CreateFolderCommand{}), wrap(CreateFolder))

			folderRoute.Group("/:uid", func(folderUidRoute RouteRegister) {
				folderUidRoute.Get("/", wrap(GetFolderByUID))
				folderUidRoute.Put("/", bind(m.UpdateFolderCommand{}), wrap(UpdateFolder))
				folderUidRoute.Delete("/", wrap(DeleteFolder))

				folderUidRoute.Group("/permissions", func(folderPermissionRoute RouteRegister) {
					folderPermissionRoute.Get("/", wrap(GetFolderPermissionList))
					folderPermissionRoute.Post("/", bind(dtos.UpdateDashboardAclCommand{}), wrap(UpdateFolderPermissions))
				})
			})
		})

		// Dashboard
		apiRoute.Group("/dashboards", func(dashboardRoute RouteRegister) {
			dashboardRoute.Get("/uid/:uid", wrap(GetDashboard))
			dashboardRoute.Delete("/uid/:uid", wrap(DeleteDashboardByUID))

			dashboardRoute.Get("/db/:slug", wrap(GetDashboard))
			dashboardRoute.Delete("/db/:slug", wrap(DeleteDashboard))

			dashboardRoute.Post("/calculate-diff", bind(dtos.CalculateDiffOptions{}), wrap(CalculateDashboardDiff))

			dashboardRoute.Post("/db", bind(m.SaveDashboardCommand{}), wrap(PostDashboard))
			dashboardRoute.Get("/home", wrap(GetHomeDashboard))
			dashboardRoute.Get("/tags", GetDashboardTags)
			dashboardRoute.Post("/import", bind(dtos.ImportDashboardCommand{}), wrap(ImportDashboard))

			dashboardRoute.Group("/id/:dashboardId", func(dashIdRoute RouteRegister) {
				dashIdRoute.Get("/versions", wrap(GetDashboardVersions))
				dashIdRoute.Get("/versions/:id", wrap(GetDashboardVersion))
				dashIdRoute.Post("/restore", bind(dtos.RestoreDashboardVersionCommand{}), wrap(RestoreDashboardVersion))

				dashIdRoute.Group("/permissions", func(dashboardPermissionRoute RouteRegister) {
					dashboardPermissionRoute.Get("/", wrap(GetDashboardPermissionList))
					dashboardPermissionRoute.Post("/", bind(dtos.UpdateDashboardAclCommand{}), wrap(UpdateDashboardPermissions))
				})
			})
		})

		// Dashboard snapshots
		apiRoute.Group("/dashboard/snapshots", func(dashboardRoute RouteRegister) {
			dashboardRoute.Get("/", wrap(SearchDashboardSnapshots))
		})

		// Playlist
		apiRoute.Group("/playlists", func(playlistRoute RouteRegister) {
			playlistRoute.Get("/", wrap(SearchPlaylists))
			playlistRoute.Get("/:id", ValidateOrgPlaylist, wrap(GetPlaylist))
			playlistRoute.Get("/:id/items", ValidateOrgPlaylist, wrap(GetPlaylistItems))
			playlistRoute.Get("/:id/dashboards", ValidateOrgPlaylist, wrap(GetPlaylistDashboards))
			playlistRoute.Delete("/:id", reqEditorRole, ValidateOrgPlaylist, wrap(DeletePlaylist))
			playlistRoute.Put("/:id", reqEditorRole, bind(m.UpdatePlaylistCommand{}), ValidateOrgPlaylist, wrap(UpdatePlaylist))
			playlistRoute.Post("/", reqEditorRole, bind(m.CreatePlaylistCommand{}), wrap(CreatePlaylist))
		})

		// Search
		apiRoute.Get("/search/", Search)

		// metrics
		apiRoute.Post("/tsdb/query", bind(dtos.MetricRequest{}), wrap(QueryMetrics))
		apiRoute.Get("/tsdb/testdata/scenarios", wrap(GetTestDataScenarios))
		apiRoute.Get("/tsdb/testdata/gensql", reqGrafanaAdmin, wrap(GenerateSQLTestData))
		apiRoute.Get("/tsdb/testdata/random-walk", wrap(GetTestDataRandomWalk))

		apiRoute.Group("/alerts", func(alertsRoute RouteRegister) {
			alertsRoute.Post("/test", bind(dtos.AlertTestCommand{}), wrap(AlertTest))
			alertsRoute.Post("/:alertId/pause", reqEditorRole, bind(dtos.PauseAlertCommand{}), wrap(PauseAlert))
			alertsRoute.Get("/:alertId", ValidateOrgAlert, wrap(GetAlert))
			alertsRoute.Get("/", wrap(GetAlerts))
			alertsRoute.Get("/states-for-dashboard", wrap(GetAlertStatesForDashboard))
		})

		apiRoute.Get("/alert-notifications", wrap(GetAlertNotifications))
		apiRoute.Get("/alert-notifiers", wrap(GetAlertNotifiers))

		apiRoute.Group("/alert-notifications", func(alertNotifications RouteRegister) {
			alertNotifications.Post("/test", bind(dtos.NotificationTestCommand{}), wrap(NotificationTest))
			alertNotifications.Post("/", bind(m.CreateAlertNotificationCommand{}), wrap(CreateAlertNotification))
			alertNotifications.Put("/:notificationId", bind(m.UpdateAlertNotificationCommand{}), wrap(UpdateAlertNotification))
			alertNotifications.Get("/:notificationId", wrap(GetAlertNotificationByID))
			alertNotifications.Delete("/:notificationId", wrap(DeleteAlertNotification))
		}, reqEditorRole)

		apiRoute.Get("/annotations", wrap(GetAnnotations))
		apiRoute.Post("/annotations/mass-delete", reqOrgAdmin, bind(dtos.DeleteAnnotationsCmd{}), wrap(DeleteAnnotations))

		apiRoute.Group("/annotations", func(annotationsRoute RouteRegister) {
			annotationsRoute.Post("/", bind(dtos.PostAnnotationsCmd{}), wrap(PostAnnotation))
			annotationsRoute.Delete("/:annotationId", wrap(DeleteAnnotationByID))
			annotationsRoute.Put("/:annotationId", bind(dtos.UpdateAnnotationsCmd{}), wrap(UpdateAnnotation))
			annotationsRoute.Delete("/region/:regionId", wrap(DeleteAnnotationRegion))
			annotationsRoute.Post("/graphite", reqEditorRole, bind(dtos.PostGraphiteAnnotationsCmd{}), wrap(PostGraphiteAnnotation))
		})

		// error test
		r.Get("/metrics/error", wrap(GenerateError))

	}, reqSignedIn)

	// admin api
	r.Group("/api/admin", func(adminRoute RouteRegister) {
		adminRoute.Get("/settings", AdminGetSettings)
		adminRoute.Post("/users", bind(dtos.AdminCreateUserForm{}), AdminCreateUser)
		adminRoute.Put("/users/:id/password", bind(dtos.AdminUpdateUserPasswordForm{}), AdminUpdateUserPassword)
		adminRoute.Put("/users/:id/permissions", bind(dtos.AdminUpdateUserPermissionsForm{}), AdminUpdateUserPermissions)
		adminRoute.Delete("/users/:id", AdminDeleteUser)
		adminRoute.Get("/users/:id/quotas", wrap(GetUserQuotas))
		adminRoute.Put("/users/:id/quotas/:target", bind(m.UpdateUserQuotaCmd{}), wrap(UpdateUserQuota))
		adminRoute.Get("/stats", AdminGetStats)
		adminRoute.Post("/pause-all-alerts", bind(dtos.PauseAllAlertsCommand{}), wrap(PauseAllAlerts))
	}, reqGrafanaAdmin)

	// rendering
	r.Get("/render/*", reqSignedIn, RenderToPng)

	// grafana.net proxy
	r.Any("/api/gnet/*", reqSignedIn, ProxyGnetRequest)

	// Gravatar service.
	avatarCacheServer := avatar.NewCacheServer()
	r.Get("/avatar/:hash", avatarCacheServer.Handler)

	// Websocket
	r.Any("/ws", hs.streamManager.Serve)

	// streams
	//r.Post("/api/streams/push", reqSignedIn, bind(dtos.StreamMessage{}), liveConn.PushToStream)

	r.Register(macaronR)

	InitAppPluginRoutes(macaronR)

	macaronR.NotFound(NotFoundHandler)
}
