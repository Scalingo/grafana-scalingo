//go:build integration
// +build integration

package service

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	acmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

var orgID = int64(1)
var user = &models.SignedInUser{UserId: 1}

func TestProvideFolderService(t *testing.T) {
	t.Run("should register scope resolvers", func(t *testing.T) {
		store := &dashboards.FakeDashboardStore{}
		cfg := setting.NewCfg()
		features := featuremgmt.WithFeatures()
		cfg.IsFeatureToggleEnabled = features.IsEnabled
		permissionsServices := acmock.NewPermissionsServicesMock()
		dashboardService := ProvideDashboardService(cfg, store, nil, features, permissionsServices)
		ac := acmock.New()

		ProvideFolderService(
			cfg, &dashboards.FakeDashboardService{DashboardService: dashboardService},
			store, nil, features, permissionsServices, ac, mockstore.NewSQLStoreMock(),
		)

		require.Len(t, ac.Calls.RegisterAttributeScopeResolver, 2)
	})
}

func TestFolderService(t *testing.T) {
	t.Run("Folder service tests", func(t *testing.T) {
		store := &dashboards.FakeDashboardStore{}
		cfg := setting.NewCfg()
		features := featuremgmt.WithFeatures()
		cfg.IsFeatureToggleEnabled = features.IsEnabled
		permissionsServices := acmock.NewPermissionsServicesMock()
		dashboardService := ProvideDashboardService(cfg, store, nil, features, permissionsServices)
		mockStore := mockstore.NewSQLStoreMock()

		service := FolderServiceImpl{
			cfg:              cfg,
			log:              log.New("test-folder-service"),
			dashboardService: dashboardService,
			dashboardStore:   store,
			searchService:    nil,
			features:         features,
			permissions:      permissionsServices.GetFolderService(),
			sqlStore:         mockStore,
		}

		t.Run("Given user has no permissions", func(t *testing.T) {
			origNewGuardian := guardian.New
			guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{})

			folderId := rand.Int63()
			folderUID := util.GenerateShortUID()

			folder := models.NewFolder("Folder")
			folder.Id = folderId
			folder.Uid = folderUID

			store.On("GetFolderByID", mock.Anything, orgID, folderId).Return(folder, nil)
			store.On("GetFolderByUID", mock.Anything, orgID, folderUID).Return(folder, nil)

			t.Run("When get folder by id should return access denied error", func(t *testing.T) {
				_, err := service.GetFolderByID(context.Background(), user, folderId, orgID)
				require.Equal(t, err, models.ErrFolderAccessDenied)
			})

			t.Run("When get folder by id, with id = 0 should return default folder", func(t *testing.T) {
				folder, err := service.GetFolderByID(context.Background(), user, 0, orgID)
				require.NoError(t, err)
				require.Equal(t, folder, &models.Folder{Id: 0, Title: "General"})
			})

			t.Run("When get folder by uid should return access denied error", func(t *testing.T) {
				_, err := service.GetFolderByUID(context.Background(), user, orgID, folderUID)
				require.Equal(t, err, models.ErrFolderAccessDenied)
			})

			t.Run("When creating folder should return access denied error", func(t *testing.T) {
				store.On("ValidateDashboardBeforeSave", mock.Anything, mock.Anything).Return(true, nil).Times(2)
				_, err := service.CreateFolder(context.Background(), user, orgID, folder.Title, folderUID)
				require.Equal(t, err, models.ErrFolderAccessDenied)
			})

			t.Run("When updating folder should return access denied error", func(t *testing.T) {
				mockStore.ExpectedDashboard = models.NewDashboardFolder("Folder")
				err := service.UpdateFolder(context.Background(), user, orgID, folderUID, &models.UpdateFolderCommand{
					Uid:   folderUID,
					Title: "Folder-TEST",
				})
				require.Equal(t, err, models.ErrFolderAccessDenied)
			})

			t.Run("When deleting folder by uid should return access denied error", func(t *testing.T) {
				_, err := service.DeleteFolder(context.Background(), user, orgID, folderUID, false)
				require.Error(t, err)
				require.Equal(t, err, models.ErrFolderAccessDenied)
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
				f := models.DashboardToFolder(dash)

				store.On("ValidateDashboardBeforeSave", mock.Anything, mock.Anything).Return(true, nil)
				store.On("SaveDashboard", mock.Anything).Return(dash, nil).Once()
				store.On("GetFolderByID", mock.Anything, orgID, dash.Id).Return(f, nil)

				actualFolder, err := service.CreateFolder(context.Background(), user, orgID, dash.Title, "")
				require.NoError(t, err)
				require.Equal(t, f, actualFolder)
			})

			t.Run("When creating folder should return error if uid is general", func(t *testing.T) {
				dash := models.NewDashboardFolder("Test-Folder")
				dash.Id = rand.Int63()

				_, err := service.CreateFolder(context.Background(), user, orgID, dash.Title, "general")
				require.ErrorIs(t, err, models.ErrFolderInvalidUID)
			})

			t.Run("When updating folder should not return access denied error", func(t *testing.T) {
				dashboardFolder := models.NewDashboardFolder("Folder")
				dashboardFolder.Id = rand.Int63()
				dashboardFolder.Uid = util.GenerateShortUID()
				f := models.DashboardToFolder(dashboardFolder)

				mockStore.ExpectedDashboard = dashboardFolder

				store.On("ValidateDashboardBeforeSave", mock.Anything, mock.Anything).Return(true, nil)
				store.On("SaveDashboard", mock.Anything).Return(dashboardFolder, nil)
				store.On("GetFolderByID", mock.Anything, orgID, dashboardFolder.Id).Return(f, nil)

				req := &models.UpdateFolderCommand{
					Uid:   dashboardFolder.Uid,
					Title: "TEST-Folder",
				}

				err := service.UpdateFolder(context.Background(), user, orgID, dashboardFolder.Uid, req)
				require.NoError(t, err)
				require.Equal(t, f, req.Result)
			})

			t.Run("When deleting folder by uid should not return access denied error", func(t *testing.T) {
				f := models.NewFolder(util.GenerateShortUID())
				f.Id = rand.Int63()
				f.Uid = util.GenerateShortUID()
				store.On("GetFolderByUID", mock.Anything, orgID, f.Uid).Return(f, nil)

				var actualCmd *models.DeleteDashboardCommand
				store.On("DeleteDashboard", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					actualCmd = args.Get(1).(*models.DeleteDashboardCommand)
				}).Return(nil).Once()

				expectedForceDeleteRules := rand.Int63()%2 == 0
				_, err := service.DeleteFolder(context.Background(), user, orgID, f.Uid, expectedForceDeleteRules)
				require.NoError(t, err)
				require.NotNil(t, actualCmd)
				require.Equal(t, f.Id, actualCmd.Id)
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
				expected := models.NewFolder(util.GenerateShortUID())
				expected.Id = rand.Int63()

				store.On("GetFolderByID", mock.Anything, orgID, expected.Id).Return(expected, nil)

				actual, err := service.GetFolderByID(context.Background(), user, expected.Id, orgID)
				require.Equal(t, expected, actual)
				require.NoError(t, err)
			})

			t.Run("When get folder by uid should return folder", func(t *testing.T) {
				expected := models.NewFolder(util.GenerateShortUID())
				expected.Uid = util.GenerateShortUID()

				store.On("GetFolderByUID", mock.Anything, orgID, expected.Uid).Return(expected, nil)

				actual, err := service.GetFolderByUID(context.Background(), user, orgID, expected.Uid)
				require.Equal(t, expected, actual)
				require.NoError(t, err)
			})

			t.Run("When get folder by title should return folder", func(t *testing.T) {
				expected := models.NewFolder("TEST-" + util.GenerateShortUID())

				store.On("GetFolderByTitle", mock.Anything, orgID, expected.Title).Return(expected, nil)

				actual, err := service.GetFolderByTitle(context.Background(), user, orgID, expected.Title)
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
				{ActualError: models.ErrDashboardTitleEmpty, ExpectedError: models.ErrFolderTitleEmpty},
				{ActualError: models.ErrDashboardUpdateAccessDenied, ExpectedError: models.ErrFolderAccessDenied},
				{ActualError: models.ErrDashboardWithSameNameInFolderExists, ExpectedError: models.ErrFolderSameNameExists},
				{ActualError: models.ErrDashboardWithSameUIDExists, ExpectedError: models.ErrFolderWithSameUIDExists},
				{ActualError: models.ErrDashboardVersionMismatch, ExpectedError: models.ErrFolderVersionMismatch},
				{ActualError: models.ErrDashboardNotFound, ExpectedError: models.ErrFolderNotFound},
				{ActualError: models.ErrDashboardFailedGenerateUniqueUid, ExpectedError: models.ErrFolderFailedGenerateUniqueUid},
				{ActualError: models.ErrDashboardInvalidUid, ExpectedError: models.ErrDashboardInvalidUid},
			}

			for _, tc := range testCases {
				actualError := toFolderError(tc.ActualError)
				assert.EqualErrorf(t, actualError, tc.ExpectedError.Error(),
					"For error '%s' expected error '%s', actual '%s'", tc.ActualError, tc.ExpectedError, actualError)
			}
		})
	})
}
