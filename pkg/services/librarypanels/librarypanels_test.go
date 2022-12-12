package librarypanels

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/routing"
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
	"github.com/grafana/grafana/pkg/services/libraryelements"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/quota/quotatest"
	"github.com/grafana/grafana/pkg/services/tag/tagimpl"
	"github.com/grafana/grafana/pkg/services/team/teamtest"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
)

const userInDbName = "user_in_db"
const userInDbAvatar = "/avatar/402d08de060496d6b6874495fe20f5ad"

func TestConnectLibraryPanelsForDashboard(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel, it should connect the two",
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
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid": sc.initialResult.Result.UID,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Title: "Testing ConnectLibraryPanelsForDashboard",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.Id)

			err := sc.service.ConnectLibraryPanelsForDashboard(sc.ctx, sc.user, dashInDB)
			require.NoError(t, err)

			elements, err := sc.elementService.GetElementsForDashboard(sc.ctx, dashInDB.Id)
			require.NoError(t, err)
			require.Len(t, elements, 1)
			require.Equal(t, sc.initialResult.Result.UID, elements[sc.initialResult.Result.UID].UID)
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with library panels inside and outside of rows, it should connect all",
		func(t *testing.T, sc scenarioContext) {
			cmd := libraryelements.CreateLibraryElementCommand{
				FolderID: sc.initialResult.Result.FolderID,
				Name:     "Outside row",
				Model: []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "id": 1,
			  "title": "Text - Library Panel",
			  "type": "text",
			  "description": "A description"
			}
		`),
				Kind: int64(models.PanelElement),
			}
			outsidePanel, err := sc.elementService.CreateElement(sc.ctx, sc.user, cmd)
			require.NoError(t, err)
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
						"collapsed": true,
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 6,
						},
						"id":   int64(2),
						"type": "row",
						"panels": []interface{}{
							map[string]interface{}{
								"id": int64(3),
								"gridPos": map[string]interface{}{
									"h": 6,
									"w": 6,
									"x": 0,
									"y": 7,
								},
							},
							map[string]interface{}{
								"id": int64(4),
								"gridPos": map[string]interface{}{
									"h": 6,
									"w": 6,
									"x": 6,
									"y": 13,
								},
								"datasource": "${DS_GDEV-TESTDATA}",
								"libraryPanel": map[string]interface{}{
									"uid": sc.initialResult.Result.UID,
								},
								"title": "Inside row",
								"type":  "text",
							},
						},
					},
					map[string]interface{}{
						"id": int64(5),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 19,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid": outsidePanel.UID,
						},
						"title": "Outside row",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Title: "Testing ConnectLibraryPanelsForDashboard",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.Id)

			err = sc.service.ConnectLibraryPanelsForDashboard(sc.ctx, sc.user, dashInDB)
			require.NoError(t, err)

			elements, err := sc.elementService.GetElementsForDashboard(sc.ctx, dashInDB.Id)
			require.NoError(t, err)
			require.Len(t, elements, 2)
			require.Equal(t, sc.initialResult.Result.UID, elements[sc.initialResult.Result.UID].UID)
			require.Equal(t, outsidePanel.UID, elements[outsidePanel.UID].UID)
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel without uid, it should fail",
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
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Title: "Testing ConnectLibraryPanelsForDashboard",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.Id)

			err := sc.service.ConnectLibraryPanelsForDashboard(sc.ctx, sc.user, dashInDB)
			require.EqualError(t, err, errLibraryPanelHeaderUIDMissing.Error())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with unused/removed library panels, it should disconnect unused/removed library panels",
		func(t *testing.T, sc scenarioContext) {
			unused, err := sc.elementService.CreateElement(sc.ctx, sc.user, libraryelements.CreateLibraryElementCommand{
				FolderID: sc.folder.Id,
				Name:     "Unused Libray Panel",
				Model: []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "id": 4,
			  "title": "Unused Libray Panel",
			  "type": "text",
			  "description": "Unused description"
			}
		`),
				Kind: int64(models.PanelElement),
			})
			require.NoError(t, err)
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
						"id": int64(4),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid": unused.UID,
						},
						"title":       "Unused Libray Panel",
						"description": "Unused description",
					},
				},
			}

			dash := models.Dashboard{
				Title: "Testing ConnectLibraryPanelsForDashboard",
				Data:  simplejson.NewFromAny(dashJSON),
			}
			dashInDB := createDashboard(t, sc.sqlStore, sc.user, &dash, sc.folder.Id)
			err = sc.elementService.ConnectElementsToDashboard(sc.ctx, sc.user, []string{sc.initialResult.Result.UID}, dashInDB.Id)
			require.NoError(t, err)

			panelJSON := []interface{}{
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
					"datasource": "${DS_GDEV-TESTDATA}",
					"libraryPanel": map[string]interface{}{
						"uid": sc.initialResult.Result.UID,
					},
					"title": "Text - Library Panel",
					"type":  "text",
				},
			}
			dashInDB.Data.Set("panels", panelJSON)
			err = sc.service.ConnectLibraryPanelsForDashboard(sc.ctx, sc.user, dashInDB)
			require.NoError(t, err)

			elements, err := sc.elementService.GetElementsForDashboard(sc.ctx, dashInDB.Id)
			require.NoError(t, err)
			require.Len(t, elements, 1)
			require.Equal(t, sc.initialResult.Result.UID, elements[sc.initialResult.Result.UID].UID)
		})
}

