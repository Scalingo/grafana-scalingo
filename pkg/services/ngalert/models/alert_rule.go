package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/grafana/grafana/pkg/util/cmputil"
)

var (
	// ErrAlertRuleNotFound is an error for an unknown alert rule.
	ErrAlertRuleNotFound = fmt.Errorf("could not find alert rule")
	// ErrAlertRuleFailedGenerateUniqueUID is an error for failure to generate alert rule UID
	ErrAlertRuleFailedGenerateUniqueUID = errors.New("failed to generate alert rule UID")
	// ErrCannotEditNamespace is an error returned if the user does not have permissions to edit the namespace
	ErrCannotEditNamespace                = errors.New("user does not have permissions to edit the namespace")
	ErrRuleGroupNamespaceNotFound         = errors.New("rule group not found under this namespace")
	ErrAlertRuleFailedValidation          = errors.New("invalid alert rule")
	ErrAlertRuleUniqueConstraintViolation = errors.New("a conflicting alert rule is found: rule title under the same organisation and folder should be unique")
)

type NoDataState string

func (noDataState NoDataState) String() string {
	return string(noDataState)
}

func NoDataStateFromString(state string) (NoDataState, error) {
	switch state {
	case string(Alerting):
		return Alerting, nil
	case string(NoData):
		return NoData, nil
	case string(OK):
		return OK, nil
	default:
		return "", fmt.Errorf("unknown NoData state option %s", state)
	}
}

const (
	Alerting NoDataState = "Alerting"
	NoData   NoDataState = "NoData"
	OK       NoDataState = "OK"
)

type ExecutionErrorState string

func (executionErrorState ExecutionErrorState) String() string {
	return string(executionErrorState)
}

func ErrStateFromString(opt string) (ExecutionErrorState, error) {
	switch opt {
	case string(Alerting):
		return AlertingErrState, nil
	case string(ErrorErrState):
		return ErrorErrState, nil
	case string(OkErrState):
		return OkErrState, nil
	default:
		return "", fmt.Errorf("unknown Error state option %s", opt)
	}
}

const (
	AlertingErrState ExecutionErrorState = "Alerting"
	ErrorErrState    ExecutionErrorState = "Error"
	OkErrState       ExecutionErrorState = "OK"
)

// InternalLabelNameSet are labels that grafana automatically include as part of the labelset.
var InternalLabelNameSet = map[string]struct{}{
	RuleUIDLabel:      {},
	NamespaceUIDLabel: {},
}

const (
	RuleUIDLabel      = "__alert_rule_uid__"
	NamespaceUIDLabel = "__alert_rule_namespace_uid__"

	// Annotations are actually a set of labels, so technically this is the label name of an annotation.
	DashboardUIDAnnotation = "__dashboardUid__"
	PanelIDAnnotation      = "__panelId__"
)

// AlertRule is the model for alert rules in unified alerting.
type AlertRule struct {
	ID              int64 `xorm:"pk autoincr 'id'"`
	OrgID           int64 `xorm:"org_id"`
	Title           string
	Condition       string
	Data            []AlertQuery
	Updated         time.Time
	IntervalSeconds int64
	Version         int64
	UID             string  `xorm:"uid"`
	NamespaceUID    string  `xorm:"namespace_uid"`
	DashboardUID    *string `xorm:"dashboard_uid"`
	PanelID         *int64  `xorm:"panel_id"`
	RuleGroup       string
	NoDataState     NoDataState
	ExecErrState    ExecutionErrorState
	// ideally this field should have been apimodels.ApiDuration
	// but this is currently not possible because of circular dependencies
	For         time.Duration
	Annotations map[string]string
	Labels      map[string]string
}

type LabelOption func(map[string]string)

func WithoutInternalLabels() LabelOption {
	return func(labels map[string]string) {
		for k := range labels {
			if _, ok := InternalLabelNameSet[k]; ok {
				delete(labels, k)
			}
		}
	}
}

// GetLabels returns the labels specified as part of the alert rule.
func (alertRule *AlertRule) GetLabels(opts ...LabelOption) map[string]string {
	labels := alertRule.Labels

	for _, opt := range opts {
		opt(labels)
	}

	return labels
}

// Diff calculates diff between two alert rules. Returns nil if two rules are equal. Otherwise, returns cmputil.DiffReport
func (alertRule *AlertRule) Diff(rule *AlertRule, ignore ...string) cmputil.DiffReport {
	var reporter cmputil.DiffReporter
	ops := make([]cmp.Option, 0, 4)

	// json.RawMessage is a slice of bytes and therefore cmp's default behavior is to compare it by byte, which is not really useful
	var jsonCmp = cmp.Transformer("", func(in json.RawMessage) string {
		return string(in)
	})
	ops = append(ops, cmp.Reporter(&reporter), cmpopts.IgnoreFields(AlertQuery{}, "modelProps"), jsonCmp)

	if len(ignore) > 0 {
		ops = append(ops, cmpopts.IgnoreFields(AlertRule{}, ignore...))
	}
	cmp.Equal(alertRule, rule, ops...)
	return reporter.Diffs
}

// AlertRuleKey is the alert definition identifier
type AlertRuleKey struct {
	OrgID int64
	UID   string
}

func (k AlertRuleKey) String() string {
	return fmt.Sprintf("{orgID: %d, UID: %s}", k.OrgID, k.UID)
}

