package conditions

import (
	"fmt"
	"strings"
	"time"

	gocontext "context"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/null"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/tsdb"
)

func init() {
	alerting.RegisterCondition("query", func(model *simplejson.Json, index int) (alerting.Condition, error) {
		return newQueryCondition(model, index)
	})
}

// QueryCondition is responsible for issue and query, reduce the
// timeseries into single values and evaluate if they are firing or not.
type QueryCondition struct {
	Index         int
	Query         AlertQuery
	Reducer       *queryReducer
	Evaluator     AlertEvaluator
	Operator      string
	HandleRequest tsdb.HandleRequestFunc
}

// AlertQuery contains information about what datasource a query
// should be sent to and the query object.
type AlertQuery struct {
	Model        *simplejson.Json
	DatasourceID int64
	From         string
	To           string
}

// Eval evaluates the `QueryCondition`.
func (c *QueryCondition) Eval(context *alerting.EvalContext) (*alerting.ConditionResult, error) {
	timeRange := tsdb.NewTimeRange(c.Query.From, c.Query.To)

	seriesList, err := c.executeQuery(context, timeRange)
	if err != nil {
		return nil, err
	}

	emptySerieCount := 0
	evalMatchCount := 0
	var matches []*alerting.EvalMatch

	for _, series := range seriesList {
		reducedValue := c.Reducer.Reduce(series)
		evalMatch := c.Evaluator.Eval(reducedValue)

		if !reducedValue.Valid {
			emptySerieCount++
		}

		if context.IsTestRun {
			context.Logs = append(context.Logs, &alerting.ResultLogEntry{
				Message: fmt.Sprintf("Condition[%d]: Eval: %v, Metric: %s, Value: %s", c.Index, evalMatch, series.Name, reducedValue),
			})
		}

		if evalMatch {
			evalMatchCount++

			matches = append(matches, &alerting.EvalMatch{
				Metric: series.Name,
				Value:  reducedValue,
				Tags:   series.Tags,
			})
		}
	}

	// handle no series special case
	if len(seriesList) == 0 {
		// eval condition for null value
		evalMatch := c.Evaluator.Eval(null.FloatFromPtr(nil))

		if context.IsTestRun {
			context.Logs = append(context.Logs, &alerting.ResultLogEntry{
				Message: fmt.Sprintf("Condition: Eval: %v, Query Returned No Series (reduced to null/no value)", evalMatch),
			})
		}

		if evalMatch {
			evalMatchCount++
			matches = append(matches, &alerting.EvalMatch{Metric: "NoData", Value: null.FloatFromPtr(nil)})
		}
	}

	return &alerting.ConditionResult{
		Firing:      evalMatchCount > 0,
		NoDataFound: emptySerieCount == len(seriesList),
		Operator:    c.Operator,
		EvalMatches: matches,
	}, nil
}