func TestImportLibraryPanelsForDashboard(t *testing.T) {
	testScenario(t, "When an admin tries to import a dashboard with a library panel that does not exist, it should import the library panel",
		func(t *testing.T, sc scenarioContext) {
			var missingUID = "jL6MrxCMz"
			var missingName = "Missing Library Panel"
			var missingModel = map[string]interface{}{
				"id": int64(2),
				"gridPos": map[string]interface{}{
					"h": int64(6),
					"w": int64(6),
					"x": int64(0),
					"y": int64(0),
				},
				"description": "",
				"datasource":  "${DS_GDEV-TESTDATA}",
				"libraryPanel": map[string]interface{}{
					"uid":  missingUID,
					"name": missingName,
				},
				"title": "Text - Library Panel",
				"type":  "text",
			}
			var libraryElements = map[string]interface{}{
				missingUID: map[string]interface{}{
					"model": missingModel,
				},
			}

			panels := []interface{}{
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
					"libraryPanel": map[string]interface{}{
						"uid":  missingUID,
						"name": missingName,
					},
				},
			}

			_, err := sc.elementService.GetElement(sc.ctx, sc.user, missingUID)

			require.EqualError(t, err, libraryelements.ErrLibraryElementNotFound.Error())

			err = sc.service.ImportLibraryPanelsForDashboard(sc.ctx, sc.user, simplejson.NewFromAny(libraryElements), panels, 0)
			require.NoError(t, err)

			element, err := sc.elementService.GetElement(sc.ctx, sc.user, missingUID)
			require.NoError(t, err)
			var expected = getExpected(t, element, missingUID, missingName, missingModel)
			var result = toLibraryElement(t, element)
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to import a dashboard with a library panel that already exist, it should not import the library panel and existing library panel should be unchanged",
		func(t *testing.T, sc scenarioContext) {
			var existingUID = sc.initialResult.Result.UID
			var existingName = sc.initialResult.Result.Name

			panels := []interface{}{
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
					"libraryPanel": map[string]interface{}{
						"uid":  sc.initialResult.Result.UID,
						"name": sc.initialResult.Result.Name,
					},
				},
			}

			_, err := sc.elementService.GetElement(sc.ctx, sc.user, existingUID)
			require.NoError(t, err)

			err = sc.service.ImportLibraryPanelsForDashboard(sc.ctx, sc.user, simplejson.New(), panels, sc.folder.Id)
			require.NoError(t, err)

			element, err := sc.elementService.GetElement(sc.ctx, sc.user, existingUID)
			require.NoError(t, err)
			var expected = getExpected(t, element, existingUID, existingName, sc.initialResult.Result.Model)
			expected.FolderID = sc.initialResult.Result.FolderID
			expected.Description = sc.initialResult.Result.Description
			expected.Meta.FolderUID = sc.folder.Uid
			expected.Meta.FolderName = sc.folder.Title
			var result = toLibraryElement(t, element)
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	testScenario(t, "When an admin tries to import a dashboard with library panels inside and outside of rows, it should import all that do not exist",
		func(t *testing.T, sc scenarioContext) {
			var outsideUID = "jL6MrxCMz"
			var outsideName = "Outside Library Panel"
			var outsideModel = map[string]interface{}{
				"id": int64(5),
				"gridPos": map[string]interface{}{
					"h": 6,
					"w": 6,
					"x": 0,
					"y": 19,
				},
				"datasource": "${DS_GDEV-TESTDATA}",
				"libraryPanel": map[string]interface{}{
					"uid":  outsideUID,
					"name": outsideName,
				},
				"title": "Outside row",
				"type":  "text",
			}

			var insideUID = "iK7NsyDNz"
			var insideName = "Inside Library Panel"
			var insideModel = map[string]interface{}{
				"id": int64(4),
				"gridPos": map[string]interface{}{
					"h": 6,
					"w": 6,
					"x": 6,
					"y": 13,
				},
				"datasource": "${DS_GDEV-TESTDATA}",
				"libraryPanel": map[string]interface{}{
					"uid":  insideUID,
					"name": insideName,
				},
				"title": "Inside row",
				"type":  "text",
			}

			var libraryElements = map[string]interface{}{
				outsideUID: map[string]interface{}{
					"model": outsideModel,
				},
				insideUID: map[string]interface{}{
					"model": insideModel,
				},
			}

			panels := []interface{}{
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
					"libraryPanel": map[string]interface{}{
						"uid":  outsideUID,
						"name": outsideName,
					},
				},
				map[string]interface{}{
					"collapsed": true,
					"gridPos": map[string]interface{}{
						"h": 6,
						"w": 6,
						"x": 0,
						"y": 6,
					},
					"id":   int64(2),
					"type": "row",
					"panels": []interface{}{
						map[string]interface{}{
							"id": int64(3),
							"gridPos": map[string]interface{}{
								"h": 6,
								"w": 6,
								"x": 0,
								"y": 7,
							},
						},
						map[string]interface{}{
							"libraryPanel": map[string]interface{}{
								"uid":  insideUID,
								"name": insideName,
							},
						},
					},
				},
			}

			_, err := sc.elementService.GetElement(sc.ctx, sc.user, outsideUID)
			require.EqualError(t, err, libraryelements.ErrLibraryElementNotFound.Error())
			_, err = sc.elementService.GetElement(sc.ctx, sc.user, insideUID)
			require.EqualError(t, err, libraryelements.ErrLibraryElementNotFound.Error())

			err = sc.service.ImportLibraryPanelsForDashboard(sc.ctx, sc.user, simplejson.NewFromAny(libraryElements), panels, 0)
			require.NoError(t, err)

			element, err := sc.elementService.GetElement(sc.ctx, sc.user, outsideUID)
			require.NoError(t, err)
			expected := getExpected(t, element, outsideUID, outsideName, outsideModel)
			result := toLibraryElement(t, element)
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}

			element, err = sc.elementService.GetElement(sc.ctx, sc.user, insideUID)
			require.NoError(t, err)
			expected = getExpected(t, element, insideUID, insideName, insideModel)
			result = toLibraryElement(t, element)
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})
}

