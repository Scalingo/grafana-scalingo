package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
	"github.com/stretchr/testify/require"
)

func TestApi_getUsageStats(t *testing.T) {
	type getUsageStatsTestCase struct {
		desc           string
		expectedStatus int
		IsGrafanaAdmin bool
		enabled        bool
	}
	tests := []getUsageStatsTestCase{
		{
			desc:           "expect usage stats",
			enabled:        true,
			IsGrafanaAdmin: true,
			expectedStatus: 200,
		},
		{
			desc:           "expect usage stat preview still there after disabling",
			enabled:        false,
			IsGrafanaAdmin: true,
			expectedStatus: 200,
		},
		{
			desc:           "expect http status 403 when not admin",
			enabled:        false,
			IsGrafanaAdmin: false,
			expectedStatus: 403,
		},
	}
	sqlStore := mockstore.NewSQLStoreMock()
	uss := createService(t, setting.Cfg{}, sqlStore, false)
	uss.registerAPIEndpoints()

	sqlStore.ExpectedSystemStats = &models.SystemStats{}
	sqlStore.ExpectedDataSourceStats = []*models.DataSourceStats{}
	sqlStore.ExpectedDataSources = []*models.DataSource{}
	sqlStore.ExpectedDataSourcesAccessStats = []*models.DataSourceAccessStats{}
	sqlStore.ExpectedNotifierUsageStats = []*models.NotifierUsageStats{}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			uss.Cfg.ReportingEnabled = tt.enabled
			server := setupTestServer(t, &models.SignedInUser{OrgId: 1, IsGrafanaAdmin: tt.IsGrafanaAdmin}, uss)

			usageStats, recorder := getUsageStats(t, server)
			require.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedStatus == http.StatusOK {
				require.NotNil(t, usageStats)
			}
		})
	}
}

func getUsageStats(t *testing.T, server *web.Mux) (*models.SystemStats, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(http.MethodGet, "/api/admin/usage-report-preview", http.NoBody)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	var usageStats models.SystemStats
	if recorder.Code == http.StatusOK {
		require.NoError(t, json.NewDecoder(recorder.Body).Decode(&usageStats))
	}
	return &usageStats, recorder
}

func setupTestServer(t *testing.T, user *models.SignedInUser, service *UsageStats) *web.Mux {
	server := web.New()
	server.UseMiddleware(web.Renderer(path.Join(setting.StaticRootPath, "views"), "[[", "]]"))
	server.Use(contextProvider(&testContext{user}))
	service.RouteRegister.Register(server)
	return server
}

type testContext struct {
	user *models.SignedInUser
}

func contextProvider(tc *testContext) web.Handler {
	return func(c *web.Context) {
		signedIn := tc.user != nil
		reqCtx := &models.ReqContext{
			Context:      c,
			SignedInUser: tc.user,
			IsSignedIn:   signedIn,
			SkipCache:    true,
			Logger:       log.New("test"),
		}
		c.Map(reqCtx)
	}
}
