package schedule

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	prometheusModel "github.com/prometheus/common/model"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	"github.com/grafana/grafana/pkg/services/ngalert/metrics"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/state"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util/ticker"

	"github.com/benbjohnson/clock"
	"golang.org/x/sync/errgroup"
)

// ScheduleService is an interface for a service that schedules the evaluation
// of alert rules.
//
//go:generate mockery --name ScheduleService --structname FakeScheduleService --inpackage --filename schedule_mock.go --unroll-variadic=False
type ScheduleService interface {
	// Run the scheduler until the context is canceled or the scheduler returns
	// an error. The scheduler is terminated when this function returns.
	Run(context.Context) error
	// UpdateAlertRule notifies scheduler that a rule has been changed
	UpdateAlertRule(key ngmodels.AlertRuleKey, lastVersion int64)
	// DeleteAlertRule notifies scheduler that rules have been deleted
	DeleteAlertRule(keys ...ngmodels.AlertRuleKey)
}

// AlertsSender is an interface for a service that is responsible for sending notifications to the end-user.
//
//go:generate mockery --name AlertsSender --structname AlertsSenderMock --inpackage --filename alerts_sender_mock.go --with-expecter
type AlertsSender interface {
	Send(key ngmodels.AlertRuleKey, alerts definitions.PostableAlerts)
}

// RulesStore is a store that provides alert rules for scheduling
type RulesStore interface {
	GetAlertRulesKeysForScheduling(ctx context.Context) ([]ngmodels.AlertRuleKeyWithVersion, error)
	GetAlertRulesForScheduling(ctx context.Context, query *ngmodels.GetAlertRulesForSchedulingQuery) error
}

type schedule struct {
	// base tick rate (fastest possible configured check)
	baseInterval time.Duration

	// each alert rule gets its own channel and routine
	registry alertRuleInfoRegistry

	maxAttempts int64

	clock clock.Clock

	// evalApplied is only used for tests: test code can set it to non-nil
	// function, and then it'll be called from the event loop whenever the
	// message from evalApplied is handled.
	evalAppliedFunc func(ngmodels.AlertRuleKey, time.Time)

	// stopApplied is only used for tests: test code can set it to non-nil
	// function, and then it'll be called from the event loop whenever the
	// message from stopApplied is handled.
	stopAppliedFunc func(ngmodels.AlertRuleKey)

	log log.Logger

	evaluatorFactory eval.EvaluatorFactory

	ruleStore RulesStore

	stateManager *state.Manager

	appURL               *url.URL
	disableGrafanaFolder bool

	metrics *metrics.Scheduler

	alertsSender    AlertsSender
	minRuleInterval time.Duration

	// schedulableAlertRules contains the alert rules that are considered for
	// evaluation in the current tick. The evaluation of an alert rule in the
	// current tick depends on its evaluation interval and when it was
	// last evaluated.
	schedulableAlertRules alertRulesRegistry
}

// SchedulerCfg is the scheduler configuration.
type SchedulerCfg struct {
	Cfg              setting.UnifiedAlertingSettings
	C                clock.Clock
	EvaluatorFactory eval.EvaluatorFactory
	RuleStore        RulesStore
	Metrics          *metrics.Scheduler
	AlertSender      AlertsSender
}

// NewScheduler returns a new schedule.
func NewScheduler(cfg SchedulerCfg, appURL *url.URL, stateManager *state.Manager) *schedule {
	sch := schedule{
		registry:              alertRuleInfoRegistry{alertRuleInfo: make(map[ngmodels.AlertRuleKey]*alertRuleInfo)},
		maxAttempts:           cfg.Cfg.MaxAttempts,
		clock:                 cfg.C,
		baseInterval:          cfg.Cfg.BaseInterval,
		log:                   log.New("ngalert.scheduler"),
		evaluatorFactory:      cfg.EvaluatorFactory,
		ruleStore:             cfg.RuleStore,
		metrics:               cfg.Metrics,
		appURL:                appURL,
		disableGrafanaFolder:  cfg.Cfg.ReservedLabels.IsReservedLabelDisabled(ngmodels.FolderTitleLabel),
		stateManager:          stateManager,
		minRuleInterval:       cfg.Cfg.MinInterval,
		schedulableAlertRules: alertRulesRegistry{rules: make(map[ngmodels.AlertRuleKey]*ngmodels.AlertRule)},
		alertsSender:          cfg.AlertSender,
	}

	return &sch
}

func (sch *schedule) Run(ctx context.Context) error {
	t := ticker.New(sch.clock, sch.baseInterval, sch.metrics.Ticker)
	defer t.Stop()

	if err := sch.schedulePeriodic(ctx, t); err != nil {
		sch.log.Error("Failure while running the rule evaluation loop", "error", err)
	}
	return nil
}

