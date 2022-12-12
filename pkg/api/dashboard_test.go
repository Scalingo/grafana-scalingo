package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/registry/corekind"
	"github.com/grafana/grafana/pkg/services/accesscontrol/actest"
	accesscontrolmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/annotations/annotationstest"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/dashboards/database"
	"github.com/grafana/grafana/pkg/services/dashboards/service"
	dashver "github.com/grafana/grafana/pkg/services/dashboardversion"
	"github.com/grafana/grafana/pkg/services/dashboardversion/dashvertest"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/folder"
	"github.com/grafana/grafana/pkg/services/folder/foldertest"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/libraryelements"
	"github.com/grafana/grafana/pkg/services/live"
	"github.com/grafana/grafana/pkg/services/org"
	pref "github.com/grafana/grafana/pkg/services/preference"
	"github.com/grafana/grafana/pkg/services/preference/preftest"
	"github.com/grafana/grafana/pkg/services/provisioning"
	"github.com/grafana/grafana/pkg/services/quota/quotatest"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/services/tag/tagimpl"
	"github.com/grafana/grafana/pkg/services/team/teamtest"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

func TestGetHomeDashboard(t *testing.T) {
	httpReq, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)
	httpReq.Header.Add("Content-Type", "application/json")
	req := &models.ReqContext{SignedInUser: &user.SignedInUser{}, Context: &web.Context{Req: httpReq}}
	cfg := setting.NewCfg()
	cfg.StaticRootPath = "../../public/"
	prefService := preftest.NewPreferenceServiceFake()
	dashboardVersionService := dashvertest.NewDashboardVersionServiceFake()

	hs := &HTTPServer{
		Cfg:                     cfg,
		pluginStore:             &plugins.FakePluginStore{},
		SQLStore:                mockstore.NewSQLStoreMock(),
		preferenceService:       prefService,
		dashboardVersionService: dashboardVersionService,
		Kinds:                   corekind.NewBase(nil),
	}

	tests := []struct {
		name                  string
		defaultSetting        string
		expectedDashboardPath string
	}{
		{name: "using default config", defaultSetting: "", expectedDashboardPath: "../../public/dashboards/home.json"},
		{name: "custom path", defaultSetting: "../../public/dashboards/default.json", expectedDashboardPath: "../../public/dashboards/default.json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dash := dtos.DashboardFullWithMeta{}
			dash.Meta.FolderTitle = "General"

			homeDashJSON, err := os.ReadFile(tc.expectedDashboardPath)
			require.NoError(t, err, "must be able to read expected dashboard file")
			hs.Cfg.DefaultHomeDashboardPath = tc.defaultSetting
			bytes, err := simplejson.NewJson(homeDashJSON)
			require.NoError(t, err, "must be able to encode file as JSON")

			prefService.ExpectedPreference = &pref.Preference{}

			dash.Dashboard = bytes

			b, err := json.Marshal(dash)
			require.NoError(t, err, "must be able to marshal object to JSON")

			res := hs.GetHomeDashboard(req)
			nr, ok := res.(*response.NormalResponse)
			require.True(t, ok, "should return *NormalResponse")
			require.Equal(t, b, nr.Body(), "default home dashboard should equal content on disk")
		})
	}
}

func newTestLive(t *testing.T, store db.DB) *live.GrafanaLive {
	features := featuremgmt.WithFeatures()
	cfg := &setting.Cfg{AppURL: "http://localhost:3000/"}
	cfg.IsFeatureToggleEnabled = features.IsEnabled
	gLive, err := live.ProvideService(nil, cfg,
		routing.NewRouteRegister(),
		nil, nil, nil,
		store,
		nil,
		&usagestats.UsageStatsMock{T: t},
		nil,
		features, accesscontrolmock.New(), &dashboards.FakeDashboardService{}, annotationstest.NewFakeAnnotationsRepo(), nil)
	require.NoError(t, err)
	return gLive
}

// This tests three main scenarios.
// If a user has access to execute an action on a dashboard:
//   1. and the dashboard is in a folder which does not have an acl
//   2. and the dashboard is in a folder which does have an acl
// 3. Post dashboard response tests