type libraryPanel struct {
	ID          int64
	OrgID       int64
	FolderID    int64
	UID         string
	Name        string
	Type        string
	Description string
	Model       map[string]interface{}
	Version     int64
	Meta        libraryelements.LibraryElementDTOMeta
}

type libraryElementGridPos struct {
	H int64 `json:"h"`
	W int64 `json:"w"`
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

type libraryElementLibraryPanel struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type libraryElementModel struct {
	ID           int64                      `json:"id"`
	Datasource   string                     `json:"datasource"`
	Description  string                     `json:"description"`
	Title        string                     `json:"title"`
	Type         string                     `json:"type"`
	GridPos      libraryElementGridPos      `json:"gridPos"`
	LibraryPanel libraryElementLibraryPanel `json:"libraryPanel"`
}

type libraryElement struct {
	ID          int64                                 `json:"id"`
	OrgID       int64                                 `json:"orgId"`
	FolderID    int64                                 `json:"folderId"`
	UID         string                                `json:"uid"`
	Name        string                                `json:"name"`
	Kind        int64                                 `json:"kind"`
	Type        string                                `json:"type"`
	Description string                                `json:"description"`
	Model       libraryElementModel                   `json:"model"`
	Version     int64                                 `json:"version"`
	Meta        libraryelements.LibraryElementDTOMeta `json:"meta"`
}

type libraryPanelResult struct {
	Result libraryPanel `json:"result"`
}

type scenarioContext struct {
	ctx            context.Context
	service        Service
	elementService libraryelements.Service
	user           *user.SignedInUser
	folder         *models.Folder
	initialResult  libraryPanelResult
	sqlStore       db.DB
}

type folderACLItem struct {
	roleType   org.RoleType
	permission models.PermissionType
}

func toLibraryElement(t *testing.T, res libraryelements.LibraryElementDTO) libraryElement {
	var model = libraryElementModel{}
	err := json.Unmarshal(res.Model, &model)
	require.NoError(t, err)

	return libraryElement{
		ID:          res.ID,
		OrgID:       res.OrgID,
		FolderID:    res.FolderID,
		UID:         res.UID,
		Name:        res.Name,
		Type:        res.Type,
		Description: res.Description,
		Kind:        res.Kind,
		Model:       model,
		Version:     res.Version,
		Meta: libraryelements.LibraryElementDTOMeta{
			FolderName:          res.Meta.FolderName,
			FolderUID:           res.Meta.FolderUID,
			ConnectedDashboards: res.Meta.ConnectedDashboards,
			Created:             res.Meta.Created,
			Updated:             res.Meta.Updated,
			CreatedBy: libraryelements.LibraryElementDTOMetaUser{
				ID:        res.Meta.CreatedBy.ID,
				Name:      res.Meta.CreatedBy.Name,
				AvatarURL: res.Meta.CreatedBy.AvatarURL,
			},
			UpdatedBy: libraryelements.LibraryElementDTOMetaUser{
				ID:        res.Meta.UpdatedBy.ID,
				Name:      res.Meta.UpdatedBy.Name,
				AvatarURL: res.Meta.UpdatedBy.AvatarURL,
			},
		},
	}
}

func getExpected(t *testing.T, res libraryelements.LibraryElementDTO, UID string, name string, model map[string]interface{}) libraryElement {
	marshalled, err := json.Marshal(model)
	require.NoError(t, err)
	var libModel libraryElementModel
	err = json.Unmarshal(marshalled, &libModel)
	require.NoError(t, err)

	return libraryElement{
		ID:          res.ID,
		OrgID:       1,
		FolderID:    0,
		UID:         UID,
		Name:        name,
		Type:        "text",
		Description: "",
		Kind:        1,
		Model:       libModel,
		Version:     1,
		Meta: libraryelements.LibraryElementDTOMeta{
			FolderName:          "General",
			FolderUID:           "",
			ConnectedDashboards: 0,
			Created:             res.Meta.Created,
			Updated:             res.Meta.Updated,
			CreatedBy: libraryelements.LibraryElementDTOMetaUser{
				ID:        1,
				Name:      userInDbName,
				AvatarURL: userInDbAvatar,
			},
			UpdatedBy: libraryelements.LibraryElementDTOMetaUser{
				ID:        1,
				Name:      userInDbName,
				AvatarURL: userInDbAvatar,
			},
		},
	}
}

func createDashboard(t *testing.T, sqlStore db.DB, user *user.SignedInUser, dash *models.Dashboard, folderID int64) *models.Dashboard {
	dash.FolderId = folderID
	dashItem := &dashboards.SaveDashboardDTO{
		Dashboard: dash,
		Message:   "",
		OrgId:     user.OrgID,
		User:      user,
		Overwrite: false,
	}

	cfg := setting.NewCfg()
	cfg.RBACEnabled = false
	cfg.IsFeatureToggleEnabled = featuremgmt.WithFeatures().IsEnabled
	quotaService := quotatest.New(false, nil)
	dashboardStore, err := database.ProvideDashboardStore(sqlStore, cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, cfg), quotaService)
	require.NoError(t, err)
	dashAlertService := alerting.ProvideDashAlertExtractorService(nil, nil, nil)
	ac := acmock.New()
	service := dashboardservice.ProvideDashboardService(
		cfg, dashboardStore, dashAlertService,
		featuremgmt.WithFeatures(), acmock.NewMockedPermissionsService(), acmock.NewMockedPermissionsService(), ac,
	)
	dashboard, err := service.SaveDashboard(context.Background(), dashItem, true)
	require.NoError(t, err)

	return dashboard
}