// UpdateAlertRule looks for the active rule evaluation and commands it to update the rule
func (sch *schedule) UpdateAlertRule(key ngmodels.AlertRuleKey, lastVersion int64) {
	ruleInfo, err := sch.registry.get(key)
	if err != nil {
		return
	}
	ruleInfo.update(ruleVersion(lastVersion))
}

// DeleteAlertRule stops evaluation of the rule, deletes it from active rules, and cleans up state cache.
func (sch *schedule) DeleteAlertRule(keys ...ngmodels.AlertRuleKey) {
	for _, key := range keys {
		// It can happen that the scheduler has deleted the alert rule before the
		// Ruler API has called DeleteAlertRule. This can happen as requests to
		// the Ruler API do not hold an exclusive lock over all scheduler operations.
		if _, ok := sch.schedulableAlertRules.del(key); !ok {
			sch.log.Info("Alert rule cannot be removed from the scheduler as it is not scheduled", key.LogContext()...)
		}
		// Delete the rule routine
		ruleInfo, ok := sch.registry.del(key)
		if !ok {
			sch.log.Info("Alert rule cannot be stopped as it is not running", key.LogContext()...)
			continue
		}
		// stop rule evaluation
		ruleInfo.stop(errRuleDeleted)
	}
	// Our best bet at this point is that we update the metrics with what we hope to schedule in the next tick.
	alertRules, _ := sch.schedulableAlertRules.all()
	sch.metrics.SchedulableAlertRules.Set(float64(len(alertRules)))
	sch.metrics.SchedulableAlertRulesHash.Set(float64(hashUIDs(alertRules)))
}

func (sch *schedule) schedulePeriodic(ctx context.Context, t *ticker.T) error {
	dispatcherGroup, ctx := errgroup.WithContext(ctx)
	for {
		select {
		case tick := <-t.C:
			// We use Round(0) on the start time to remove the monotonic clock.
			// This is required as ticks from the ticker and time.Now() can have
			// a monotonic clock that when subtracted do not represent the delta
			// in wall clock time.
			start := time.Now().Round(0)
			sch.metrics.BehindSeconds.Set(start.Sub(tick).Seconds())

			sch.processTick(ctx, dispatcherGroup, tick)

			sch.metrics.SchedulePeriodicDuration.Observe(time.Since(start).Seconds())
		case <-ctx.Done():
			// waiting for all rule evaluation routines to stop
			waitErr := dispatcherGroup.Wait()
			return waitErr
		}
	}
}

type readyToRunItem struct {
	ruleInfo *alertRuleInfo
	evaluation
}

