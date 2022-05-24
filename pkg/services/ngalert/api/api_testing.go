package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/expr"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/datasources"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

type TestingApiSrv struct {
	*AlertingProxy
	ExpressionService *expr.Service
	DatasourceCache   datasources.CacheService
	log               log.Logger
	accessControl     accesscontrol.AccessControl
	evaluator         eval.Evaluator
}

func (srv TestingApiSrv) RouteTestGrafanaRuleConfig(c *models.ReqContext, body apimodels.TestRulePayload) response.Response {
	if body.Type() != apimodels.GrafanaBackend || body.GrafanaManagedCondition == nil {
		return ErrResp(http.StatusBadRequest, errors.New("unexpected payload"), "")
	}

	if !authorizeDatasourceAccessForRule(&ngmodels.AlertRule{Data: body.GrafanaManagedCondition.Data}, func(evaluator accesscontrol.Evaluator) bool {
		return accesscontrol.HasAccess(srv.accessControl, c)(accesscontrol.ReqSignedIn, evaluator)
	}) {
		return ErrResp(http.StatusUnauthorized, fmt.Errorf("%w to query one or many data sources used by the rule", ErrAuthorization), "")
	}

	evalCond := ngmodels.Condition{
		Condition: body.GrafanaManagedCondition.Condition,
		OrgID:     c.SignedInUser.OrgId,
		Data:      body.GrafanaManagedCondition.Data,
	}

	if err := validateCondition(c.Req.Context(), evalCond, c.SignedInUser, c.SkipCache, srv.DatasourceCache); err != nil {
		return ErrResp(http.StatusBadRequest, err, "invalid condition")
	}

	now := body.GrafanaManagedCondition.Now
	if now.IsZero() {
		now = timeNow()
	}

	evalResults, err := srv.evaluator.ConditionEval(&evalCond, now, srv.ExpressionService)
	if err != nil {
		return ErrResp(http.StatusBadRequest, err, "Failed to evaluate conditions")
	}

	frame := evalResults.AsDataFrame()
	return response.JSONStreaming(http.StatusOK, util.DynMap{
		"instances": []*data.Frame{&frame},
	})
}

func (srv TestingApiSrv) RouteTestRuleConfig(c *models.ReqContext, body apimodels.TestRulePayload) response.Response {
	recipient := web.Params(c.Req)[":Recipient"]
	if body.Type() != apimodels.LoTexRulerBackend {
		return ErrResp(http.StatusBadRequest, errors.New("unexpected payload"), "")
	}

	var path string
	if datasourceID, err := strconv.ParseInt(recipient, 10, 64); err == nil {
		ds, err := srv.DatasourceCache.GetDatasource(context.Background(), datasourceID, c.SignedInUser, c.SkipCache)
		if err != nil {
			return ErrResp(http.StatusInternalServerError, err, "failed to get datasource")
		}

		switch ds.Type {
		case "loki":
			path = "loki/api/v1/query"
		case "prometheus":
			path = "api/v1/query"
		default:
			return ErrResp(http.StatusBadRequest, fmt.Errorf("unexpected recipient type %s", ds.Type), "")
		}
	}

	t := timeNow()
	queryURL, err := url.Parse(path)
	if err != nil {
		return ErrResp(http.StatusInternalServerError, err, "failed to parse url")
	}
	params := queryURL.Query()
	params.Set("query", body.Expr)
	params.Set("time", strconv.FormatInt(t.Unix(), 10))
	queryURL.RawQuery = params.Encode()
	return srv.withReq(
		c,
		http.MethodGet,
		queryURL,
		nil,
		instantQueryResultsExtractor,
		nil,
	)
}

func (srv TestingApiSrv) RouteEvalQueries(c *models.ReqContext, cmd apimodels.EvalQueriesPayload) response.Response {
	now := cmd.Now
	if now.IsZero() {
		now = timeNow()
	}

	if !authorizeDatasourceAccessForRule(&ngmodels.AlertRule{Data: cmd.Data}, func(evaluator accesscontrol.Evaluator) bool {
		return accesscontrol.HasAccess(srv.accessControl, c)(accesscontrol.ReqSignedIn, evaluator)
	}) {
		return ErrResp(http.StatusUnauthorized, fmt.Errorf("%w to query one or many data sources used by the rule", ErrAuthorization), "")
	}

	if _, err := validateQueriesAndExpressions(c.Req.Context(), cmd.Data, c.SignedInUser, c.SkipCache, srv.DatasourceCache); err != nil {
		return ErrResp(http.StatusBadRequest, err, "invalid queries or expressions")
	}

	evalResults, err := srv.evaluator.QueriesAndExpressionsEval(c.SignedInUser.OrgId, cmd.Data, now, srv.ExpressionService)
	if err != nil {
		return ErrResp(http.StatusBadRequest, err, "Failed to evaluate queries and expressions")
	}

	return response.JSONStreaming(http.StatusOK, evalResults)
}