// GetKey returns the alert definitions identifier
func (alertRule *AlertRule) GetKey() AlertRuleKey {
	return AlertRuleKey{OrgID: alertRule.OrgID, UID: alertRule.UID}
}

// PreSave sets default values and loads the updated model for each alert query.
func (alertRule *AlertRule) PreSave(timeNow func() time.Time) error {
	for i, q := range alertRule.Data {
		err := q.PreSave()
		if err != nil {
			return fmt.Errorf("invalid alert query %s: %w", q.RefID, err)
		}
		alertRule.Data[i] = q
	}
	alertRule.Updated = timeNow()
	return nil
}

func (alertRule *AlertRule) ResourceType() string {
	return "alertRule"
}

func (alertRule *AlertRule) ResourceID() string {
	return alertRule.UID
}

func (alertRule *AlertRule) ResourceOrgID() int64 {
	return alertRule.OrgID
}

// AlertRuleVersion is the model for alert rule versions in unified alerting.
type AlertRuleVersion struct {
	ID               int64  `xorm:"pk autoincr 'id'"`
	RuleOrgID        int64  `xorm:"rule_org_id"`
	RuleUID          string `xorm:"rule_uid"`
	RuleNamespaceUID string `xorm:"rule_namespace_uid"`
	RuleGroup        string
	ParentVersion    int64
	RestoredFrom     int64
	Version          int64

	Created         time.Time
	Title           string
	Condition       string
	Data            []AlertQuery
	IntervalSeconds int64
	NoDataState     NoDataState
	ExecErrState    ExecutionErrorState
	// ideally this field should have been apimodels.ApiDuration
	// but this is currently not possible because of circular dependencies
	For         time.Duration
	Annotations map[string]string
	Labels      map[string]string
}

// GetAlertRuleByUIDQuery is the query for retrieving/deleting an alert rule by UID and organisation ID.
type GetAlertRuleByUIDQuery struct {
	UID   string
	OrgID int64

	Result *AlertRule
}

// ListAlertRulesQuery is the query for listing alert rules
type ListAlertRulesQuery struct {
	OrgID         int64
	NamespaceUIDs []string
	ExcludeOrgs   []int64
	RuleGroup     string

	// DashboardUID and PanelID are optional and allow filtering rules
	// to return just those for a dashboard and panel.
	DashboardUID string
	PanelID      int64

	Result []*AlertRule
}

// ListNamespaceAlertRulesQuery is the query for listing namespace alert rules
type ListNamespaceAlertRulesQuery struct {
	OrgID int64
	// Namespace is the folder slug
	NamespaceUID string

	Result []*AlertRule
}

// ListRuleGroupsQuery is the query for listing unique rule groups
// across all organizations
type ListRuleGroupsQuery struct {
	Result []string
}

// ListOrgRuleGroupsQuery is the query for listing unique rule groups
// for an organization
type ListOrgRuleGroupsQuery struct {
	OrgID         int64
	NamespaceUIDs []string

	// DashboardUID and PanelID are optional and allow filtering rules
	// to return just those for a dashboard and panel.
	DashboardUID string
	PanelID      int64

	Result [][]string
}

// Condition contains backend expressions and queries and the RefID
// of the query or expression that will be evaluated.
type Condition struct {
	// Condition is the RefID of the query or expression from
	// the Data property to get the results for.
	Condition string `json:"condition"`
	OrgID     int64  `json:"-"`

	// Data is an array of data source queries and/or server side expressions.
	Data []AlertQuery `json:"data"`
}

// IsValid checks the condition's validity.
func (c Condition) IsValid() bool {
	// TODO search for refIDs in QueriesAndExpressions
	return len(c.Data) != 0
}

// PatchPartialAlertRule patches `ruleToPatch` by `existingRule` following the rule that if a field of `ruleToPatch` is empty or has the default value, it is populated by the value of the corresponding field from `existingRule`.
// There are several exceptions:
// 1. Following fields are not patched and therefore will be ignored: AlertRule.ID, AlertRule.OrgID, AlertRule.Updated, AlertRule.Version, AlertRule.UID, AlertRule.DashboardUID, AlertRule.PanelID, AlertRule.Annotations and AlertRule.Labels
// 2. There are fields that are patched together:
//    - AlertRule.Condition and AlertRule.Data
// If either of the pair is specified, neither is patched.
func PatchPartialAlertRule(existingRule *AlertRule, ruleToPatch *AlertRule) {
	if ruleToPatch.Title == "" {
		ruleToPatch.Title = existingRule.Title
	}
	if ruleToPatch.Condition == "" || len(ruleToPatch.Data) == 0 {
		ruleToPatch.Condition = existingRule.Condition
		ruleToPatch.Data = existingRule.Data
	}
	if ruleToPatch.IntervalSeconds == 0 {
		ruleToPatch.IntervalSeconds = existingRule.IntervalSeconds
	}
	if ruleToPatch.NamespaceUID == "" {
		ruleToPatch.NamespaceUID = existingRule.NamespaceUID
	}
	if ruleToPatch.RuleGroup == "" {
		ruleToPatch.RuleGroup = existingRule.RuleGroup
	}
	if ruleToPatch.ExecErrState == "" {
		ruleToPatch.ExecErrState = existingRule.ExecErrState
	}
	if ruleToPatch.NoDataState == "" {
		ruleToPatch.NoDataState = existingRule.NoDataState
	}
	if ruleToPatch.For == 0 {
		ruleToPatch.For = existingRule.For
	}
}