func createFolderWithACL(t *testing.T, sqlStore db.DB, title string, user *user.SignedInUser,
	items []folderACLItem) *folder.Folder {
	t.Helper()

	ac := acmock.New()
	cfg := setting.NewCfg()
	cfg.RBACEnabled = false
	cfg.IsFeatureToggleEnabled = featuremgmt.WithFeatures().IsEnabled
	features := featuremgmt.WithFeatures()
	folderPermissions := acmock.NewMockedPermissionsService()
	dashboardPermissions := acmock.NewMockedPermissionsService()
	quotaService := quotatest.New(false, nil)
	dashboardStore, err := database.ProvideDashboardStore(sqlStore, cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, cfg), quotaService)
	require.NoError(t, err)
	d := dashboardservice.ProvideDashboardService(cfg, dashboardStore, nil, features, folderPermissions, dashboardPermissions, ac)
	s := folderimpl.ProvideService(ac, bus.ProvideBus(tracing.InitializeTracerForTest()), cfg, d, dashboardStore, nil, features, folderPermissions, nil)

	t.Logf("Creating folder with title and UID %q", title)
	ctx := appcontext.WithUser(context.Background(), user)
	folder, err := s.Create(ctx, &folder.CreateFolderCommand{OrgID: user.OrgID, Title: title, UID: title})
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

