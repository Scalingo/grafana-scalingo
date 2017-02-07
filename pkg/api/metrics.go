package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/metrics"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
	"github.com/grafana/grafana/pkg/tsdb/testdata"
	"github.com/grafana/grafana/pkg/util"
)

// POST /api/tsdb/query
func QueryMetrics(c *middleware.Context, reqDto dtos.MetricRequest) Response {
	timeRange := tsdb.NewTimeRange(reqDto.From, reqDto.To)

	request := &tsdb.Request{TimeRange: timeRange}

	for _, query := range reqDto.Queries {
		request.Queries = append(request.Queries, &tsdb.Query{
			RefId:         query.Get("refId").MustString("A"),
			MaxDataPoints: query.Get("maxDataPoints").MustInt64(100),
			IntervalMs:    query.Get("intervalMs").MustInt64(1000),
			Model:         query,
			DataSource: &models.DataSource{
				Name: "Grafana TestDataDB",
				Type: "grafana-testdata-datasource",
			},
		})
	}

	resp, err := tsdb.HandleRequest(context.TODO(), request)
	if err != nil {
		return ApiError(500, "Metric request error", err)
	}

	return Json(200, &resp)
}

// GET /api/tsdb/testdata/scenarios
func GetTestDataScenarios(c *middleware.Context) Response {
	result := make([]interface{}, 0)

	for _, scenario := range testdata.ScenarioRegistry {
		result = append(result, map[string]interface{}{
			"id":          scenario.Id,
			"name":        scenario.Name,
			"description": scenario.Description,
			"stringInput": scenario.StringInput,
		})
	}

	return Json(200, &result)
}

func GetInternalMetrics(c *middleware.Context) Response {
	if metrics.UseNilMetrics {
		return Json(200, util.DynMap{"message": "Metrics disabled"})
	}

	snapshots := metrics.MetricStats.GetSnapshots()

	resp := make(map[string]interface{})

	for _, m := range snapshots {
		metricName := m.Name() + m.StringifyTags()

		switch metric := m.(type) {
		case metrics.Gauge:
			resp[metricName] = map[string]interface{}{
				"value": metric.Value(),
			}
		case metrics.Counter:
			resp[metricName] = map[string]interface{}{
				"count": metric.Count(),
			}
		case metrics.Timer:
			percentiles := metric.Percentiles([]float64{0.25, 0.75, 0.90, 0.99})
			resp[metricName] = map[string]interface{}{
				"count": metric.Count(),
				"min":   metric.Min(),
				"max":   metric.Max(),
				"mean":  metric.Mean(),
				"std":   metric.StdDev(),
				"p25":   percentiles[0],
				"p75":   percentiles[1],
				"p90":   percentiles[2],
				"p99":   percentiles[3],
			}
		}
	}

	var b []byte
	var err error
	if b, err = json.MarshalIndent(resp, "", " "); err != nil {
		return ApiError(500, "body json marshal", err)
	}

	return &NormalResponse{
		body:   b,
		status: 200,
		header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}

// Genereates a index out of range error
func GenerateError(c *middleware.Context) Response {
	var array []string
	return Json(200, array[20])
}
