package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/plugindashboards"
	"github.com/grafana/grafana/pkg/web/webtest"
	"github.com/stretchr/testify/require"
)

func TestGetPluginDashboards(t *testing.T) {
	const existingPluginID = "existing-plugin"
	pluginDashboardService := &pluginDashboardServiceMock{
		pluginDashboards: map[string][]*plugindashboards.PluginDashboard{
			existingPluginID: {
				{
					PluginId: existingPluginID,
					UID:      "a",
					Title:    "A",
				},
				{
					PluginId: existingPluginID,
					UID:      "b",
					Title:    "B",
				},
			},
		},
		unexpectedErrors: map[string]error{
			"boom": fmt.Errorf("BOOM"),
		},
	}

	s := SetupAPITestServer(t, func(hs *HTTPServer) {
		hs.pluginDashboardService = pluginDashboardService
	})

	t.Run("Not signed in should return 404 Not Found", func(t *testing.T) {
		req := s.NewGetRequest("/api/plugins/test/dashboards")
		resp, err := s.Send(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Signed in and not org admin should return 403 Forbidden", func(t *testing.T) {
		user := &models.SignedInUser{
			UserId:  1,
			OrgRole: models.ROLE_EDITOR,
		}

		resp, err := sendGetPluginDashboardsRequestForSignedInUser(t, s, existingPluginID, user)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("Signed in and org admin", func(t *testing.T) {
		user := &models.SignedInUser{
			UserId:  1,
			OrgId:   1,
			OrgRole: models.ROLE_ADMIN,
		}

		t.Run("When plugin doesn't exist should return 404 Not Found", func(t *testing.T) {
			resp, err := sendGetPluginDashboardsRequestForSignedInUser(t, s, "not-exists", user)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("When result is unexpected error should return 500 Internal Server Error", func(t *testing.T) {
			resp, err := sendGetPluginDashboardsRequestForSignedInUser(t, s, "boom", user)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		})

		t.Run("When plugin exists should return 200 OK with expected payload", func(t *testing.T) {
			resp, err := sendGetPluginDashboardsRequestForSignedInUser(t, s, existingPluginID, user)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			bytes, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			var listResp []*plugindashboards.PluginDashboard
			err = json.Unmarshal(bytes, &listResp)
			require.NoError(t, err)
			require.NotNil(t, listResp)
			require.Len(t, listResp, 2)
			require.Equal(t, pluginDashboardService.pluginDashboards[existingPluginID], listResp)
		})
	})
}

func sendGetPluginDashboardsRequestForSignedInUser(t *testing.T, s *webtest.Server, pluginID string, user *models.SignedInUser) (*http.Response, error) {
	t.Helper()

	req := s.NewGetRequest(fmt.Sprintf("/api/plugins/%s/dashboards", pluginID))
	webtest.RequestWithSignedInUser(req, user)
	return s.Send(req)
}

type pluginDashboardServiceMock struct {
	plugindashboards.Service
	pluginDashboards map[string][]*plugindashboards.PluginDashboard
	unexpectedErrors map[string]error
}

func (m *pluginDashboardServiceMock) ListPluginDashboards(ctx context.Context, req *plugindashboards.ListPluginDashboardsRequest) (*plugindashboards.ListPluginDashboardsResponse, error) {
	if pluginDashboards, exists := m.pluginDashboards[req.PluginID]; exists {
		return &plugindashboards.ListPluginDashboardsResponse{
			Items: pluginDashboards,
		}, nil
	}

	if err, exists := m.unexpectedErrors[req.PluginID]; exists {
		return nil, err
	}

	return nil, plugins.NotFoundError{PluginID: req.PluginID}
}
