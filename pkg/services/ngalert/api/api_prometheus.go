package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/state"
	"github.com/grafana/grafana/pkg/services/ngalert/store"

	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type PrometheusSrv struct {
	log     log.Logger
	manager state.AlertInstanceManager
	store   store.RuleStore
	ac      accesscontrol.AccessControl
}

const queryIncludeInternalLabels = "includeInternalLabels"

func (srv PrometheusSrv) RouteGetAlertStatuses(c *models.ReqContext) response.Response {
	alertResponse := apimodels.AlertResponse{
		DiscoveryBase: apimodels.DiscoveryBase{
			Status: "success",
		},
		Data: apimodels.AlertDiscovery{
			Alerts: []*apimodels.Alert{},
		},
	}

	var labelOptions []ngmodels.LabelOption
	if !c.QueryBoolWithDefault(queryIncludeInternalLabels, false) {
		labelOptions = append(labelOptions, ngmodels.WithoutInternalLabels())
	}

	for _, alertState := range srv.manager.GetAll(c.OrgId) {
		startsAt := alertState.StartsAt
		valString := ""

		if alertState.State == eval.Alerting || alertState.State == eval.Pending {
			valString = formatValues(alertState)
		}

		alertResponse.Data.Alerts = append(alertResponse.Data.Alerts, &apimodels.Alert{
			Labels:      alertState.GetLabels(labelOptions...),
			Annotations: alertState.Annotations,
			State:       alertState.State.String(),
			ActiveAt:    &startsAt,
			Value:       valString,
		})
	}

	return response.JSON(http.StatusOK, alertResponse)
}

func formatValues(alertState *state.State) string {
	var fv string
	values := alertState.GetLastEvaluationValuesForCondition()

	switch len(values) {
	case 0:
		fv = alertState.LastEvaluationString
	case 1:
		for _, v := range values {
			fv = strconv.FormatFloat(v, 'e', -1, 64)
			break
		}

	default:
		vs := make([]string, 0, len(values))

		for k, v := range values {
			vs = append(vs, fmt.Sprintf("%s: %s", k, strconv.FormatFloat(v, 'e', -1, 64)))
		}

		// Ensure we have a consistent natural ordering after formatting e.g. A0, A1, A10, A11, A3, etc.
		sort.Strings(vs)
		fv = strings.Join(vs, ", ")
	}

	return fv
}

func getPanelIDFromRequest(r *http.Request) (int64, error) {
	if s := strings.TrimSpace(r.URL.Query().Get("panel_id")); s != "" {
		return strconv.ParseInt(s, 10, 64)
	}
	return 0, nil
}