func (c *QueryCondition) executeQuery(context *alerting.EvalContext, timeRange *tsdb.TimeRange) (tsdb.TimeSeriesSlice, error) {
	getDsInfo := &models.GetDataSourceByIdQuery{
		Id:    c.Query.DatasourceID,
		OrgId: context.Rule.OrgID,
	}

	if err := bus.Dispatch(getDsInfo); err != nil {
		return nil, fmt.Errorf("Could not find datasource %v", err)
	}

	req := c.getRequestForAlertRule(getDsInfo.Result, timeRange, context.IsDebug)
	result := make(tsdb.TimeSeriesSlice, 0)

	if context.IsDebug {
		data := simplejson.New()
		if req.TimeRange != nil {
			data.Set("from", req.TimeRange.GetFromAsMsEpoch())
			data.Set("to", req.TimeRange.GetToAsMsEpoch())
		}

		type queryDto struct {
			RefId         string           `json:"refId"`
			Model         *simplejson.Json `json:"model"`
			Datasource    *simplejson.Json `json:"datasource"`
			MaxDataPoints int64            `json:"maxDataPoints"`
			IntervalMs    int64            `json:"intervalMs"`
		}

		queries := []*queryDto{}
		for _, q := range req.Queries {
			queries = append(queries, &queryDto{
				RefId: q.RefId,
				Model: q.Model,
				Datasource: simplejson.NewFromAny(map[string]interface{}{
					"id":   q.DataSource.Id,
					"name": q.DataSource.Name,
				}),
				MaxDataPoints: q.MaxDataPoints,
				IntervalMs:    q.IntervalMs,
			})
		}

		data.Set("queries", queries)

		context.Logs = append(context.Logs, &alerting.ResultLogEntry{
			Message: fmt.Sprintf("Condition[%d]: Query", c.Index),
			Data:    data,
		})
	}

	resp, err := c.HandleRequest(context.Ctx, getDsInfo.Result, req)
	if err != nil {
		if err == gocontext.DeadlineExceeded {
			return nil, fmt.Errorf("Alert execution exceeded the timeout")
		}

		return nil, fmt.Errorf("tsdb.HandleRequest() error %v", err)
	}

	for _, v := range resp.Results {
		if v.Error != nil {
			return nil, fmt.Errorf("tsdb.HandleRequest() response error %v", v)
		}

		result = append(result, v.Series...)

		queryResultData := map[string]interface{}{}

		if context.IsTestRun {
			queryResultData["series"] = v.Series
		}

		if context.IsDebug && v.Meta != nil {
			queryResultData["meta"] = v.Meta
		}

		if context.IsTestRun || context.IsDebug {
			context.Logs = append(context.Logs, &alerting.ResultLogEntry{
				Message: fmt.Sprintf("Condition[%d]: Query Result", c.Index),
				Data:    simplejson.NewFromAny(queryResultData),
			})
		}
	}

	return result, nil
}

func (c *QueryCondition) getRequestForAlertRule(datasource *models.DataSource, timeRange *tsdb.TimeRange, debug bool) *tsdb.TsdbQuery {
	req := &tsdb.TsdbQuery{
		TimeRange: timeRange,
		Queries: []*tsdb.Query{
			{
				RefId:      "A",
				Model:      c.Query.Model,
				DataSource: datasource,
			},
		},
		Debug: debug,
	}

	return req
}

func newQueryCondition(model *simplejson.Json, index int) (*QueryCondition, error) {
	condition := QueryCondition{}
	condition.Index = index
	condition.HandleRequest = tsdb.HandleRequest

	queryJSON := model.Get("query")

	condition.Query.Model = queryJSON.Get("model")
	condition.Query.From = queryJSON.Get("params").MustArray()[1].(string)
	condition.Query.To = queryJSON.Get("params").MustArray()[2].(string)

	if err := validateFromValue(condition.Query.From); err != nil {
		return nil, err
	}

	if err := validateToValue(condition.Query.To); err != nil {
		return nil, err
	}

	condition.Query.DatasourceID = queryJSON.Get("datasourceId").MustInt64()

	reducerJSON := model.Get("reducer")
	condition.Reducer = newSimpleReducer(reducerJSON.Get("type").MustString())

	evaluatorJSON := model.Get("evaluator")
	evaluator, err := NewAlertEvaluator(evaluatorJSON)
	if err != nil {
		return nil, fmt.Errorf("error in condition %v: %v", index, err)
	}
	condition.Evaluator = evaluator

	operatorJSON := model.Get("operator")
	operator := operatorJSON.Get("type").MustString("and")
	condition.Operator = operator

	return &condition, nil
}

func validateFromValue(from string) error {
	fromRaw := strings.Replace(from, "now-", "", 1)

	_, err := time.ParseDuration("-" + fromRaw)
	return err
}

func validateToValue(to string) error {
	if to == "now" {
		return nil
	} else if strings.HasPrefix(to, "now-") {
		withoutNow := strings.Replace(to, "now-", "", 1)

		_, err := time.ParseDuration("-" + withoutNow)
		if err == nil {
			return nil
		}
	}

	_, err := time.ParseDuration(to)
	return err
}
