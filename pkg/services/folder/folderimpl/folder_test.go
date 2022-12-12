package folderimpl

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/appcontext"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol/actest"
	acmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/dashboards"
	dashboardsvc "github.com/grafana/grafana/pkg/services/dashboards/service"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/folder"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

var orgID = int64(1)
var usr = &user.SignedInUser{UserID: 1, OrgID: orgID}

func TestIntegrationProvideFolderService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	t.Run("should register scope resolvers", func(t *testing.T) {
		cfg := setting.NewCfg()
		ac := acmock.New()
		ProvideService(ac, bus.ProvideBus(tracing.InitializeTracerForTest()), cfg, nil, nil, nil, &featuremgmt.FeatureManager{}, nil, nil)

		require.Len(t, ac.Calls.RegisterAttributeScopeResolver, 2)
	})
}

func TestIntegrationFolderService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	t.Run("Folder service tests", func(t *testing.T) {
		dashStore := &dashboards.FakeDashboardStore{}
		db := sqlstore.InitTestDB(t)
		store := ProvideStore(db, db.Cfg, featuremgmt.WithFeatures([]interface{}{"nestedFolders"}))

		cfg := setting.NewCfg()
		cfg.RBACEnabled = false
		features := featuremgmt.WithFeatures()
		cfg.IsFeatureToggleEnabled = features.IsEnabled
		folderPermissions := acmock.NewMockedPermissionsService()
		dashboardPermissions := acmock.NewMockedPermissionsService()
		dashboardService := dashboardsvc.ProvideDashboardService(cfg, dashStore, nil, features, folderPermissions, dashboardPermissions, acmock.New())

		service := &Service{
			cfg:              cfg,
			log:              log.New("test-folder-service"),
			dashboardService: dashboardService,
			dashboardStore:   dashStore,
			store:            store,
			searchService:    nil,
			features:         features,
			permissions:      folderPermissions,
			bus:              bus.ProvideBus(tracing.InitializeTracerForTest()),
		}

		t.Run("Given user has no permissions", func(t *testing.T) {
			origNewGuardian := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{})

			folderId := rand.Int63()
			folderUID := util.GenerateShortUID()

			f := folder.NewFolder("Folder", "")
			f.ID = folderId
			f.UID = folderUID

			dashStore.On("GetFolderByID", mock.Anything, orgID, folderId).Return(f, nil)
			dashStore.On("GetFolderByUID", mock.Anything, orgID, folderUID).Return(f, nil)

			t.Run("When get folder by id should return access denied error", func(t *testing.T) {
				_, err := service.getFolderByID(context.Background(), usr, folderId, orgID)
				require.Equal(t, err, dashboards.ErrFolderAccessDenied)
			})

			t.Run("When get folder by id, with id = 0 should return default folder", func(t *testing.T) {
				foldr, err := service.getFolderByID(context.Background(), usr, 0, orgID)
				require.NoError(t, err)
				require.Equal(t, foldr, &folder.Folder{ID: 0, Title: "General"})
			})

			t.Run("When get folder by uid should return access denied error", func(t *testing.T) {
				_, err := service.getFolderByUID(context.Background(), usr, orgID, folderUID)
				require.Equal(t, err, dashboards.ErrFolderAccessDenied)
			})

			t.Run("When creating folder should return access denied error", func(t *testing.T) {
				dashStore.On("ValidateDashboardBeforeSave", mock.Anything, mock.AnythingOfType("*models.Dashboard"), mock.AnythingOfType("bool")).Return(true, nil).Times(2)
				ctx := appcontext.WithUser(context.Background(), usr)
				_, err := service.Create(ctx, &folder.CreateFolderCommand{
					OrgID: orgID,
					Title: f.Title,
					UID:   folderUID,
				})
				require.Equal(t, err, dashboards.ErrFolderAccessDenied)
			})

			t.Run("When updating folder should return access denied error", func(t *testing.T) {
				dashStore.On("GetDashboard", mock.Anything, mock.AnythingOfType("*models.GetDashboardQuery")).Run(func(args mock.Arguments) {
					folder := args.Get(1).(*models.GetDashboardQuery)
					folder.Result = models.NewDashboard("dashboard-test")
					folder.Result.IsFolder = true
				}).Return(&models.Dashboard{}, nil)
				_, err := service.Update(context.Background(), usr, orgID, folderUID, &models.UpdateFolderCommand{
					Uid:   folderUID,
					Title: "Folder-TEST",
				})
				require.Equal(t, err, dashboards.ErrFolderAccessDenied)
			})

			t.Run("When deleting folder by uid should return access denied error", func(t *testing.T) {
				ctx := context.Background()
				ctx = appcontext.WithUser(ctx, usr)

				newFolder := models.NewFolder("Folder")
				newFolder.Uid = folderUID

				dashStore.On("GetFolderByID", mock.Anything, orgID, folderId).Return(newFolder, nil)
				dashStore.On("GetFolderByUID", mock.Anything, orgID, folderUID).Return(newFolder, nil)

				err := service.DeleteFolder(ctx, &folder.DeleteFolderCommand{
					UID:              folderUID,
					OrgID:            orgID,
					ForceDeleteRules: false,
				})
				require.Error(t, err)
				require.Equal(t, err, dashboards.ErrFolderAccessDenied)
			})

			t.Cleanup(func() {
				guardian.New = origNewGuardian
			})
		})

		t.Run("Given user has permission to save", func(t *testing.T) {
			origNewGuardian := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{CanSaveValue: true})

			t.Run("When creating folder should not return access denied error", func(t *testing.T) {
				dash := models.NewDashboardFolder("Test-Folder")
				dash.Id = rand.Int63()
				f := folder.FromDashboard(dash)

				dashStore.On("ValidateDashboardBeforeSave", mock.Anything, mock.AnythingOfType("*models.Dashboard"), mock.AnythingOfType("bool")).Return(true, nil)
				dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(dash, nil).Once()
				dashStore.On("GetFolderByID", mock.Anything, orgID, dash.Id).Return(f, nil)

				ctx := appcontext.WithUser(context.Background(), usr)
				actualFolder, err := service.Create(ctx, &folder.CreateFolderCommand{
					OrgID: orgID,
					Title: dash.Title,
					UID:   "someuid",
				})
				require.NoError(t, err)
				require.Equal(t, f, actualFolder)
			})

			t.Run("When creating folder should return error if uid is general", func(t *testing.T) {
				dash := models.NewDashboardFolder("Test-Folder")
				dash.Id = rand.Int63()

				ctx := appcontext.WithUser(context.Background(), usr)
				_, err := service.Create(ctx, &folder.CreateFolderCommand{
					OrgID: orgID,
					Title: dash.Title,
					UID:   "general",
				})
				require.ErrorIs(t, err, dashboards.ErrFolderInvalidUID)
			})

			t.Run("When updating folder should not return access denied error", func(t *testing.T) {
				dashboardFolder := models.NewDashboardFolder("Folder")
				dashboardFolder.Id = rand.Int63()
				dashboardFolder.Uid = util.GenerateShortUID()
				f := folder.FromDashboard(dashboardFolder)

				dashStore.On("ValidateDashboardBeforeSave", mock.Anything, mock.AnythingOfType("*models.Dashboard"), mock.AnythingOfType("bool")).Return(true, nil)
				dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(dashboardFolder, nil)
				dashStore.On("GetFolderByID", mock.Anything, orgID, dashboardFolder.Id).Return(f, nil)

				req := &models.UpdateFolderCommand{
					Uid:   dashboardFolder.Uid,
					Title: "TEST-Folder",
				}

				reqResult, err := service.Update(context.Background(), usr, orgID, dashboardFolder.Uid, req)
				require.NoError(t, err)
				require.Equal(t, f, reqResult)
			})

			t.Run("When deleting folder by uid should not return access denied error", func(t *testing.T) {
				f := folder.NewFolder(util.GenerateShortUID(), "")
				f.ID = rand.Int63()
				f.UID = util.GenerateShortUID()
				dashStore.On("GetFolderByUID", mock.Anything, orgID, f.UID).Return(f, nil)

				var actualCmd *models.DeleteDashboardCommand
				dashStore.On("DeleteDashboard", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					actualCmd = args.Get(1).(*models.DeleteDashboardCommand)
				}).Return(nil).Once()

				expectedForceDeleteRules := rand.Int63()%2 == 0
				ctx := context.Background()
				ctx = appcontext.WithUser(ctx, usr)
				err := service.DeleteFolder(ctx, &folder.DeleteFolderCommand{
					UID:              f.UID,
					OrgID:            orgID,
					ForceDeleteRules: expectedForceDeleteRules,
				})
				require.NoError(t, err)
				require.NotNil(t, actualCmd)
				require.Equal(t, f.ID, actualCmd.Id)
				require.Equal(t, orgID, actualCmd.OrgId)
				require.Equal(t, expectedForceDeleteRules, actualCmd.ForceDeleteFolderRules)
			})

			t.Cleanup(func() {
				guardian.New = origNewGuardian
			})
		})

		t.Run("Given user has permission to view", func(t *testing.T) {
			origNewGuardian := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{CanViewValue: true})

			t.Run("When get folder by id should return folder", func(t *testing.T) {
				expected := folder.NewFolder(util.GenerateShortUID(), "")
				expected.ID = rand.Int63()

				dashStore.On("GetFolderByID", mock.Anything, orgID, expected.ID).Return(expected, nil)

				actual, err := service.getFolderByID(context.Background(), usr, expected.ID, orgID)
				require.Equal(t, expected, actual)
				require.NoError(t, err)
			})

			t.Run("When get folder by uid should return folder", func(t *testing.T) {
				expected := folder.NewFolder(util.GenerateShortUID(), "")
				expected.UID = util.GenerateShortUID()

				dashStore.On("GetFolderByUID", mock.Anything, orgID, expected.UID).Return(expected, nil)

				actual, err := service.getFolderByUID(context.Background(), usr, orgID, expected.UID)
				require.Equal(t, expected, actual)
				require.NoError(t, err)
			})

			t.Run("When get folder by title should return folder", func(t *testing.T) {
				expected := folder.NewFolder("TEST-"+util.GenerateShortUID(), "")

				dashStore.On("GetFolderByTitle", mock.Anything, orgID, expected.Title).Return(expected, nil)

				actual, err := service.getFolderByTitle(context.Background(), usr, orgID, expected.Title)
				require.Equal(t, expected, actual)
				require.NoError(t, err)
			})

			t.Cleanup(func() {
				guardian.New = origNewGuardian
			})
		})

		t.Run("Should map errors correct", func(t *testing.T) {
			testCases := []struct {
				ActualError   error
				ExpectedError error
			}{
				{ActualError: dashboards.ErrDashboardTitleEmpty, ExpectedError: dashboards.ErrFolderTitleEmpty},
				{ActualError: dashboards.ErrDashboardUpdateAccessDenied, ExpectedError: dashboards.ErrFolderAccessDenied},
				{ActualError: dashboards.ErrDashboardWithSameNameInFolderExists, ExpectedError: dashboards.ErrFolderSameNameExists},
				{ActualError: dashboards.ErrDashboardWithSameUIDExists, ExpectedError: dashboards.ErrFolderWithSameUIDExists},
				{ActualError: dashboards.ErrDashboardVersionMismatch, ExpectedError: dashboards.ErrFolderVersionMismatch},
				{ActualError: dashboards.ErrDashboardNotFound, ExpectedError: dashboards.ErrFolderNotFound},
				{ActualError: dashboards.ErrDashboardFailedGenerateUniqueUid, ExpectedError: dashboards.ErrFolderFailedGenerateUniqueUid},
				{ActualError: dashboards.ErrDashboardInvalidUid, ExpectedError: dashboards.ErrDashboardInvalidUid},
			}

			for _, tc := range testCases {
				actualError := toFolderError(tc.ActualError)
				assert.EqualErrorf(t, actualError, tc.ExpectedError.Error(),
					"For error '%s' expected error '%s', actual '%s'", tc.ActualError, tc.ExpectedError, actualError)
			}
		})
	})
}