func (sch *schedule) processTick(ctx context.Context, dispatcherGroup *errgroup.Group, tick time.Time) ([]readyToRunItem, map[ngmodels.AlertRuleKey]struct{}) {
	tickNum := tick.Unix() / int64(sch.baseInterval.Seconds())

	if err := sch.updateSchedulableAlertRules(ctx); err != nil {
		sch.log.Error("Failed to update alert rules", "error", err)
	}
	alertRules, folderTitles := sch.schedulableAlertRules.all()

	// registeredDefinitions is a map used for finding deleted alert rules
	// initially it is assigned to all known alert rules from the previous cycle
	// each alert rule found also in this cycle is removed
	// so, at the end, the remaining registered alert rules are the deleted ones
	registeredDefinitions := sch.registry.keyMap()

	// While these are the rules that we iterate over, at the moment there's no 100% guarantee that they'll be
	// scheduled as rules could be removed before we get a chance to evaluate them.
	sch.metrics.SchedulableAlertRules.Set(float64(len(alertRules)))
	sch.metrics.SchedulableAlertRulesHash.Set(float64(hashUIDs(alertRules)))

	readyToRun := make([]readyToRunItem, 0)
	missingFolder := make(map[string][]string)
	for _, item := range alertRules {
		key := item.GetKey()
		ruleInfo, newRoutine := sch.registry.getOrCreateInfo(ctx, key)

		// enforce minimum evaluation interval
		if item.IntervalSeconds < int64(sch.minRuleInterval.Seconds()) {
			sch.log.Debug("Interval adjusted", append(key.LogContext(), "originalInterval", item.IntervalSeconds, "adjustedInterval", sch.minRuleInterval.Seconds())...)
			item.IntervalSeconds = int64(sch.minRuleInterval.Seconds())
		}

		invalidInterval := item.IntervalSeconds%int64(sch.baseInterval.Seconds()) != 0

		if newRoutine && !invalidInterval {
			dispatcherGroup.Go(func() error {
				return sch.ruleRoutine(ruleInfo.ctx, key, ruleInfo.evalCh, ruleInfo.updateCh)
			})
		}

		if invalidInterval {
			// this is expected to be always false
			// given that we validate interval during alert rule updates
			sch.log.Warn("Rule has an invalid interval and will be ignored. Interval should be divided exactly by scheduler interval", append(key.LogContext(), "ruleInterval", time.Duration(item.IntervalSeconds)*time.Second, "schedulerInterval", sch.baseInterval)...)
			continue
		}

		itemFrequency := item.IntervalSeconds / int64(sch.baseInterval.Seconds())
		if item.IntervalSeconds != 0 && tickNum%itemFrequency == 0 {
			var folderTitle string
			if !sch.disableGrafanaFolder {
				title, ok := folderTitles[item.NamespaceUID]
				if ok {
					folderTitle = title
				} else {
					missingFolder[item.NamespaceUID] = append(missingFolder[item.NamespaceUID], item.UID)
				}
			}
			readyToRun = append(readyToRun, readyToRunItem{ruleInfo: ruleInfo, evaluation: evaluation{
				scheduledAt: tick,
				rule:        item,
				folderTitle: folderTitle,
			}})
		}

		// remove the alert rule from the registered alert rules
		delete(registeredDefinitions, key)
	}

	if len(missingFolder) > 0 { // if this happens then there can be problems with fetching folders from the database.
		sch.log.Warn("Unable to obtain folder titles for some rules", "missingFolderUIDToRuleUID", missingFolder)
	}

	var step int64 = 0
	if len(readyToRun) > 0 {
		step = sch.baseInterval.Nanoseconds() / int64(len(readyToRun))
	}

	for i := range readyToRun {
		item := readyToRun[i]

		time.AfterFunc(time.Duration(int64(i)*step), func() {
			key := item.rule.GetKey()
			success, dropped := item.ruleInfo.eval(&item.evaluation)
			if !success {
				sch.log.Debug("Scheduled evaluation was canceled because evaluation routine was stopped", append(key.LogContext(), "time", tick)...)
				return
			}
			if dropped != nil {
				sch.log.Warn("Tick dropped because alert rule evaluation is too slow", append(key.LogContext(), "time", tick)...)
				orgID := fmt.Sprint(key.OrgID)
				sch.metrics.EvaluationMissed.WithLabelValues(orgID, item.rule.Title).Inc()
			}
		})
	}

	// unregister and stop routines of the deleted alert rules
	for key := range registeredDefinitions {
		sch.DeleteAlertRule(key)
	}

	return readyToRun, registeredDefinitions
}

