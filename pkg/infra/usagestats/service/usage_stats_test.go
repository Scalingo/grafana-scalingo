package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/infra/kvstore"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/setting"
)

// This is to ensure that the interface contract is held by the implementation
func Test_InterfaceContractValidity(t *testing.T) {
	newUsageStats := func() usagestats.Service {
		return &UsageStats{}
	}
	v, ok := newUsageStats().(*UsageStats)

	assert.NotNil(t, v)
	assert.True(t, ok)
}

func TestMetrics(t *testing.T) {
	const metricName = "stats.test_metric.count"

	sqlStore := mockstore.NewSQLStoreMock()
	uss := createService(t, setting.Cfg{}, sqlStore, false)

	uss.RegisterMetricsFunc(func(context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{metricName: 1}, nil
	})

	err := uss.sendUsageStats(context.Background())
	require.NoError(t, err)

	t.Run("Given reporting not enabled and sending usage stats", func(t *testing.T) {
		origSendUsageStats := sendUsageStats
		t.Cleanup(func() {
			sendUsageStats = origSendUsageStats
		})
		statsSent := false
		sendUsageStats = func(uss *UsageStats, b *bytes.Buffer) {
			statsSent = true
		}

		uss.Cfg.ReportingEnabled = false
		err := uss.sendUsageStats(context.Background())
		require.NoError(t, err)

		require.False(t, statsSent)
	})

	t.Run("Given reporting enabled, stats should be gathered and sent to HTTP endpoint", func(t *testing.T) {
		origCfg := uss.Cfg
		t.Cleanup(func() {
			uss.Cfg = origCfg
		})
		uss.Cfg = &setting.Cfg{
			ReportingEnabled:     true,
			BuildVersion:         "5.0.0",
			AnonymousEnabled:     true,
			BasicAuthEnabled:     true,
			LDAPEnabled:          true,
			AuthProxyEnabled:     true,
			Packaging:            "deb",
			ReportingDistributor: "hosted-grafana",
		}

		ch := make(chan httpResp)
		ticker := time.NewTicker(2 * time.Second)
		ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			buf, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Logf("Fake HTTP handler received an error: %s", err.Error())
				ch <- httpResp{
					err: err,
				}
				return
			}
			require.NoError(t, err, "Failed to read response body, err=%v", err)
			t.Logf("Fake HTTP handler received a response")
			ch <- httpResp{
				responseBuffer: bytes.NewBuffer(buf),
				req:            r,
			}
		}))
		t.Cleanup(ts.Close)
		t.Cleanup(func() {
			close(ch)
		})
		usageStatsURL = ts.URL

		err := uss.sendUsageStats(context.Background())
		require.NoError(t, err)

		// Wait for fake HTTP server to receive a request
		var resp httpResp
		select {
		case resp = <-ch:
			require.NoError(t, resp.err, "Fake server experienced an error")
		case <-ticker.C:
			t.Fatalf("Timed out waiting for HTTP request")
		}

		t.Logf("Received response from fake HTTP server: %+v\n", resp)

		assert.NotNil(t, resp.req)

		assert.Equal(t, http.MethodPost, resp.req.Method)
		assert.Equal(t, "application/json", resp.req.Header.Get("Content-Type"))

		require.NotNil(t, resp.responseBuffer)

		j := make(map[string]interface{})
		err = json.Unmarshal(resp.responseBuffer.Bytes(), &j)
		require.NoError(t, err)

		assert.Equal(t, "5_0_0", j["version"])
		assert.Equal(t, runtime.GOOS, j["os"])
		assert.Equal(t, runtime.GOARCH, j["arch"])

		usageId := uss.GetUsageStatsId(context.Background())
		assert.NotEmpty(t, usageId)

		metrics, ok := j["metrics"].(map[string]interface{})
		require.True(t, ok)
		assert.EqualValues(t, 1, metrics[metricName])
	})
}

func TestGetUsageReport_IncludesMetrics(t *testing.T) {
	sqlStore := mockstore.NewSQLStoreMock()
	uss := createService(t, setting.Cfg{}, sqlStore, true)
	metricName := "stats.test_metric.count"

	uss.RegisterMetricsFunc(func(context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{metricName: 1}, nil
	})

	report, err := uss.GetUsageReport(context.Background())
	require.NoError(t, err, "Expected no error")

	metric := report.Metrics[metricName]
	assert.Equal(t, 1, metric)
}

func TestRegisterMetrics(t *testing.T) {
	const goodMetricName = "stats.test_external_metric.count"

	sqlStore := mockstore.NewSQLStoreMock()
	uss := createService(t, setting.Cfg{}, sqlStore, false)
	metrics := map[string]interface{}{"stats.test_metric.count": 1, "stats.test_metric_second.count": 2}

	uss.RegisterMetricsFunc(func(context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{goodMetricName: 1}, nil
	})

	{
		extMetrics, err := uss.externalMetrics[0](context.Background())
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{goodMetricName: 1}, extMetrics)
	}

	uss.gatherMetrics(context.Background(), metrics)
	assert.Equal(t, 1, metrics[goodMetricName])

	t.Run("do not add metrics that return an error when fetched", func(t *testing.T) {
		const badMetricName = "stats.test_external_metric_error.count"

		uss.RegisterMetricsFunc(func(context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{badMetricName: 1}, errors.New("some error")
		})
		uss.gatherMetrics(context.Background(), metrics)

		extErrorMetric := metrics[badMetricName]
		extMetric := metrics[goodMetricName]

		require.Nil(t, extErrorMetric, "Invalid metric should not be added")
		assert.Equal(t, 1, extMetric)
		assert.Len(t, metrics, 3, "Expected only one available metric")
	})
}

type fakePluginStore struct {
	plugins.Store

	plugins map[string]plugins.PluginDTO
}

func (pr fakePluginStore) Plugin(_ context.Context, pluginID string) (plugins.PluginDTO, bool) {
	p, exists := pr.plugins[pluginID]

	return p, exists
}

func (pr fakePluginStore) Plugins(_ context.Context, pluginTypes ...plugins.Type) []plugins.PluginDTO {
	var result []plugins.PluginDTO
	for _, v := range pr.plugins {
		for _, t := range pluginTypes {
			if v.Type == t {
				result = append(result, v)
			}
		}
	}

	return result
}

type httpResp struct {
	req            *http.Request
	responseBuffer *bytes.Buffer
	err            error
}

func createService(t *testing.T, cfg setting.Cfg, sqlStore sqlstore.Store, withDB bool) *UsageStats {
	t.Helper()
	if withDB {
		sqlStore = sqlstore.InitTestDB(t)
	}

	return ProvideService(
		&cfg,
		&fakePluginStore{},
		kvstore.ProvideService(sqlStore),
		routing.NewRouteRegister(),
	)
}
