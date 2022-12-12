package libraryelements

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/appcontext"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/db/dbtest"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/models"
	acmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/dashboards/database"
	dashboardservice "github.com/grafana/grafana/pkg/services/dashboards/service"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/folder"
	"github.com/grafana/grafana/pkg/services/folder/folderimpl"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/quota/quotatest"
	"github.com/grafana/grafana/pkg/services/tag/tagimpl"
	"github.com/grafana/grafana/pkg/services/team/teamtest"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

const userInDbName = "user_in_db"
const userInDbAvatar = "/avatar/402d08de060496d6b6874495fe20f5ad"

func TestDeleteLibraryPanelsInFolder(t *testing.T) {
	scenarioWithPanel(t, "When an admin tries to delete a folder that contains connected library elements, it should fail",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			dash := models.Dashboard{
				Title: "Testing DeleteLibraryElementsInFolder",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.ID)
			err := sc.service.ConnectElementsToDashboard(sc.reqContext.Req.Context(), sc.reqContext.SignedInUser, []string{sc.initialResult.Result.UID}, dashInDB.Id)
			require.NoError(t, err)

			err = sc.service.DeleteLibraryElementsInFolder(sc.reqContext.Req.Context(), sc.reqContext.SignedInUser, sc.folder.UID)
			require.EqualError(t, err, ErrFolderHasConnectedLibraryElements.Error())
		})

	scenarioWithPanel(t, "When an admin tries to delete a folder uid that doesn't exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			err := sc.service.DeleteLibraryElementsInFolder(sc.reqContext.Req.Context(), sc.reqContext.SignedInUser, sc.folder.UID+"xxxx")
			require.EqualError(t, err, dashboards.ErrFolderNotFound.Error())
		})

	scenarioWithPanel(t, "When an admin tries to delete a folder that contains disconnected elements, it should delete all disconnected elements too",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateVariableCommand(sc.folder.ID, "query0")
			sc.reqContext.Req.Body = mockRequestBody(command)
			resp := sc.service.createHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			var result libraryElementsSearch
			err := json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			require.Equal(t, 2, len(result.Result.Elements))

			err = sc.service.DeleteLibraryElementsInFolder(sc.reqContext.Req.Context(), sc.reqContext.SignedInUser, sc.folder.UID)
			require.NoError(t, err)
			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			err = json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			require.Equal(t, 0, len(result.Result.Elements))
		})
}

func TestGetLibraryPanelConnections(t *testing.T) {
	scenarioWithPanel(t, "When an admin tries to get connections of library panel, it should succeed and return correct result",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			dash := models.Dashboard{
				Title: "Testing GetLibraryPanelConnections",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.ID)
			err := sc.service.ConnectElementsToDashboard(sc.reqContext.Req.Context(), sc.reqContext.SignedInUser, []string{sc.initialResult.Result.UID}, dashInDB.Id)
			require.NoError(t, err)

			var expected = func(res LibraryElementConnectionsResponse) LibraryElementConnectionsResponse {
				return LibraryElementConnectionsResponse{
					Result: []LibraryElementConnectionDTO{
						{
							ID:            sc.initialResult.Result.ID,
							Kind:          sc.initialResult.Result.Kind,
							ElementID:     1,
							ConnectionID:  dashInDB.Id,
							ConnectionUID: dashInDB.Uid,
							Created:       res.Result[0].Created,
							CreatedBy: LibraryElementDTOMetaUser{
								ID:        1,
								Name:      userInDbName,
								AvatarURL: userInDbAvatar,
							},
						},
					},
				}
			}

			sc.ctx.Req = web.SetURLParams(sc.ctx.Req, map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.getConnectionsHandler(sc.reqContext)
			var result = validateAndUnMarshalConnectionResponse(t, resp)

			if diff := cmp.Diff(expected(result), result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})
}

type libraryElement struct {
	ID          int64                  `json:"id"`
	OrgID       int64                  `json:"orgId"`
	FolderID    int64                  `json:"folderId"`
	UID         string                 `json:"uid"`
	Name        string                 `json:"name"`
	Kind        int64                  `json:"kind"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Model       map[string]interface{} `json:"model"`
	Version     int64                  `json:"version"`
	Meta        LibraryElementDTOMeta  `json:"meta"`
}

type libraryElementResult struct {
	Result libraryElement `json:"result"`
}

type libraryElementArrayResult struct {
	Result []libraryElement `json:"result"`
}

type libraryElementsSearch struct {
	Result libraryElementsSearchResult `json:"result"`
}

type libraryElementsSearchResult struct {
	TotalCount int64            `json:"totalCount"`
	Elements   []libraryElement `json:"elements"`
	Page       int              `json:"page"`
	PerPage    int              `json:"perPage"`
}

func getCreatePanelCommand(folderID int64, name string) CreateLibraryElementCommand {
	command := getCreateCommandWithModel(folderID, name, models.PanelElement, []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "id": 1,
			  "title": "Text - Library Panel",
			  "type": "text",
			  "description": "A description"
			}
		`))

	return command
}