func (srv PrometheusSrv) RouteGetRuleStatuses(c *models.ReqContext) response.Response {
	dashboardUID := c.Query("dashboard_uid")
	panelID, err := getPanelIDFromRequest(c.Req)
	if err != nil {
		return ErrResp(http.StatusBadRequest, err, "invalid panel_id")
	}
	if dashboardUID == "" && panelID != 0 {
		return ErrResp(http.StatusBadRequest, errors.New("panel_id must be set with dashboard_uid"), "")
	}

	ruleResponse := apimodels.RuleResponse{
		DiscoveryBase: apimodels.DiscoveryBase{
			Status: "success",
		},
		Data: apimodels.RuleDiscovery{
			RuleGroups: []*apimodels.RuleGroup{},
		},
	}

	var labelOptions []ngmodels.LabelOption
	if !c.QueryBoolWithDefault(queryIncludeInternalLabels, false) {
		labelOptions = append(labelOptions, ngmodels.WithoutInternalLabels())
	}

	namespaceMap, err := srv.store.GetUserVisibleNamespaces(c.Req.Context(), c.OrgId, c.SignedInUser)
	if err != nil {
		return ErrResp(http.StatusInternalServerError, err, "failed to get namespaces visible to the user")
	}

	if len(namespaceMap) == 0 {
		srv.log.Debug("User does not have access to any namespaces")
		return response.JSON(http.StatusOK, ruleResponse)
	}

	namespaceUIDs := make([]string, len(namespaceMap))
	for k := range namespaceMap {
		namespaceUIDs = append(namespaceUIDs, k)
	}

	alertRuleQuery := ngmodels.ListAlertRulesQuery{
		OrgID:         c.SignedInUser.OrgId,
		NamespaceUIDs: namespaceUIDs,
		DashboardUID:  dashboardUID,
		PanelID:       panelID,
	}
	if err := srv.store.ListAlertRules(c.Req.Context(), &alertRuleQuery); err != nil {
		ruleResponse.DiscoveryBase.Status = "error"
		ruleResponse.DiscoveryBase.Error = fmt.Sprintf("failure getting rules: %s", err.Error())
		ruleResponse.DiscoveryBase.ErrorType = apiv1.ErrServer
		return response.JSON(http.StatusInternalServerError, ruleResponse)
	}
	hasAccess := func(evaluator accesscontrol.Evaluator) bool {
		return accesscontrol.HasAccess(srv.ac, c)(accesscontrol.ReqSignedIn, evaluator)
	}

	groupMap := make(map[string]*apimodels.RuleGroup)

	for _, rule := range alertRuleQuery.Result {
		if !authorizeDatasourceAccessForRule(rule, hasAccess) {
			continue
		}
		groupKey := rule.RuleGroup + "-" + rule.NamespaceUID
		newGroup, ok := groupMap[groupKey]
		if !ok {
			folder := namespaceMap[rule.NamespaceUID]
			if folder == nil {
				srv.log.Warn("query returned rules that belong to folder the user does not have access to. The rule will not be added to the response", "folder_uid", rule.NamespaceUID, "rule_uid", rule.UID)
				continue
			}
			newGroup = &apimodels.RuleGroup{
				Name: rule.RuleGroup,
				File: folder.Title, // file is what Prometheus uses for provisioning, we replace it with namespace.
			}
			groupMap[groupKey] = newGroup
			ruleResponse.Data.RuleGroups = append(ruleResponse.Data.RuleGroups, newGroup)
		}

		alertingRule := apimodels.AlertingRule{
			State:       "inactive",
			Name:        rule.Title,
			Query:       ruleToQuery(srv.log, rule),
			Duration:    rule.For.Seconds(),
			Annotations: rule.Annotations,
		}

		newRule := apimodels.Rule{
			Name:           rule.Title,
			Labels:         rule.GetLabels(labelOptions...),
			Health:         "ok",
			Type:           apiv1.RuleTypeAlerting,
			LastEvaluation: time.Time{},
		}

		for _, alertState := range srv.manager.GetStatesForRuleUID(c.OrgId, rule.UID) {
			activeAt := alertState.StartsAt
			valString := ""
			if alertState.State == eval.Alerting || alertState.State == eval.Pending {
				valString = formatValues(alertState)
			}

			alert := &apimodels.Alert{
				Labels:      alertState.GetLabels(labelOptions...),
				Annotations: alertState.Annotations,
				State:       alertState.State.String(),
				ActiveAt:    &activeAt,
				Value:       valString,
			}

			if alertState.LastEvaluationTime.After(newRule.LastEvaluation) {
				newRule.LastEvaluation = alertState.LastEvaluationTime
			}

			newRule.EvaluationTime = alertState.EvaluationDuration.Seconds()

			switch alertState.State {
			case eval.Normal:
			case eval.Pending:
				if alertingRule.State == "inactive" {
					alertingRule.State = "pending"
				}
			case eval.Alerting:
				alertingRule.State = "firing"
			case eval.Error:
				newRule.Health = "error"
			case eval.NoData:
				newRule.Health = "nodata"
			}

			if alertState.Error != nil {
				newRule.LastError = alertState.Error.Error()
				newRule.Health = "error"
			}

			alertingRule.Alerts = append(alertingRule.Alerts, alert)
		}

		alertingRule.Rule = newRule
		newGroup.Rules = append(newGroup.Rules, alertingRule)
		newGroup.Interval = float64(rule.IntervalSeconds)
		newGroup.EvaluationTime = newRule.EvaluationTime
		newGroup.LastEvaluation = newRule.LastEvaluation
	}

	return response.JSON(http.StatusOK, ruleResponse)
}

// ruleToQuery attempts to extract the datasource queries from the alert query model.
// Returns the whole JSON model as a string if it fails to extract a minimum of 1 query.
func ruleToQuery(logger log.Logger, rule *ngmodels.AlertRule) string {
	var queryErr error
	var queries []string

	for _, q := range rule.Data {
		q, err := q.GetQuery()
		if err != nil {
			// If we can't find the query simply omit it, and try the rest.
			// Even single query alerts would have 2 `AlertQuery`, one for the query and one for the condition.
			if errors.Is(err, ngmodels.ErrNoQuery) {
				continue
			}

			// For any other type of error, it is unexpected abort and return the whole JSON.
			logger.Debug("failed to parse a query", "err", err)
			queryErr = err
			break
		}

		queries = append(queries, q)
	}

	// If we were able to extract at least one query without failure use it.
	if queryErr == nil && len(queries) > 0 {
		return strings.Join(queries, " | ")
	}

	return encodedQueriesOrError(rule.Data)
}

// encodedQueriesOrError tries to encode rule query data into JSON if it fails returns the encoding error as a string.
func encodedQueriesOrError(rules []ngmodels.AlertQuery) string {
	encodedQueries, err := json.Marshal(rules)
	if err == nil {
		return string(encodedQueries)
	}

	return err.Error()
}
