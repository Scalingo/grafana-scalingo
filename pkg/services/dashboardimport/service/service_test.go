package service

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/dashboardimport"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/librarypanels"
	"github.com/grafana/grafana/pkg/services/plugindashboards"
	"github.com/stretchr/testify/require"
)

func TestImportDashboardService(t *testing.T) {
	t.Run("When importing a plugin dashboard should save dashboard and sync library panels", func(t *testing.T) {
		pluginDashboardService := &pluginDashboardServiceMock{
			loadPluginDashboardFunc: loadTestDashboard,
		}

		var importDashboardArg *dashboards.SaveDashboardDTO
		dashboardService := &dashboardServiceMock{
			importDashboardFunc: func(ctx context.Context, dto *dashboards.SaveDashboardDTO) (*models.Dashboard, error) {
				importDashboardArg = dto
				return &models.Dashboard{
					Id:       4,
					Uid:      dto.Dashboard.Uid,
					Slug:     dto.Dashboard.Slug,
					OrgId:    3,
					Version:  dto.Dashboard.Version,
					PluginId: "prometheus",
					FolderId: dto.Dashboard.FolderId,
					Title:    dto.Dashboard.Title,
					Data:     dto.Dashboard.Data,
				}, nil
			},
		}

		importLibraryPanelsForDashboard := false
		connectLibraryPanelsForDashboardCalled := false
		libraryPanelService := &libraryPanelServiceMock{
			importLibraryPanelsForDashboardFunc: func(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard, folderID int64) error {
				importLibraryPanelsForDashboard = true
				return nil
			},
			connectLibraryPanelsForDashboardFunc: func(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard) error {
				connectLibraryPanelsForDashboardCalled = true
				return nil
			},
		}
		s := &ImportDashboardService{
			pluginDashboardService: pluginDashboardService,
			dashboardService:       dashboardService,
			libraryPanelService:    libraryPanelService,
		}

		req := &dashboardimport.ImportDashboardRequest{
			PluginId: "prometheus",
			Path:     "dashboard.json",
			Inputs: []dashboardimport.ImportDashboardInput{
				{Name: "*", Type: "datasource", Value: "prom"},
			},
			User:     &models.SignedInUser{UserId: 2, OrgRole: models.ROLE_ADMIN, OrgId: 3},
			FolderId: 5,
		}
		resp, err := s.ImportDashboard(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "UDdpyzz7z", resp.UID)

		require.NotNil(t, importDashboardArg)
		require.Equal(t, int64(3), importDashboardArg.OrgId)
		require.Equal(t, int64(2), importDashboardArg.User.UserId)
		require.Equal(t, "prometheus", importDashboardArg.Dashboard.PluginId)
		require.Equal(t, int64(5), importDashboardArg.Dashboard.FolderId)

		panel := importDashboardArg.Dashboard.Data.Get("panels").GetIndex(0)
		require.Equal(t, "prom", panel.Get("datasource").MustString())

		require.True(t, importLibraryPanelsForDashboard)
		require.True(t, connectLibraryPanelsForDashboardCalled)
	})

	t.Run("When importing a non-plugin dashboard should save dashboard and sync library panels", func(t *testing.T) {
		var importDashboardArg *dashboards.SaveDashboardDTO
		dashboardService := &dashboardServiceMock{
			importDashboardFunc: func(ctx context.Context, dto *dashboards.SaveDashboardDTO) (*models.Dashboard, error) {
				importDashboardArg = dto
				return &models.Dashboard{
					Id:       4,
					Uid:      dto.Dashboard.Uid,
					Slug:     dto.Dashboard.Slug,
					OrgId:    3,
					Version:  dto.Dashboard.Version,
					PluginId: "prometheus",
					FolderId: dto.Dashboard.FolderId,
					Title:    dto.Dashboard.Title,
					Data:     dto.Dashboard.Data,
				}, nil
			},
		}
		libraryPanelService := &libraryPanelServiceMock{}
		s := &ImportDashboardService{
			dashboardService:    dashboardService,
			libraryPanelService: libraryPanelService,
		}

		loadResp, err := loadTestDashboard(context.Background(), &plugindashboards.LoadPluginDashboardRequest{
			PluginID:  "",
			Reference: "dashboard.json",
		})
		require.NoError(t, err)

		req := &dashboardimport.ImportDashboardRequest{
			Dashboard: loadResp.Dashboard.Data,
			Path:      "plugin_dashboard.json",
			Inputs: []dashboardimport.ImportDashboardInput{
				{Name: "*", Type: "datasource", Value: "prom"},
			},
			User:     &models.SignedInUser{UserId: 2, OrgRole: models.ROLE_ADMIN, OrgId: 3},
			FolderId: 5,
		}
		resp, err := s.ImportDashboard(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "UDdpyzz7z", resp.UID)

		require.NotNil(t, importDashboardArg)
		require.Equal(t, int64(3), importDashboardArg.OrgId)
		require.Equal(t, int64(2), importDashboardArg.User.UserId)
		require.Equal(t, "", importDashboardArg.Dashboard.PluginId)
		require.Equal(t, int64(5), importDashboardArg.Dashboard.FolderId)

		panel := importDashboardArg.Dashboard.Data.Get("panels").GetIndex(0)
		require.Equal(t, "prom", panel.Get("datasource").MustString())
	})
}

