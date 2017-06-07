package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
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

	if len(reqDto.Queries) == 0 {
		return ApiError(400, "No queries found in query", nil)
	}

	dsId, err := reqDto.Queries[0].Get("datasourceId").Int64()
	if err != nil {
		return ApiError(400, "Query missing datasourceId", nil)
	}

	dsQuery := models.GetDataSourceByIdQuery{Id: dsId}
	if err := bus.Dispatch(&dsQuery); err != nil {
		return ApiError(500, "failed to fetch data source", err)
	}

	request := &tsdb.Request{TimeRange: timeRange}

	for _, query := range reqDto.Queries {
		request.Queries = append(request.Queries, &tsdb.Query{
			RefId:         query.Get("refId").MustString("A"),
			MaxDataPoints: query.Get("maxDataPoints").MustInt64(100),
			IntervalMs:    query.Get("intervalMs").MustInt64(1000),
			Model:         query,
			DataSource:    dsQuery.Result,
		})
	}

	resp, err := tsdb.HandleRequest(context.Background(), request)
	if err != nil {
		return ApiError(500, "Metric request error", err)
	}

	statusCode := 200
	for _, res := range resp.Results {
		if res.Error != nil {
			res.ErrorString = res.Error.Error()
			resp.Message = res.ErrorString
			statusCode = 500
		}
	}

	return Json(statusCode, &resp)
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

// GET /api/tsdb/testdata/gensql
func GenerateSqlTestData(c *middleware.Context) Response {
	if err := bus.Dispatch(&models.InsertSqlTestDataCommand{}); err != nil {
		return ApiError(500, "Failed to insert test data", err)
	}

	return Json(200, &util.DynMap{"message": "OK"})
}

// GET /api/tsdb/testdata/random-walk
func GetTestDataRandomWalk(c *middleware.Context) Response {
	from := c.Query("from")
	to := c.Query("to")
	intervalMs := c.QueryInt64("intervalMs")

	timeRange := tsdb.NewTimeRange(from, to)
	request := &tsdb.Request{TimeRange: timeRange}

	request.Queries = append(request.Queries, &tsdb.Query{
		RefId:      "A",
		IntervalMs: intervalMs,
		Model: simplejson.NewFromAny(&util.DynMap{
			"scenario": "random_walk",
		}),
		DataSource: &models.DataSource{Type: "grafana-testdata-datasource"},
	})

	resp, err := tsdb.HandleRequest(context.Background(), request)
	if err != nil {
		return ApiError(500, "Metric request error", err)
	}

	return Json(200, &resp)
}