func getCreateVariableCommand(folderID int64, name string) CreateLibraryElementCommand {
	command := getCreateCommandWithModel(folderID, name, models.VariableElement, []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "name": "query0",
			  "type": "query",
			  "description": "A description"
			}
		`))

	return command
}

func getCreateCommandWithModel(folderID int64, name string, kind models.LibraryElementKind, model []byte) CreateLibraryElementCommand {
	command := CreateLibraryElementCommand{
		FolderID: folderID,
		Name:     name,
		Model:    model,
		Kind:     int64(kind),
	}

	return command
}

type scenarioContext struct {
	ctx           *web.Context
	service       *LibraryElementService
	reqContext    *models.ReqContext
	user          user.SignedInUser
	folder        *folder.Folder
	initialResult libraryElementResult
	sqlStore      db.DB
}

type folderACLItem struct {
	roleType   org.RoleType
	permission models.PermissionType
}

func createDashboard(t *testing.T, sqlStore db.DB, user user.SignedInUser, dash *models.Dashboard, folderID int64) *models.Dashboard {
	dash.FolderId = folderID
	dashItem := &dashboards.SaveDashboardDTO{
		Dashboard: dash,
		Message:   "",
		OrgId:     user.OrgID,
		User:      &user,
		Overwrite: false,
	}

	cfg := setting.NewCfg()
	cfg.RBACEnabled = false
	features := featuremgmt.WithFeatures()
	cfg.IsFeatureToggleEnabled = features.IsEnabled
	quotaService := quotatest.New(false, nil)
	dashboardStore, err := database.ProvideDashboardStore(sqlStore, cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, cfg), quotaService)
	require.NoError(t, err)
	dashAlertExtractor := alerting.ProvideDashAlertExtractorService(nil, nil, nil)
	ac := acmock.New()
	folderPermissions := acmock.NewMockedPermissionsService()
	dashboardPermissions := acmock.NewMockedPermissionsService()
	service := dashboardservice.ProvideDashboardService(
		cfg, dashboardStore, dashAlertExtractor,
		features, folderPermissions, dashboardPermissions, ac,
	)
	dashboard, err := service.SaveDashboard(context.Background(), dashItem, true)
	require.NoError(t, err)

	return dashboard
}

func createFolderWithACL(t *testing.T, sqlStore db.DB, title string, user user.SignedInUser,
	items []folderACLItem) *folder.Folder {
	t.Helper()

	cfg := setting.NewCfg()
	cfg.RBACEnabled = false
	features := featuremgmt.WithFeatures()
	cfg.IsFeatureToggleEnabled = features.IsEnabled
	ac := acmock.New()
	folderPermissions := acmock.NewMockedPermissionsService()
	dashboardPermissions := acmock.NewMockedPermissionsService()
	quotaService := quotatest.New(false, nil)
	dashboardStore, err := database.ProvideDashboardStore(sqlStore, cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, cfg), quotaService)
	require.NoError(t, err)

	d := dashboardservice.ProvideDashboardService(
		cfg, dashboardStore, nil,
		features, folderPermissions, dashboardPermissions, ac,
	)
	s := folderimpl.ProvideService(ac, bus.ProvideBus(tracing.InitializeTracerForTest()), cfg, d, dashboardStore, nil, features, folderPermissions, nil)
	t.Logf("Creating folder with title and UID %q", title)
	ctx := appcontext.WithUser(context.Background(), &user)
	folder, err := s.Create(ctx, &folder.CreateFolderCommand{
		OrgID: user.OrgID, Title: title, UID: title,
	})
	require.NoError(t, err)

	updateFolderACL(t, dashboardStore, folder.ID, items)

	return folder
}

func updateFolderACL(t *testing.T, dashboardStore *database.DashboardStore, folderID int64, items []folderACLItem) {
	t.Helper()

	if len(items) == 0 {
		return
	}

	var aclItems []*models.DashboardACL
	for _, item := range items {
		role := item.roleType
		permission := item.permission
		aclItems = append(aclItems, &models.DashboardACL{
			DashboardID: folderID,
			Role:        &role,
			Permission:  permission,
			Created:     time.Now(),
			Updated:     time.Now(),
		})
	}

	err := dashboardStore.UpdateDashboardACL(context.Background(), folderID, aclItems)
	require.NoError(t, err)
}

func validateAndUnMarshalResponse(t *testing.T, resp response.Response) libraryElementResult {
	t.Helper()

	require.Equal(t, 200, resp.Status())

	var result = libraryElementResult{}
	err := json.Unmarshal(resp.Body(), &result)
	require.NoError(t, err)

	return result
}

func validateAndUnMarshalConnectionResponse(t *testing.T, resp response.Response) LibraryElementConnectionsResponse {
	t.Helper()
	require.Equal(t, 200, resp.Status())
	var result = LibraryElementConnectionsResponse{}
	err := json.Unmarshal(resp.Body(), &result)
	require.NoError(t, err)
	return result
}

func validateAndUnMarshalArrayResponse(t *testing.T, resp response.Response) libraryElementArrayResult {
	t.Helper()

	require.Equal(t, 200, resp.Status())
	var result = libraryElementArrayResult{}
	err := json.Unmarshal(resp.Body(), &result)
	require.NoError(t, err)

	return result
}

func scenarioWithPanel(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	t.Helper()
	store := dbtest.NewFakeDB()
	guardian.InitLegacyGuardian(store, &dashboards.FakeDashboardService{}, &teamtest.FakeService{})

	testScenario(t, desc, func(t *testing.T, sc scenarioContext) {
		command := getCreatePanelCommand(sc.folder.ID, "Text - Library Panel")
		sc.reqContext.Req.Body = mockRequestBody(command)
		resp := sc.service.createHandler(sc.reqContext)
		sc.initialResult = validateAndUnMarshalResponse(t, resp)

		fn(t, sc)
	})
}

// testScenario is a wrapper around t.Run performing common setup for library panel tests.
// It takes your real test function as a callback.
func testScenario(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	t.Helper()

	t.Run(desc, func(t *testing.T) {
		orgID := int64(1)
		role := org.RoleAdmin
		usr := user.SignedInUser{
			UserID:     1,
			Name:       "Signed In User",
			Login:      "signed_in_user",
			Email:      "signed.in.user@test.com",
			OrgID:      orgID,
			OrgRole:    role,
			LastSeenAt: time.Now(),
		}
		req := &http.Request{
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}
		ctx := appcontext.WithUser(context.Background(), &usr)
		req = req.WithContext(ctx)
		webCtx := web.Context{Req: req}

		sqlStore := db.InitTestDB(t)
		quotaService := quotatest.New(false, nil)
		dashboardStore, err := database.ProvideDashboardStore(sqlStore, sqlStore.Cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, sqlStore.Cfg), quotaService)
		require.NoError(t, err)
		features := featuremgmt.WithFeatures()
		ac := acmock.New().WithDisabled()
		// TODO: Update tests to work with rbac
		sqlStore.Cfg.RBACEnabled = false
		folderPermissions := acmock.NewMockedPermissionsService()
		dashboardPermissions := acmock.NewMockedPermissionsService()
		dashboardService := dashboardservice.ProvideDashboardService(
			sqlStore.Cfg, dashboardStore, nil,
			features, folderPermissions, dashboardPermissions, ac,
		)
		guardian.InitLegacyGuardian(sqlStore, dashboardService, &teamtest.FakeService{})
		service := LibraryElementService{
			Cfg:           sqlStore.Cfg,
			SQLStore:      sqlStore,
			folderService: folderimpl.ProvideService(ac, bus.ProvideBus(tracing.InitializeTracerForTest()), sqlStore.Cfg, dashboardService, dashboardStore, nil, features, folderPermissions, nil),
		}

		// deliberate difference between signed in user and user in db to make it crystal clear
		// what to expect in the tests
		// In the real world these are identical
		cmd := user.CreateUserCommand{
			Email: "user.in.db@test.com",
			Name:  "User In DB",
			Login: userInDbName,
		}

		_, err = sqlStore.CreateUser(context.Background(), cmd)
		require.NoError(t, err)

		sc := scenarioContext{
			user:     usr,
			ctx:      &webCtx,
			service:  &service,
			sqlStore: sqlStore,
			reqContext: &models.ReqContext{
				Context:      &webCtx,
				SignedInUser: &usr,
			},
		}

		sc.folder = createFolderWithACL(t, sc.sqlStore, "ScenarioFolder", sc.user, []folderACLItem{})

		fn(t, sc)
	})
}

func getCompareOptions() []cmp.Option {
	return []cmp.Option{
		cmp.Transformer("Time", func(in time.Time) int64 {
			return in.UTC().Unix()
		}),
	}
}

func mockRequestBody(v interface{}) io.ReadCloser {
	b, _ := json.Marshal(v)
	return io.NopCloser(bytes.NewReader(b))
}