func TestDashboardAPIEndpoint(t *testing.T) {
	t.Run("Given a dashboard with a parent folder which does not have an ACL", func(t *testing.T) {
		fakeDash := models.NewDashboard("Child dash")
		fakeDash.Id = 1
		fakeDash.FolderId = 1
		fakeDash.HasACL = false
		fakeDashboardVersionService := dashvertest.NewDashboardVersionServiceFake()
		fakeDashboardVersionService.ExpectedDashboardVersion = &dashver.DashboardVersion{}
		teamService := &teamtest.FakeService{}
		dashboardService := dashboards.NewFakeDashboardService(t)
		dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardQuery)
			q.Result = fakeDash
		}).Return(nil)
		mockSQLStore := mockstore.NewSQLStoreMock()

		hs := &HTTPServer{
			Cfg:                     setting.NewCfg(),
			pluginStore:             &plugins.FakePluginStore{},
			SQLStore:                mockSQLStore,
			AccessControl:           accesscontrolmock.New(),
			Features:                featuremgmt.WithFeatures(),
			DashboardService:        dashboardService,
			dashboardVersionService: fakeDashboardVersionService,
			Kinds:                   corekind.NewBase(nil),
			QuotaService:            quotatest.New(false, nil),
		}

		setUp := func() {
			viewerRole := org.RoleViewer
			editorRole := org.RoleEditor
			dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
				q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
				q.Result = []*models.DashboardACLInfoDTO{
					{Role: &viewerRole, Permission: models.PERMISSION_VIEW},
					{Role: &editorRole, Permission: models.PERMISSION_EDIT},
				}
			}).Return(nil)
			guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
		}

		// This tests two scenarios:
		// 1. user is an org viewer
		// 2. user is an org editor

		t.Run("When user is an Org Viewer", func(t *testing.T) {
			role := org.RoleViewer
			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

					assert.False(t, dash.Meta.CanEdit)
					assert.False(t, dash.Meta.CanSave)
					assert.False(t, dash.Meta.CanAdmin)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1",
				"/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore

					hs.callGetDashboardVersion(sc)
					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions",
				"/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore

					hs.callGetDashboardVersions(sc)
					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)
		})

		t.Run("When user is an Org Editor", func(t *testing.T) {
			role := org.RoleEditor
			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

					assert.True(t, dash.Meta.CanEdit)
					assert.True(t, dash.Meta.CanSave)
					assert.False(t, dash.Meta.CanAdmin)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1",
				"/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					hs.callGetDashboardVersion(sc)

					assert.Equal(t, 200, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions",
				"/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
					setUp()
					hs.callGetDashboardVersions(sc)

					assert.Equal(t, 200, sc.resp.Code)
				}, mockSQLStore)
		})
	})

	t.Run("Given a dashboard with a parent folder which has an ACL", func(t *testing.T) {
		fakeDash := models.NewDashboard("Child dash")
		fakeDash.Id = 1
		fakeDash.FolderId = 1
		fakeDash.HasACL = true
		fakeDashboardVersionService := dashvertest.NewDashboardVersionServiceFake()
		fakeDashboardVersionService.ExpectedDashboardVersion = &dashver.DashboardVersion{}
		teamService := &teamtest.FakeService{}
		dashboardService := dashboards.NewFakeDashboardService(t)
		dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardQuery)
			q.Result = fakeDash
		}).Return(nil)
		dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
			q.Result = []*models.DashboardACLInfoDTO{
				{
					DashboardId: 1,
					Permission:  models.PERMISSION_EDIT,
					UserId:      200,
				},
			}
		}).Return(nil)

		mockSQLStore := mockstore.NewSQLStoreMock()
		cfg := setting.NewCfg()
		sql := db.InitTestDB(t)

		hs := &HTTPServer{
			Cfg:                     cfg,
			Live:                    newTestLive(t, sql),
			LibraryPanelService:     &mockLibraryPanelService{},
			LibraryElementService:   &mockLibraryElementService{},
			SQLStore:                mockSQLStore,
			AccessControl:           accesscontrolmock.New(),
			DashboardService:        dashboardService,
			dashboardVersionService: fakeDashboardVersionService,
			Features:                featuremgmt.WithFeatures(),
			Kinds:                   corekind.NewBase(nil),
		}

		setUp := func() {
			origCanEdit := setting.ViewersCanEdit
			t.Cleanup(func() {
				setting.ViewersCanEdit = origCanEdit
			})
			setting.ViewersCanEdit = false
			guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
		}

		// This tests six scenarios:
		// 1. user is an org viewer AND has no permissions for this dashboard
		// 2. user is an org editor AND has no permissions for this dashboard
		// 3. user is an org viewer AND has been granted edit permission for the dashboard
		// 4. user is an org viewer AND all viewers have edit permission for this dashboard
		// 5. user is an org viewer AND has been granted an admin permission
		// 6. user is an org editor AND has been granted a view permission

		t.Run("When user is an Org Viewer and has no permissions for this dashboard", func(t *testing.T) {
			role := org.RoleViewer
			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					sc.handlerFunc = hs.GetDashboard
					sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					hs.callDeleteDashboardByUID(t, sc, dashboardService)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1",
				"/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					hs.callGetDashboardVersion(sc)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions",
				"/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
					setUp()
					hs.callGetDashboardVersions(sc)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)
		})

		t.Run("When user is an Org Editor and has no permissions for this dashboard", func(t *testing.T) {
			role := org.RoleEditor
			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					sc.sqlStore = mockSQLStore
					sc.handlerFunc = hs.GetDashboard
					sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUp()
					hs.callDeleteDashboardByUID(t, sc, dashboardService)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1",
				"/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
					setUp()
					hs.callGetDashboardVersion(sc)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions",
				"/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
					setUp()
					hs.callGetDashboardVersions(sc)

					assert.Equal(t, 403, sc.resp.Code)
				}, mockSQLStore)
		})

		t.Run("When user is an Org Viewer but has an edit permission", func(t *testing.T) {
			role := org.RoleViewer

			setUpInner := func() {
				origCanEdit := setting.ViewersCanEdit
				t.Cleanup(func() {
					setting.ViewersCanEdit = origCanEdit
				})
				setting.ViewersCanEdit = false

				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
					q.Result = []*models.DashboardACLInfoDTO{
						{OrgId: 1, DashboardId: 2, UserId: 1, Permission: models.PERMISSION_EDIT},
					}
				}).Return(nil)
				guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
			}

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi",
				"/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
					setUpInner()
					sc.sqlStore = mockSQLStore
					dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

					assert.True(t, dash.Meta.CanEdit)
					assert.True(t, dash.Meta.CanSave)
					assert.False(t, dash.Meta.CanAdmin)
				}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()
				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardQuery)
					q.Result = models.NewDashboard("test")
				}).Return(nil)
				dashboardService.On("DeleteDashboard", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(nil)

				hs.callDeleteDashboardByUID(t, sc, dashboardService)

				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1", "/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
				setUpInner()
				sc.sqlStore = mockSQLStore
				sc.dashboardVersionService = fakeDashboardVersionService
				hs.callGetDashboardVersion(sc)

				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions", "/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
				setUpInner()
				hs.callGetDashboardVersions(sc)

				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)
		})

		t.Run("When user is an Org Viewer and viewers can edit", func(t *testing.T) {
			role := org.RoleViewer

			setUpInner := func() {
				origCanEdit := setting.ViewersCanEdit
				t.Cleanup(func() {
					setting.ViewersCanEdit = origCanEdit
				})
				setting.ViewersCanEdit = true

				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
					q.Result = []*models.DashboardACLInfoDTO{
						{OrgId: 1, DashboardId: 2, UserId: 1, Permission: models.PERMISSION_VIEW},
					}
				}).Return(nil)
				guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
			}

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()

				require.True(t, setting.ViewersCanEdit)
				sc.sqlStore = mockSQLStore
				dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

				assert.True(t, dash.Meta.CanEdit)
				assert.False(t, dash.Meta.CanSave)
				assert.False(t, dash.Meta.CanAdmin)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()

				hs.callDeleteDashboardByUID(t, sc, dashboardService)
				assert.Equal(t, 403, sc.resp.Code)
			}, mockSQLStore)
		})

		t.Run("When user is an Org Viewer but has an admin permission", func(t *testing.T) {
			role := org.RoleViewer

			setUpInner := func() {
				origCanEdit := setting.ViewersCanEdit
				t.Cleanup(func() {
					setting.ViewersCanEdit = origCanEdit
				})
				setting.ViewersCanEdit = true

				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
					q.Result = []*models.DashboardACLInfoDTO{
						{OrgId: 1, DashboardId: 2, UserId: 1, Permission: models.PERMISSION_ADMIN},
					}
				}).Return(nil)
				guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
			}

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()
				sc.sqlStore = mockSQLStore
				dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

				assert.True(t, dash.Meta.CanEdit)
				assert.True(t, dash.Meta.CanSave)
				assert.True(t, dash.Meta.CanAdmin)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()
				sc.sqlStore = mockSQLStore
				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardQuery)
					q.Result = models.NewDashboard("test")
				}).Return(nil)
				dashboardService.On("DeleteDashboard", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(nil)
				hs.callDeleteDashboardByUID(t, sc, dashboardService)

				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1", "/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
				setUpInner()

				hs.callGetDashboardVersion(sc)
				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions", "/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
				setUpInner()

				hs.callGetDashboardVersions(sc)
				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)
		})

		t.Run("When user is an Org Editor but has a view permission", func(t *testing.T) {
			role := org.RoleEditor

			setUpInner := func() {
				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
					q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
					q.Result = []*models.DashboardACLInfoDTO{
						{OrgId: 1, DashboardId: 2, UserId: 1, Permission: models.PERMISSION_VIEW},
					}
				}).Return(nil)
				guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)
			}

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()
				sc.sqlStore = mockSQLStore
				dash := getDashboardShouldReturn200WithConfig(t, sc, nil, nil, dashboardService)

				assert.False(t, dash.Meta.CanEdit)
				assert.False(t, dash.Meta.CanSave)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling DELETE on", "DELETE", "/api/dashboards/uid/abcdefghi", "/api/dashboards/uid/:uid", role, func(sc *scenarioContext) {
				setUpInner()
				hs.callDeleteDashboardByUID(t, sc, dashboardService)

				assert.Equal(t, 403, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions/1", "/api/dashboards/id/:dashboardId/versions/:id", role, func(sc *scenarioContext) {
				setUpInner()
				hs.callGetDashboardVersion(sc)

				assert.Equal(t, 403, sc.resp.Code)
			}, mockSQLStore)

			loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/id/2/versions", "/api/dashboards/id/:dashboardId/versions", role, func(sc *scenarioContext) {
				setUpInner()
				hs.callGetDashboardVersions(sc)

				assert.Equal(t, 403, sc.resp.Code)
			}, mockSQLStore)
		})
	})

	t.Run("Given two dashboards with the same title in different folders", func(t *testing.T) {
		dashOne := models.NewDashboard("dash")
		dashOne.Id = 2
		dashOne.FolderId = 1
		dashOne.HasACL = false

		dashTwo := models.NewDashboard("dash")
		dashTwo.Id = 4
		dashTwo.FolderId = 3
		dashTwo.HasACL = false
	})

	t.Run("Post dashboard response tests", func(t *testing.T) {
		dashboardStore := &dashboards.FakeDashboardStore{}
		defer dashboardStore.AssertExpectations(t)
		// This tests that a valid request returns correct response
		t.Run("Given a correct request for creating a dashboard", func(t *testing.T) {
			const folderID int64 = 3
			const dashID int64 = 2

			cmd := models.SaveDashboardCommand{
				OrgId:  1,
				UserId: 5,
				Dashboard: simplejson.NewFromAny(map[string]interface{}{
					"title": "Dash",
				}),
				Overwrite: true,
				FolderId:  folderID,
				IsFolder:  false,
				Message:   "msg",
			}

			dashboardService := dashboards.NewFakeDashboardService(t)
			dashboardService.On("SaveDashboard", mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"), mock.AnythingOfType("bool")).
				Return(&models.Dashboard{Id: dashID, Uid: "uid", Title: "Dash", Slug: "dash", Version: 2}, nil)

			postDashboardScenario(t, "When calling POST on", "/api/dashboards", "/api/dashboards", cmd, dashboardService, nil, func(sc *scenarioContext) {
				callPostDashboardShouldReturnSuccess(sc)

				result := sc.ToJSON()
				assert.Equal(t, "success", result.Get("status").MustString())
				assert.Equal(t, dashID, result.Get("id").MustInt64())
				assert.Equal(t, "uid", result.Get("uid").MustString())
				assert.Equal(t, "dash", result.Get("slug").MustString())
				assert.Equal(t, "/d/uid/dash", result.Get("url").MustString())
			})
		})

		t.Run("Given a correct request for creating a dashboard with folder uid", func(t *testing.T) {
			const folderUid string = "folderUID"
			const dashID int64 = 2

			cmd := models.SaveDashboardCommand{
				OrgId:  1,
				UserId: 5,
				Dashboard: simplejson.NewFromAny(map[string]interface{}{
					"title": "Dash",
				}),
				Overwrite: true,
				FolderUid: folderUid,
				IsFolder:  false,
				Message:   "msg",
			}

			dashboardService := dashboards.NewFakeDashboardService(t)
			dashboardService.On("SaveDashboard", mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"), mock.AnythingOfType("bool")).
				Return(&models.Dashboard{Id: dashID, Uid: "uid", Title: "Dash", Slug: "dash", Version: 2}, nil)

			mockFolder := &foldertest.FakeService{
				ExpectedFolder: &folder.Folder{ID: 1, UID: "folderUID", Title: "Folder"},
			}

			postDashboardScenario(t, "When calling POST on", "/api/dashboards", "/api/dashboards", cmd, dashboardService, mockFolder, func(sc *scenarioContext) {
				callPostDashboardShouldReturnSuccess(sc)

				result := sc.ToJSON()
				assert.Equal(t, "success", result.Get("status").MustString())
				assert.Equal(t, dashID, result.Get("id").MustInt64())
				assert.Equal(t, "uid", result.Get("uid").MustString())
				assert.Equal(t, "dash", result.Get("slug").MustString())
				assert.Equal(t, "/d/uid/dash", result.Get("url").MustString())
			})
		})

		t.Run("Given a request with incorrect folder uid for creating a dashboard with", func(t *testing.T) {
			cmd := models.SaveDashboardCommand{
				OrgId:  1,
				UserId: 5,
				Dashboard: simplejson.NewFromAny(map[string]interface{}{
					"title": "Dash",
				}),
				Overwrite: true,
				FolderUid: "folderUID",
				IsFolder:  false,
				Message:   "msg",
			}

			dashboardService := dashboards.NewFakeDashboardService(t)

			mockFolder := &foldertest.FakeService{
				ExpectedError: errors.New("Error while searching Folder ID"),
			}

			postDashboardScenario(t, "When calling POST on", "/api/dashboards", "/api/dashboards", cmd, dashboardService, mockFolder, func(sc *scenarioContext) {
				callPostDashboard(sc)
				assert.Equal(t, 500, sc.resp.Code)
			})
		})

		// This tests that invalid requests returns expected error responses
		t.Run("Given incorrect requests for creating a dashboard", func(t *testing.T) {
			testCases := []struct {
				SaveError          error
				ExpectedStatusCode int
			}{
				{SaveError: dashboards.ErrDashboardNotFound, ExpectedStatusCode: 404},
				{SaveError: dashboards.ErrFolderNotFound, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardWithSameUIDExists, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardWithSameNameInFolderExists, ExpectedStatusCode: 412},
				{SaveError: dashboards.ErrDashboardVersionMismatch, ExpectedStatusCode: 412},
				{SaveError: dashboards.ErrDashboardTitleEmpty, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardFolderCannotHaveParent, ExpectedStatusCode: 400},
				{SaveError: alerting.ValidationError{Reason: "Mu"}, ExpectedStatusCode: 422},
				{SaveError: dashboards.ErrDashboardFailedGenerateUniqueUid, ExpectedStatusCode: 500},
				{SaveError: dashboards.ErrDashboardTypeMismatch, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardFolderWithSameNameAsDashboard, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardWithSameNameAsFolder, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardFolderNameExists, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardUpdateAccessDenied, ExpectedStatusCode: 403},
				{SaveError: dashboards.ErrDashboardInvalidUid, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardUidTooLong, ExpectedStatusCode: 400},
				{SaveError: dashboards.ErrDashboardCannotSaveProvisionedDashboard, ExpectedStatusCode: 400},
				{SaveError: dashboards.UpdatePluginDashboardError{PluginId: "plug"}, ExpectedStatusCode: 412},
			}

			cmd := models.SaveDashboardCommand{
				OrgId: 1,
				Dashboard: simplejson.NewFromAny(map[string]interface{}{
					"title": "",
				}),
			}

			for _, tc := range testCases {
				dashboardService := dashboards.NewFakeDashboardService(t)
				dashboardService.On("SaveDashboard", mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"), mock.AnythingOfType("bool")).Return(nil, tc.SaveError)

				postDashboardScenario(t, fmt.Sprintf("Expect '%s' error when calling POST on", tc.SaveError.Error()),
					"/api/dashboards", "/api/dashboards", cmd, dashboardService, nil, func(sc *scenarioContext) {
						callPostDashboard(sc)
						assert.Equal(t, tc.ExpectedStatusCode, sc.resp.Code)
					})
			}
		})
	})

	t.Run("Given a dashboard to validate", func(t *testing.T) {
		sqlmock := mockstore.SQLStoreMock{}

		t.Run("When an invalid dashboard json is posted", func(t *testing.T) {
			cmd := models.ValidateDashboardCommand{
				Dashboard: "{\"hello\": \"world\"}",
			}

			role := org.RoleAdmin
			postValidateScenario(t, "When calling POST on", "/api/dashboards/validate", "/api/dashboards/validate", cmd, role, func(sc *scenarioContext) {
				callPostDashboard(sc)

				result := sc.ToJSON()
				assert.Equal(t, 422, sc.resp.Code)
				assert.False(t, result.Get("isValid").MustBool())
				assert.NotEmpty(t, result.Get("message").MustString())
			}, &sqlmock)
		})

		t.Run("When a dashboard with a too-low schema version is posted", func(t *testing.T) {
			cmd := models.ValidateDashboardCommand{
				Dashboard: "{\"schemaVersion\": 1}",
			}

			role := org.RoleAdmin
			postValidateScenario(t, "When calling POST on", "/api/dashboards/validate", "/api/dashboards/validate", cmd, role, func(sc *scenarioContext) {
				callPostDashboard(sc)

				result := sc.ToJSON()
				assert.Equal(t, 412, sc.resp.Code)
				assert.False(t, result.Get("isValid").MustBool())
				assert.Equal(t, "invalid schema version", result.Get("message").MustString())
			}, &sqlmock)
		})

		t.Run("When a valid dashboard is posted", func(t *testing.T) {
			devenvDashboard, readErr := os.ReadFile("../../devenv/dev-dashboards/home.json")
			assert.Empty(t, readErr)

			cmd := models.ValidateDashboardCommand{
				Dashboard: string(devenvDashboard),
			}

			role := org.RoleAdmin
			postValidateScenario(t, "When calling POST on", "/api/dashboards/validate", "/api/dashboards/validate", cmd, role, func(sc *scenarioContext) {
				callPostDashboard(sc)

				result := sc.ToJSON()
				assert.Equal(t, 200, sc.resp.Code)
				assert.True(t, result.Get("isValid").MustBool())
			}, &sqlmock)
		})
	})

	t.Run("Given two dashboards being compared", func(t *testing.T) {
		fakeDashboardVersionService := dashvertest.NewDashboardVersionServiceFake()
		fakeDashboardVersionService.ExpectedDashboardVersions = []*dashver.DashboardVersion{
			{
				DashboardID: 1,
				Version:     1,
				Data: simplejson.NewFromAny(map[string]interface{}{
					"title": "Dash1",
				}),
			},
			{
				DashboardID: 2,
				Version:     2,
				Data: simplejson.NewFromAny(map[string]interface{}{
					"title": "Dash2",
				}),
			},
		}
		sqlmock := mockstore.SQLStoreMock{}
		setUp := func() {
			teamSvc := &teamtest.FakeService{}
			dashSvc := dashboards.NewFakeDashboardService(t)
			dashSvc.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Return(nil)
			guardian.InitLegacyGuardian(&sqlmock, dashSvc, teamSvc)
		}

		cmd := dtos.CalculateDiffOptions{
			Base: dtos.CalculateDiffTarget{
				DashboardId: 1,
				Version:     1,
			},
			New: dtos.CalculateDiffTarget{
				DashboardId: 2,
				Version:     2,
			},
			DiffType: "basic",
		}

		t.Run("when user does not have permission", func(t *testing.T) {
			role := org.RoleViewer
			postDiffScenario(t, "When calling POST on", "/api/dashboards/calculate-diff", "/api/dashboards/calculate-diff", cmd, role, func(sc *scenarioContext) {
				setUp()

				callPostDashboard(sc)
				assert.Equal(t, 403, sc.resp.Code)
			}, &sqlmock, fakeDashboardVersionService)
		})

		t.Run("when user does have permission", func(t *testing.T) {
			role := org.RoleAdmin
			postDiffScenario(t, "When calling POST on", "/api/dashboards/calculate-diff", "/api/dashboards/calculate-diff", cmd, role, func(sc *scenarioContext) {
				// This test shouldn't hit GetDashboardACLInfoList, so no setup needed
				sc.dashboardVersionService = fakeDashboardVersionService
				callPostDashboard(sc)
				assert.Equal(t, 200, sc.resp.Code)
			}, &sqlmock, fakeDashboardVersionService)
		})
	})

	t.Run("Given dashboard in folder being restored should restore to folder", func(t *testing.T) {
		const folderID int64 = 1
		fakeDash := models.NewDashboard("Child dash")
		fakeDash.Id = 2
		fakeDash.FolderId = folderID
		fakeDash.HasACL = false

		dashboardService := dashboards.NewFakeDashboardService(t)
		dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardQuery)
			q.Result = fakeDash
		}).Return(nil)
		dashboardService.On("SaveDashboard", mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"), mock.AnythingOfType("bool")).Run(func(args mock.Arguments) {
			cmd := args.Get(1).(*dashboards.SaveDashboardDTO)
			cmd.Dashboard = &models.Dashboard{
				Id: 2, Uid: "uid", Title: "Dash", Slug: "dash", Version: 1,
			}
		}).Return(nil, nil)

		cmd := dtos.RestoreDashboardVersionCommand{
			Version: 1,
		}
		fakeDashboardVersionService := dashvertest.NewDashboardVersionServiceFake()
		fakeDashboardVersionService.ExpectedDashboardVersions = []*dashver.DashboardVersion{
			{
				DashboardID: 2,
				Version:     1,
				Data:        fakeDash.Data,
			}}
		mockSQLStore := mockstore.NewSQLStoreMock()
		restoreDashboardVersionScenario(t, "When calling POST on", "/api/dashboards/id/1/restore",
			"/api/dashboards/id/:dashboardId/restore", dashboardService, fakeDashboardVersionService, cmd, func(sc *scenarioContext) {
				sc.dashboardVersionService = fakeDashboardVersionService

				callRestoreDashboardVersion(sc)
				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)
	})

	t.Run("Given dashboard in general folder being restored should restore to general folder", func(t *testing.T) {
		fakeDash := models.NewDashboard("Child dash")
		fakeDash.Id = 2
		fakeDash.HasACL = false

		dashboardService := dashboards.NewFakeDashboardService(t)
		dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardQuery)
			q.Result = fakeDash
		}).Return(nil)
		dashboardService.On("SaveDashboard", mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"), mock.AnythingOfType("bool")).Run(func(args mock.Arguments) {
			cmd := args.Get(1).(*dashboards.SaveDashboardDTO)
			cmd.Dashboard = &models.Dashboard{
				Id: 2, Uid: "uid", Title: "Dash", Slug: "dash", Version: 1,
			}
		}).Return(nil, nil)

		fakeDashboardVersionService := dashvertest.NewDashboardVersionServiceFake()
		fakeDashboardVersionService.ExpectedDashboardVersions = []*dashver.DashboardVersion{
			{
				DashboardID: 2,
				Version:     1,
				Data:        fakeDash.Data,
			}}

		cmd := dtos.RestoreDashboardVersionCommand{
			Version: 1,
		}
		mockSQLStore := mockstore.NewSQLStoreMock()
		restoreDashboardVersionScenario(t, "When calling POST on", "/api/dashboards/id/1/restore",
			"/api/dashboards/id/:dashboardId/restore", dashboardService, fakeDashboardVersionService, cmd, func(sc *scenarioContext) {
				callRestoreDashboardVersion(sc)
				assert.Equal(t, 200, sc.resp.Code)
			}, mockSQLStore)
	})

	t.Run("Given provisioned dashboard", func(t *testing.T) {
		mockSQLStore := mockstore.NewSQLStoreMock()
		dashboardStore := dashboards.NewFakeDashboardStore(t)
		dashboardStore.On("GetProvisionedDataByDashboardID", mock.Anything, mock.AnythingOfType("int64")).Return(&models.DashboardProvisioning{ExternalId: "/dashboard1.json"}, nil).Once()

		teamService := &teamtest.FakeService{}
		dashboardService := dashboards.NewFakeDashboardService(t)

		dataValue, err := simplejson.NewJson([]byte(`{"id": 1, "editable": true, "style": "dark"}`))
		require.NoError(t, err)
		dashboardService.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardQuery)
			q.Result = &models.Dashboard{Id: 1, Data: dataValue}
		}).Return(nil)
		dashboardService.On("GetDashboardACLInfoList", mock.Anything, mock.AnythingOfType("*models.GetDashboardACLInfoListQuery")).Run(func(args mock.Arguments) {
			q := args.Get(1).(*models.GetDashboardACLInfoListQuery)
			q.Result = []*models.DashboardACLInfoDTO{{OrgId: testOrgID, DashboardId: 1, UserId: testUserID, Permission: models.PERMISSION_EDIT}}
		}).Return(nil)
		guardian.InitLegacyGuardian(mockSQLStore, dashboardService, teamService)

		loggedInUserScenarioWithRole(t, "When calling GET on", "GET", "/api/dashboards/uid/dash", "/api/dashboards/uid/:uid", org.RoleEditor, func(sc *scenarioContext) {
			fakeProvisioningService := provisioning.NewProvisioningServiceMock(context.Background())
			fakeProvisioningService.GetDashboardProvisionerResolvedPathFunc = func(name string) string {
				return "/tmp/grafana/dashboards"
			}

			dash := getDashboardShouldReturn200WithConfig(t, sc, fakeProvisioningService, dashboardStore, dashboardService)

			assert.Equal(t, "../../../dashboard1.json", dash.Meta.ProvisionedExternalId, mockSQLStore)
		}, mockSQLStore)

		loggedInUserScenarioWithRole(t, "When allowUiUpdates is true and calling GET on", "GET", "/api/dashboards/uid/dash", "/api/dashboards/uid/:uid", org.RoleEditor, func(sc *scenarioContext) {
			fakeProvisioningService := provisioning.NewProvisioningServiceMock(context.Background())
			fakeProvisioningService.GetDashboardProvisionerResolvedPathFunc = func(name string) string {
				return "/tmp/grafana/dashboards"
			}

			fakeProvisioningService.GetAllowUIUpdatesFromConfigFunc = func(name string) bool {
				return true
			}

			hs := &HTTPServer{
				Cfg:                          setting.NewCfg(),
				ProvisioningService:          fakeProvisioningService,
				LibraryPanelService:          &mockLibraryPanelService{},
				LibraryElementService:        &mockLibraryElementService{},
				dashboardProvisioningService: mockDashboardProvisioningService{},
				SQLStore:                     mockSQLStore,
				AccessControl:                accesscontrolmock.New(),
				DashboardService:             dashboardService,
				Features:                     featuremgmt.WithFeatures(),
				Kinds:                        corekind.NewBase(nil),
			}
			hs.callGetDashboard(sc)

			assert.Equal(t, 200, sc.resp.Code)

			dash := dtos.DashboardFullWithMeta{}
			err := json.NewDecoder(sc.resp.Body).Decode(&dash)
			require.NoError(t, err)

			assert.Equal(t, false, dash.Meta.Provisioned)
		}, mockSQLStore)
	})
}