func TestNestedFolderServiceFeatureToggle(t *testing.T) {
	folderStore := NewFakeStore()

	dashboardsvc := dashboards.FakeDashboardService{}
	dashboardsvc.On("BuildSaveDashboardCommand",
		mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"),
		mock.AnythingOfType("bool"), mock.AnythingOfType("bool")).Return(&models.SaveDashboardCommand{}, nil)
	dashStore := dashboards.FakeDashboardStore{}
	dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(&models.Dashboard{}, nil)
	dashStore.On("GetFolderByID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(&folder.Folder{}, nil)
	cfg := setting.NewCfg()
	cfg.RBACEnabled = false
	nestedFoldersEnabled := true
	features := featuremgmt.WithFeatures()
	cfg.IsFeatureToggleEnabled = func(key string) bool {
		if key == featuremgmt.FlagNestedFolders {
			return nestedFoldersEnabled
		}
		return false
	}
	cfg.IsFeatureToggleEnabled = features.IsEnabled
	folderService := &Service{
		cfg:              cfg,
		store:            folderStore,
		dashboardStore:   &dashStore,
		dashboardService: &dashboardsvc,
		features:         features,
	}
	t.Run("create folder", func(t *testing.T) {
		folderStore.ExpectedFolder = &folder.Folder{}
		ctx := appcontext.WithUser(context.Background(), usr)
		res, err := folderService.Create(ctx, &folder.CreateFolderCommand{})
		require.NoError(t, err)
		require.NotNil(t, res.UID)
	})

	t.Run("get parents folder", func(t *testing.T) {
		folderStore.ExpectedFolder = &folder.Folder{}
		_, err := folderService.GetParents(context.Background(), &folder.GetParentsQuery{})
		require.NoError(t, err)
	})

	t.Run("get children folder", func(t *testing.T) {
		folderStore.ExpectedFolders = []*folder.Folder{
			{
				UID: "test",
			},
			{
				UID: "test2",
			},
			{
				UID: "test3",
			},
			{
				UID: "test4",
			},
		}
		res, err := folderService.GetTree(context.Background(),
			&folder.GetTreeQuery{
				UID: "test",
			})
		require.NoError(t, err)
		require.Equal(t, 4, len(res))
	})
}

func TestNestedFolderService(t *testing.T) {
	t.Run("with feature flag unset", func(t *testing.T) {
		ctx := appcontext.WithUser(context.Background(), usr)
		store := &FakeStore{}
		dashStore := dashboards.FakeDashboardStore{}
		dashboardsvc := dashboards.FakeDashboardService{}
		// nothing enabled yet
		cfg := setting.NewCfg()
		cfg.RBACEnabled = false
		features := featuremgmt.WithFeatures()
		cfg.IsFeatureToggleEnabled = features.IsEnabled
		foldersvc := &Service{
			cfg:              cfg,
			log:              log.New("test-folder-service"),
			dashboardService: &dashboardsvc,
			dashboardStore:   &dashStore,
			store:            store,
			features:         features,
		}

		t.Run("When create folder, no create in folder table done", func(t *testing.T) {
			// dashboard store & service commands that should be called.
			dashboardsvc.On("BuildSaveDashboardCommand",
				mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"),
				mock.AnythingOfType("bool"), mock.AnythingOfType("bool")).Return(&models.SaveDashboardCommand{}, nil)
			dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(&models.Dashboard{}, nil)
			dashStore.On("GetFolderByID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(&folder.Folder{}, nil)

			ctx = appcontext.WithUser(ctx, usr)
			_, err := foldersvc.Create(ctx, &folder.CreateFolderCommand{
				OrgID: orgID,
				Title: "myFolder",
				UID:   "myFolder",
			})
			require.NoError(t, err)
			// CreateFolder should not call the folder store create if the feature toggle is not enabled.
			require.False(t, store.CreateCalled)
		})

		t.Run("When delete folder, no delete in folder table done", func(t *testing.T) {
			var actualCmd *models.DeleteDashboardCommand
			dashStore.On("DeleteDashboard", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				actualCmd = args.Get(1).(*models.DeleteDashboardCommand)
			}).Return(nil).Once()
			dashStore.On("GetFolderByUID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(&folder.Folder{}, nil)

			g := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{CanSaveValue: true})

			err := foldersvc.DeleteFolder(ctx, &folder.DeleteFolderCommand{UID: "myFolder", OrgID: orgID})
			require.NoError(t, err)
			require.NotNil(t, actualCmd)

			t.Cleanup(func() {
				guardian.New = g
			})
			require.False(t, store.DeleteCalled)
		})
	})

	t.Run("with nested folder feature flag on", func(t *testing.T) {
		ctx := appcontext.WithUser(context.Background(), usr)
		store := &FakeStore{}
		dashStore := &dashboards.FakeDashboardStore{}
		dashboardsvc := &dashboards.FakeDashboardService{}
		// nothing enabled yet
		cfg := setting.NewCfg()
		cfg.RBACEnabled = false
		features := featuremgmt.WithFeatures("nestedFolders")
		cfg.IsFeatureToggleEnabled = features.IsEnabled
		foldersvc := &Service{
			cfg:              cfg,
			log:              log.New("test-folder-service"),
			dashboardService: dashboardsvc,
			dashboardStore:   dashStore,
			store:            store,
			features:         features,
			accessControl: actest.FakeAccessControl{
				ExpectedEvaluate: true,
			},
		}

		t.Run("create, no error", func(t *testing.T) {
			// dashboard store & service commands that should be called.
			dashboardsvc.On("BuildSaveDashboardCommand",
				mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"),
				mock.AnythingOfType("bool"), mock.AnythingOfType("bool")).Return(&models.SaveDashboardCommand{}, nil)
			dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(&models.Dashboard{}, nil)
			dashStore.On("GetFolderByID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(&folder.Folder{}, nil)
			ctx = appcontext.WithUser(ctx, usr)
			_, err := foldersvc.Create(ctx, &folder.CreateFolderCommand{
				OrgID: orgID,
				Title: "myFolder",
				UID:   "myFolder",
			})
			require.NoError(t, err)
			// CreateFolder should also call the folder store's create method.
			require.True(t, store.CreateCalled)
		})

		t.Run("create returns error from nested folder service", func(t *testing.T) {
			// This test creates and deletes the dashboard, so needs some extra setup.
			g := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{})

			// dashboard store & service commands that should be called.
			dashboardsvc.On("BuildSaveDashboardCommand",
				mock.Anything, mock.AnythingOfType("*dashboards.SaveDashboardDTO"),
				mock.AnythingOfType("bool"), mock.AnythingOfType("bool")).Return(&models.SaveDashboardCommand{}, nil)
			dashStore.On("SaveDashboard", mock.Anything, mock.AnythingOfType("models.SaveDashboardCommand")).Return(&models.Dashboard{}, nil)
			dashStore.On("GetFolderByID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(&folder.Folder{}, nil)
			dashStore.On("GetFolderByUID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(&folder.Folder{}, nil)

			// return an error from the folder store
			store.ExpectedError = errors.New("FAILED")

			// the service return success as long as the legacy create succeeds
			ctx = appcontext.WithUser(ctx, usr)
			_, err := foldersvc.Create(ctx, &folder.CreateFolderCommand{
				OrgID: orgID,
				Title: "myFolder",
				UID:   "myFolder",
			})
			require.Error(t, err, "FAILED")

			// CreateFolder should also call the folder store's create method.
			require.True(t, store.CreateCalled)

			t.Cleanup(func() {
				guardian.New = g
			})
		})

		t.Run("move, no error", func(t *testing.T) {
			store.ExpectedError = nil
			store.ExpectedFolder = &folder.Folder{UID: "myFolder", ParentUID: "newFolder"}
			f, err := foldersvc.Move(ctx, &folder.MoveFolderCommand{UID: "myFolder", NewParentUID: "newFolder", OrgID: orgID})
			require.NoError(t, err)
			require.NotNil(t, f)
		})

		t.Run("delete with success", func(t *testing.T) {
			var actualCmd *models.DeleteDashboardCommand
			dashStore.On("DeleteDashboard", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				actualCmd = args.Get(1).(*models.DeleteDashboardCommand)
			}).Return(nil).Once()
			dashStore.On("GetFolderByUID", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(&models.Folder{}, nil)

			g := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{CanSaveValue: true})

			err := foldersvc.DeleteFolder(ctx, &folder.DeleteFolderCommand{UID: "myFolder", OrgID: orgID})
			require.NoError(t, err)
			require.NotNil(t, actualCmd)

			t.Cleanup(func() {
				guardian.New = g
			})
			require.True(t, store.DeleteCalled)
		})
	})
}
