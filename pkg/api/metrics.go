package api

import (
	"context"
	"sort"

	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/setting"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
	"github.com/grafana/grafana/pkg/tsdb/testdatasource"
	"github.com/grafana/grafana/pkg/util"
)

// POST /api/ds/query   DataSource query w/ expressions
func (hs *HTTPServer) QueryMetricsV2(c *m.ReqContext, reqDto dtos.MetricRequest) Response {
	if !setting.IsExpressionsEnabled() {
		return Error(404, "Expressions feature toggle is not enabled", nil)
	}

	if len(reqDto.Queries) == 0 {
		return Error(500, "No queries found in query", nil)
	}

	request := &tsdb.TsdbQuery{
		TimeRange: tsdb.NewTimeRange(reqDto.From, reqDto.To),
		Debug:     reqDto.Debug,
	}

	expr := false
	var ds *m.DataSource
	for i, query := range reqDto.Queries {
		name, err := query.Get("datasource").String()
		if err != nil {
			return Error(500, "datasource missing name", err)
		}
		if name == "__expr__" {
			expr = true
		}

		datasourceID, err := query.Get("datasourceId").Int64()
		if err != nil {
			return Error(500, "datasource missing ID", nil)
		}

		if i == 0 && !expr {
			ds, err = hs.DatasourceCache.GetDatasource(datasourceID, c.SignedInUser, c.SkipCache)
			if err != nil {
				if err == m.ErrDataSourceAccessDenied {
					return Error(403, "Access denied to datasource", err)
				}
				return Error(500, "Unable to load datasource meta data", err)
			}
		}
		request.Queries = append(request.Queries, &tsdb.Query{
			RefId:         query.Get("refId").MustString("A"),
			MaxDataPoints: query.Get("maxDataPoints").MustInt64(100),
			IntervalMs:    query.Get("intervalMs").MustInt64(1000),
			Model:         query,
			DataSource:    ds,
		})

	}

	var resp *tsdb.Response
	var err error
	if !expr {
		resp, err = tsdb.HandleRequest(c.Req.Context(), ds, request)
		if err != nil {
			return Error(500, "Metric request error", err)
		}
	} else {
		resp, err = plugins.Transform.Transform(c.Req.Context(), request)
		if err != nil {
			return Error(500, "Transform request error", err)
		}
	}

	statusCode := 200
	for _, res := range resp.Results {
		if res.Error != nil {
			res.ErrorString = res.Error.Error()
			resp.Message = res.ErrorString
			statusCode = 400
		}
	}

	return JSON(statusCode, &resp)
}

// POST /api/tsdb/query
func (hs *HTTPServer) QueryMetrics(c *m.ReqContext, reqDto dtos.MetricRequest) Response {
	timeRange := tsdb.NewTimeRange(reqDto.From, reqDto.To)

	if len(reqDto.Queries) == 0 {
		return Error(400, "No queries found in query", nil)
	}

	datasourceId, err := reqDto.Queries[0].Get("datasourceId").Int64()
	if err != nil {
		return Error(400, "Query missing datasourceId", nil)
	}

	ds, err := hs.DatasourceCache.GetDatasource(datasourceId, c.SignedInUser, c.SkipCache)
	if err != nil {
		if err == m.ErrDataSourceAccessDenied {
			return Error(403, "Access denied to datasource", err)
		}
		return Error(500, "Unable to load datasource meta data", err)
	}

	request := &tsdb.TsdbQuery{TimeRange: timeRange, Debug: reqDto.Debug}

	for _, query := range reqDto.Queries {
		request.Queries = append(request.Queries, &tsdb.Query{
			RefId:         query.Get("refId").MustString("A"),
			MaxDataPoints: query.Get("maxDataPoints").MustInt64(100),
			IntervalMs:    query.Get("intervalMs").MustInt64(1000),
			Model:         query,
			DataSource:    ds,
		})
	}

	resp, err := tsdb.HandleRequest(c.Req.Context(), ds, request)
	if err != nil {
		return Error(500, "Metric request error", err)
	}

	statusCode := 200
	for _, res := range resp.Results {
		if res.Error != nil {
			res.ErrorString = res.Error.Error()
			resp.Message = res.ErrorString
			statusCode = 400
		}
	}

	return JSON(statusCode, &resp)
}

// GET /api/tsdb/testdata/scenarios
func GetTestDataScenarios(c *m.ReqContext) Response {
	result := make([]interface{}, 0)

	scenarioIds := make([]string, 0)
	for id := range testdatasource.ScenarioRegistry {
		scenarioIds = append(scenarioIds, id)
	}
	sort.Strings(scenarioIds)

	for _, scenarioId := range scenarioIds {
		scenario := testdatasource.ScenarioRegistry[scenarioId]
		result = append(result, map[string]interface{}{
			"id":          scenario.Id,
			"name":        scenario.Name,
			"description": scenario.Description,
			"stringInput": scenario.StringInput,
		})
	}

	return JSON(200, &result)
}

// Generates a index out of range error
func GenerateError(c *m.ReqContext) Response {
	var array []string
	// nolint: govet
	return JSON(200, array[20])
}

// GET /api/tsdb/testdata/gensql
func GenerateSQLTestData(c *m.ReqContext) Response {
	if err := bus.Dispatch(&m.InsertSqlTestDataCommand{}); err != nil {
		return Error(500, "Failed to insert test data", err)
	}

	return JSON(200, &util.DynMap{"message": "OK"})
}

// GET /api/tsdb/testdata/random-walk
func GetTestDataRandomWalk(c *m.ReqContext) Response {
	from := c.Query("from")
	to := c.Query("to")
	intervalMs := c.QueryInt64("intervalMs")

	timeRange := tsdb.NewTimeRange(from, to)
	request := &tsdb.TsdbQuery{TimeRange: timeRange}

	dsInfo := &m.DataSource{Type: "testdata"}
	request.Queries = append(request.Queries, &tsdb.Query{
		RefId:      "A",
		IntervalMs: intervalMs,
		Model: simplejson.NewFromAny(&util.DynMap{
			"scenario": "random_walk",
		}),
		DataSource: dsInfo,
	})

	resp, err := tsdb.HandleRequest(context.Background(), dsInfo, request)
	if err != nil {
		return Error(500, "Metric request error", err)
	}

	return JSON(200, &resp)
}