func getDashboardShouldReturn200WithConfig(t *testing.T, sc *scenarioContext, provisioningService provisioning.ProvisioningService, dashboardStore dashboards.Store, dashboardService dashboards.DashboardService) dtos.DashboardFullWithMeta {
	t.Helper()

	if provisioningService == nil {
		provisioningService = provisioning.NewProvisioningServiceMock(context.Background())
	}

	var err error
	if dashboardStore == nil {
		sql := db.InitTestDB(t)
		quotaService := quotatest.New(false, nil)
		dashboardStore, err = database.ProvideDashboardStore(sql, sql.Cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sql, sql.Cfg), quotaService)
		require.NoError(t, err)
	}

	libraryPanelsService := mockLibraryPanelService{}
	libraryElementsService := mockLibraryElementService{}
	cfg := setting.NewCfg()
	ac := accesscontrolmock.New()
	folderPermissions := accesscontrolmock.NewMockedPermissionsService()
	dashboardPermissions := accesscontrolmock.NewMockedPermissionsService()
	features := featuremgmt.WithFeatures()

	if dashboardService == nil {
		dashboardService = service.ProvideDashboardService(
			cfg, dashboardStore, nil, features,
			folderPermissions, dashboardPermissions, ac,
		)
	}

	hs := &HTTPServer{
		Cfg:                   cfg,
		LibraryPanelService:   &libraryPanelsService,
		LibraryElementService: &libraryElementsService,
		SQLStore:              sc.sqlStore,
		ProvisioningService:   provisioningService,
		AccessControl:         accesscontrolmock.New(),
		dashboardProvisioningService: service.ProvideDashboardService(
			cfg, dashboardStore, nil, features,
			folderPermissions, dashboardPermissions, ac,
		),
		DashboardService: dashboardService,
		Features:         featuremgmt.WithFeatures(),
		Kinds:            corekind.NewBase(nil),
	}

	hs.callGetDashboard(sc)

	require.Equal(sc.t, 200, sc.resp.Code)

	dash := dtos.DashboardFullWithMeta{}
	err = json.NewDecoder(sc.resp.Body).Decode(&dash)
	require.NoError(sc.t, err)

	return dash
}