func scenarioWithLibraryPanel(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	store := dbtest.NewFakeDB()
	guardian.InitLegacyGuardian(store, &dashboards.FakeDashboardService{}, &teamtest.FakeService{})
	t.Helper()

	testScenario(t, desc, func(t *testing.T, sc scenarioContext) {
		command := libraryelements.CreateLibraryElementCommand{
			FolderID: sc.folder.Id,
			Name:     "Text - Library Panel",
			Model: []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "id": 1,
			  "title": "Text - Library Panel",
			  "type": "text",
			  "description": "A description"
			}
		`),
			Kind: int64(models.PanelElement),
		}
		resp, err := sc.elementService.CreateElement(sc.ctx, sc.user, command)
		require.NoError(t, err)
		var model map[string]interface{}
		err = json.Unmarshal(resp.Model, &model)
		require.NoError(t, err)

		sc.initialResult = libraryPanelResult{
			Result: libraryPanel{
				ID:          resp.ID,
				OrgID:       resp.OrgID,
				FolderID:    resp.FolderID,
				UID:         resp.UID,
				Name:        resp.Name,
				Type:        resp.Type,
				Description: resp.Description,
				Model:       model,
				Version:     resp.Version,
				Meta:        resp.Meta,
			},
		}

		fn(t, sc)
	})
}

// testScenario is a wrapper around t.Run performing common setup for library panel tests.
// It takes your real test function as a callback.
func testScenario(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	t.Helper()

	t.Run(desc, func(t *testing.T) {
		cfg := setting.NewCfg()
		cfg.RBACEnabled = false
		orgID := int64(1)
		role := org.RoleAdmin
		sqlStore, cfg := db.InitTestDBwithCfg(t)
		quotaService := quotatest.New(false, nil)
		dashboardStore, err := database.ProvideDashboardStore(sqlStore, cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, sqlStore.Cfg), quotaService)
		require.NoError(t, err)

		features := featuremgmt.WithFeatures()
		ac := acmock.New()
		folderPermissions := acmock.NewMockedPermissionsService()
		dashboardPermissions := acmock.NewMockedPermissionsService()

		dashboardService := dashboardservice.ProvideDashboardService(
			cfg, dashboardStore, &alerting.DashAlertExtractorService{},
			features, folderPermissions, dashboardPermissions, ac,
		)
		folderService := folderimpl.ProvideService(ac, bus.ProvideBus(tracing.InitializeTracerForTest()), cfg, dashboardService, dashboardStore, nil, features, folderPermissions, nil)

		elementService := libraryelements.ProvideService(cfg, sqlStore, routing.NewRouteRegister(), folderService)
		service := LibraryPanelService{
			Cfg:                   cfg,
			SQLStore:              sqlStore,
			LibraryElementService: elementService,
		}

		usr := &user.SignedInUser{
			UserID:     1,
			Name:       "Signed In User",
			Login:      "signed_in_user",
			Email:      "signed.in.user@test.com",
			OrgID:      orgID,
			OrgRole:    role,
			LastSeenAt: time.Now(),
		}

		// deliberate difference between signed in user and user in db to make it crystal clear
		// what to expect in the tests
		// In the real world these are identical
		cmd := user.CreateUserCommand{
			Email: "user.in.db@test.com",
			Name:  "User In DB",
			Login: userInDbName,
		}

		ctx := appcontext.WithUser(context.Background(), usr)

		_, err = sqlStore.CreateUser(ctx, cmd)
		require.NoError(t, err)

		sc := scenarioContext{
			user:           usr,
			ctx:            ctx,
			service:        &service,
			elementService: elementService,
			sqlStore:       sqlStore,
		}

		sc.folder = createFolderWithACL(t, sc.sqlStore, "ScenarioFolder", sc.user, []folderACLItem{}).ToLegacyModel()

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