func (sch *schedule) ruleRoutine(grafanaCtx context.Context, key ngmodels.AlertRuleKey, evalCh <-chan *evaluation, updateCh <-chan ruleVersion) error {
	grafanaCtx = ngmodels.WithRuleKey(grafanaCtx, key)
	logger := sch.log.FromContext(grafanaCtx)
	logger.Debug("Alert rule routine started")

	orgID := fmt.Sprint(key.OrgID)
	evalTotal := sch.metrics.EvalTotal.WithLabelValues(orgID)
	evalDuration := sch.metrics.EvalDuration.WithLabelValues(orgID)
	evalTotalFailures := sch.metrics.EvalFailures.WithLabelValues(orgID)

	clearState := func() {
		states := sch.stateManager.ResetStateByRuleUID(grafanaCtx, key)
		expiredAlerts := FromAlertsStateToStoppedAlert(states, sch.appURL, sch.clock)
		if len(expiredAlerts.PostableAlerts) > 0 {
			sch.alertsSender.Send(key, expiredAlerts)
		}
	}

	evaluate := func(ctx context.Context, attempt int64, e *evaluation) {
		logger := logger.New("version", e.rule.Version, "attempt", attempt, "now", e.scheduledAt)
		start := sch.clock.Now()

		schedulerUser := &user.SignedInUser{
			UserID:           -1,
			IsServiceAccount: true,
			Login:            "grafana_scheduler",
			OrgID:            e.rule.OrgID,
			OrgRole:          org.RoleAdmin,
			Permissions: map[int64]map[string][]string{
				e.rule.OrgID: {
					datasources.ActionQuery: []string{
						datasources.ScopeAll,
					},
				},
			},
		}
		evalCtx := eval.Context(ctx, schedulerUser)
		ruleEval, err := sch.evaluatorFactory.Create(evalCtx, e.rule.GetEvalCondition())
		var results eval.Results
		var dur time.Duration
		if err == nil {
			results, err = ruleEval.Evaluate(ctx, e.scheduledAt)
			if err != nil {
				logger.Error("Failed to evaluate rule", "error", err, "duration", dur)
			}
		} else {
			logger.Error("Failed to build rule evaluator", "error", err)
		}
		dur = sch.clock.Now().Sub(start)

		evalTotal.Inc()
		evalDuration.Observe(dur.Seconds())

		if err != nil || results.HasErrors() {
			evalTotalFailures.Inc()
			if results == nil {
				results = append(results, eval.NewResultFromError(err, e.scheduledAt, dur))
			}
		} else {
			logger.Debug("Alert rule evaluated", "results", results, "duration", dur)
		}
		if ctx.Err() != nil { // check if the context is not cancelled. The evaluation can be a long-running task.
			logger.Debug("Skip updating the state because the context has been cancelled")
			return
		}
		processedStates := sch.stateManager.ProcessEvalResults(ctx, e.scheduledAt, e.rule, results, sch.getRuleExtraLabels(e))
		alerts := FromAlertStateToPostableAlerts(processedStates, sch.stateManager, sch.appURL)
		if len(alerts.PostableAlerts) > 0 {
			sch.alertsSender.Send(key, alerts)
		}
	}

	retryIfError := func(f func(attempt int64) error) error {
		var attempt int64
		var err error
		for attempt = 0; attempt < sch.maxAttempts; attempt++ {
			err = f(attempt)
			if err == nil {
				return nil
			}
		}
		return err
	}

	evalRunning := false
	var currentRuleVersion int64 = 0
	defer sch.stopApplied(key)
	for {
		select {
		// used by external services (API) to notify that rule is updated.
		case lastVersion := <-updateCh:
			// sometimes it can happen when, for example, the rule evaluation took so long,
			// and there were two concurrent messages in updateCh and evalCh, and the eval's one got processed first.
			// therefore, at the time when message from updateCh is processed the current rule will have
			// at least the same version (or greater) and the state created for the new version of the rule.
			if currentRuleVersion >= int64(lastVersion) {
				logger.Info("Skip updating rule because its current version is actual", "version", currentRuleVersion, "newVersion", lastVersion)
				continue
			}
			logger.Info("Clearing the state of the rule because version has changed", "version", currentRuleVersion, "newVersion", lastVersion)
			// clear the state. So the next evaluation will start from the scratch.
			clearState()
		// evalCh - used by the scheduler to signal that evaluation is needed.
		case ctx, ok := <-evalCh:
			if !ok {
				logger.Debug("Evaluation channel has been closed. Exiting")
				return nil
			}
			if evalRunning {
				continue
			}

			func() {
				evalRunning = true
				defer func() {
					evalRunning = false
					sch.evalApplied(key, ctx.scheduledAt)
				}()

				err := retryIfError(func(attempt int64) error {
					newVersion := ctx.rule.Version
					// fetch latest alert rule version
					if currentRuleVersion != newVersion {
						if currentRuleVersion > 0 { // do not clean up state if the eval loop has just started.
							logger.Debug("Got a new version of alert rule. Clear up the state and refresh extra labels", "version", currentRuleVersion, "newVersion", newVersion)
							clearState()
						}
						currentRuleVersion = newVersion
					}
					evaluate(grafanaCtx, attempt, ctx)
					return nil
				})
				if err != nil {
					logger.Error("Evaluation failed after all retries", "error", err)
				}
			}()
		case <-grafanaCtx.Done():
			// clean up the state only if the reason for stopping the evaluation loop is that the rule was deleted
			if errors.Is(grafanaCtx.Err(), errRuleDeleted) {
				clearState()
			}
			logger.Debug("Stopping alert rule routine")
			return nil
		}
	}
}

// evalApplied is only used on tests.
func (sch *schedule) evalApplied(alertDefKey ngmodels.AlertRuleKey, now time.Time) {
	if sch.evalAppliedFunc == nil {
		return
	}

	sch.evalAppliedFunc(alertDefKey, now)
}

// stopApplied is only used on tests.
func (sch *schedule) stopApplied(alertDefKey ngmodels.AlertRuleKey) {
	if sch.stopAppliedFunc == nil {
		return
	}

	sch.stopAppliedFunc(alertDefKey)
}

func (sch *schedule) getRuleExtraLabels(evalCtx *evaluation) map[string]string {
	extraLabels := make(map[string]string, 4)

	extraLabels[ngmodels.NamespaceUIDLabel] = evalCtx.rule.NamespaceUID
	extraLabels[prometheusModel.AlertNameLabel] = evalCtx.rule.Title
	extraLabels[ngmodels.RuleUIDLabel] = evalCtx.rule.UID

	if !sch.disableGrafanaFolder {
		extraLabels[ngmodels.FolderTitleLabel] = evalCtx.folderTitle
	}
	return extraLabels
}