func loadTestDashboard(ctx context.Context, req *plugindashboards.LoadPluginDashboardRequest) (*plugindashboards.LoadPluginDashboardResponse, error) {
	// It's safe to ignore gosec warning G304 since this is a test and arguments comes from test configuration.
	// nolint:gosec
	bytes, err := ioutil.ReadFile(filepath.Join("testdata", req.Reference))
	if err != nil {
		return nil, err
	}

	dashboardJSON, err := simplejson.NewJson(bytes)
	if err != nil {
		return nil, err
	}

	return &plugindashboards.LoadPluginDashboardResponse{
		Dashboard: models.NewDashboardFromJson(dashboardJSON),
	}, nil
}

type pluginDashboardServiceMock struct {
	plugindashboards.Service
	loadPluginDashboardFunc func(ctx context.Context, req *plugindashboards.LoadPluginDashboardRequest) (*plugindashboards.LoadPluginDashboardResponse, error)
}

func (m *pluginDashboardServiceMock) LoadPluginDashboard(ctx context.Context, req *plugindashboards.LoadPluginDashboardRequest) (*plugindashboards.LoadPluginDashboardResponse, error) {
	if m.loadPluginDashboardFunc != nil {
		return m.loadPluginDashboardFunc(ctx, req)
	}

	return nil, nil
}

type dashboardServiceMock struct {
	dashboards.DashboardService
	importDashboardFunc func(ctx context.Context, dto *dashboards.SaveDashboardDTO) (*models.Dashboard, error)
}

func (s *dashboardServiceMock) ImportDashboard(ctx context.Context, dto *dashboards.SaveDashboardDTO) (*models.Dashboard, error) {
	if s.importDashboardFunc != nil {
		return s.importDashboardFunc(ctx, dto)
	}

	return nil, nil
}

type libraryPanelServiceMock struct {
	librarypanels.Service
	connectLibraryPanelsForDashboardFunc func(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard) error
	importLibraryPanelsForDashboardFunc  func(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard, folderID int64) error
}

func (s *libraryPanelServiceMock) ConnectLibraryPanelsForDashboard(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard) error {
	if s.connectLibraryPanelsForDashboardFunc != nil {
		return s.connectLibraryPanelsForDashboardFunc(ctx, signedInUser, dash)
	}

	return nil
}

func (s *libraryPanelServiceMock) ImportLibraryPanelsForDashboard(ctx context.Context, signedInUser *models.SignedInUser, dash *models.Dashboard, folderID int64) error {
	if s.importLibraryPanelsForDashboardFunc != nil {
		return s.importLibraryPanelsForDashboardFunc(ctx, signedInUser, dash, folderID)
	}

	return nil
}