func (hs *HTTPServer) callGetDashboard(sc *scenarioContext) {
	sc.handlerFunc = hs.GetDashboard
	sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()
}

func (hs *HTTPServer) callGetDashboardVersion(sc *scenarioContext) {
	sc.handlerFunc = hs.GetDashboardVersion
	sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()
}

func (hs *HTTPServer) callGetDashboardVersions(sc *scenarioContext) {
	sc.handlerFunc = hs.GetDashboardVersions
	sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()
}

func (hs *HTTPServer) callDeleteDashboardByUID(t *testing.T,
	sc *scenarioContext, mockDashboard *dashboards.FakeDashboardService) {
	hs.DashboardService = mockDashboard
	sc.handlerFunc = hs.DeleteDashboardByUID
	sc.fakeReqWithParams("DELETE", sc.url, map[string]string{}).exec()
}

func callPostDashboard(sc *scenarioContext) {
	sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
}

func callRestoreDashboardVersion(sc *scenarioContext) {
	sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
}

func callPostDashboardShouldReturnSuccess(sc *scenarioContext) {
	callPostDashboard(sc)

	assert.Equal(sc.t, 200, sc.resp.Code)
}

func postDashboardScenario(t *testing.T, desc string, url string, routePattern string, cmd models.SaveDashboardCommand, dashboardService dashboards.DashboardService, folderService folder.Service, fn scenarioFunc) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		cfg := setting.NewCfg()
		hs := HTTPServer{
			Cfg:                   cfg,
			ProvisioningService:   provisioning.NewProvisioningServiceMock(context.Background()),
			Live:                  newTestLive(t, db.InitTestDB(t)),
			QuotaService:          quotatest.New(false, nil),
			pluginStore:           &plugins.FakePluginStore{},
			LibraryPanelService:   &mockLibraryPanelService{},
			LibraryElementService: &mockLibraryElementService{},
			DashboardService:      dashboardService,
			folderService:         folderService,
			Features:              featuremgmt.WithFeatures(),
			Kinds:                 corekind.NewBase(nil),
			accesscontrolService:  actest.FakeService{},
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.SignedInUser = &user.SignedInUser{OrgID: cmd.OrgId, UserID: cmd.UserId}

			return hs.PostDashboard(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func postValidateScenario(t *testing.T, desc string, url string, routePattern string, cmd models.ValidateDashboardCommand,
	role org.RoleType, fn scenarioFunc, sqlmock sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		cfg := setting.NewCfg()
		hs := HTTPServer{
			Cfg:                   cfg,
			ProvisioningService:   provisioning.NewProvisioningServiceMock(context.Background()),
			Live:                  newTestLive(t, db.InitTestDB(t)),
			QuotaService:          quotatest.New(false, nil),
			LibraryPanelService:   &mockLibraryPanelService{},
			LibraryElementService: &mockLibraryElementService{},
			SQLStore:              sqlmock,
			Features:              featuremgmt.WithFeatures(),
			Kinds:                 corekind.NewBase(nil),
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.SignedInUser = &user.SignedInUser{
				OrgID:  testOrgID,
				UserID: testUserID,
			}
			sc.context.OrgRole = role

			return hs.ValidateDashboard(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func postDiffScenario(t *testing.T, desc string, url string, routePattern string, cmd dtos.CalculateDiffOptions,
	role org.RoleType, fn scenarioFunc, sqlmock sqlstore.Store, fakeDashboardVersionService *dashvertest.FakeDashboardVersionService) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		cfg := setting.NewCfg()
		hs := HTTPServer{
			Cfg:                     cfg,
			ProvisioningService:     provisioning.NewProvisioningServiceMock(context.Background()),
			Live:                    newTestLive(t, db.InitTestDB(t)),
			QuotaService:            quotatest.New(false, nil),
			LibraryPanelService:     &mockLibraryPanelService{},
			LibraryElementService:   &mockLibraryElementService{},
			SQLStore:                sqlmock,
			dashboardVersionService: fakeDashboardVersionService,
			Features:                featuremgmt.WithFeatures(),
			Kinds:                   corekind.NewBase(nil),
		}

		sc := setupScenarioContext(t, url)
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.SignedInUser = &user.SignedInUser{
				OrgID:  testOrgID,
				UserID: testUserID,
			}
			sc.context.OrgRole = role

			return hs.CalculateDashboardDiff(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func restoreDashboardVersionScenario(t *testing.T, desc string, url string, routePattern string,
	mock *dashboards.FakeDashboardService, fakeDashboardVersionService *dashvertest.FakeDashboardVersionService,
	cmd dtos.RestoreDashboardVersionCommand, fn scenarioFunc, sqlStore sqlstore.Store) {
	t.Run(fmt.Sprintf("%s %s", desc, url), func(t *testing.T) {
		cfg := setting.NewCfg()
		hs := HTTPServer{
			Cfg:                     cfg,
			ProvisioningService:     provisioning.NewProvisioningServiceMock(context.Background()),
			Live:                    newTestLive(t, db.InitTestDB(t)),
			QuotaService:            quotatest.New(false, nil),
			LibraryPanelService:     &mockLibraryPanelService{},
			LibraryElementService:   &mockLibraryElementService{},
			DashboardService:        mock,
			SQLStore:                sqlStore,
			Features:                featuremgmt.WithFeatures(),
			dashboardVersionService: fakeDashboardVersionService,
			Kinds:                   corekind.NewBase(nil),
			accesscontrolService:    actest.FakeService{},
		}

		sc := setupScenarioContext(t, url)
		sc.sqlStore = sqlStore
		sc.dashboardVersionService = fakeDashboardVersionService
		sc.defaultHandler = routing.Wrap(func(c *models.ReqContext) response.Response {
			c.Req.Body = mockRequestBody(cmd)
			c.Req.Header.Add("Content-Type", "application/json")
			sc.context = c
			sc.context.SignedInUser = &user.SignedInUser{
				OrgID:  testOrgID,
				UserID: testUserID,
			}
			sc.context.OrgRole = org.RoleAdmin

			return hs.RestoreDashboardVersion(c)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}

func (sc *scenarioContext) ToJSON() *simplejson.Json {
	result := simplejson.New()
	err := json.NewDecoder(sc.resp.Body).Decode(result)
	require.NoError(sc.t, err)
	return result
}

type mockDashboardProvisioningService struct {
	dashboards.DashboardProvisioningService
}

func (s mockDashboardProvisioningService) GetProvisionedDashboardDataByDashboardID(ctx context.Context, dashboardID int64) (
	*models.DashboardProvisioning, error) {
	return nil, nil
}

type mockLibraryPanelService struct {
}

func (m *mockLibraryPanelService) ConnectLibraryPanelsForDashboard(c context.Context, signedInUser *user.SignedInUser, dash *models.Dashboard) error {
	return nil
}

func (m *mockLibraryPanelService) ImportLibraryPanelsForDashboard(c context.Context, signedInUser *user.SignedInUser, libraryPanels *simplejson.Json, panels []interface{}, folderID int64) error {
	return nil
}

type mockLibraryElementService struct {
}

func (l *mockLibraryElementService) CreateElement(c context.Context, signedInUser *user.SignedInUser, cmd libraryelements.CreateLibraryElementCommand) (libraryelements.LibraryElementDTO, error) {
	return libraryelements.LibraryElementDTO{}, nil
}

// GetElement gets an element from a UID.
func (l *mockLibraryElementService) GetElement(c context.Context, signedInUser *user.SignedInUser, UID string) (libraryelements.LibraryElementDTO, error) {
	return libraryelements.LibraryElementDTO{}, nil
}

// GetElementsForDashboard gets all connected elements for a specific dashboard.
func (l *mockLibraryElementService) GetElementsForDashboard(c context.Context, dashboardID int64) (map[string]libraryelements.LibraryElementDTO, error) {
	return map[string]libraryelements.LibraryElementDTO{}, nil
}

// ConnectElementsToDashboard connects elements to a specific dashboard.
func (l *mockLibraryElementService) ConnectElementsToDashboard(c context.Context, signedInUser *user.SignedInUser, elementUIDs []string, dashboardID int64) error {
	return nil
}

// DisconnectElementsFromDashboard disconnects elements from a specific dashboard.
func (l *mockLibraryElementService) DisconnectElementsFromDashboard(c context.Context, dashboardID int64) error {
	return nil
}

// DeleteLibraryElementsInFolder deletes all elements for a specific folder.
func (l *mockLibraryElementService) DeleteLibraryElementsInFolder(c context.Context, signedInUser *user.SignedInUser, folderUID string) error {
	return nil
}
