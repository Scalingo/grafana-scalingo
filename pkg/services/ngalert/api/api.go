package api

import (
	"context"
	"net/url"
	"time"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/expr"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/datasourceproxy"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	"github.com/grafana/grafana/pkg/services/ngalert/metrics"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/notifier"
	"github.com/grafana/grafana/pkg/services/ngalert/provisioning"
	"github.com/grafana/grafana/pkg/services/ngalert/schedule"
	"github.com/grafana/grafana/pkg/services/ngalert/state"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/services/quota"
	"github.com/grafana/grafana/pkg/services/secrets"
	"github.com/grafana/grafana/pkg/setting"
)

// timeNow makes it possible to test usage of time
var timeNow = time.Now

type Scheduler interface {
	AlertmanagersFor(orgID int64) []*url.URL
	DroppedAlertmanagersFor(orgID int64) []*url.URL
}

type Alertmanager interface {
	// Configuration
	SaveAndApplyConfig(ctx context.Context, config *apimodels.PostableUserConfig) error
	SaveAndApplyDefaultConfig(ctx context.Context) error
	GetStatus() apimodels.GettableStatus

	// Silences
	CreateSilence(ps *apimodels.PostableSilence) (string, error)
	DeleteSilence(silenceID string) error
	GetSilence(silenceID string) (apimodels.GettableSilence, error)
	ListSilences(filter []string) (apimodels.GettableSilences, error)

	// Alerts
	GetAlerts(active, silenced, inhibited bool, filter []string, receiver string) (apimodels.GettableAlerts, error)
	GetAlertGroups(active, silenced, inhibited bool, filter []string, receiver string) (apimodels.AlertGroups, error)

	// Testing
	TestReceivers(ctx context.Context, c apimodels.TestReceiversConfigBodyParams) (*notifier.TestReceiversResult, error)
}

type AlertingStore interface {
	GetLatestAlertmanagerConfiguration(ctx context.Context, query *models.GetLatestAlertmanagerConfigurationQuery) error
}

// API handlers.
type API struct {
	Cfg                  *setting.Cfg
	DatasourceCache      datasources.CacheService
	RouteRegister        routing.RouteRegister
	ExpressionService    *expr.Service
	QuotaService         *quota.QuotaService
	Schedule             schedule.ScheduleService
	TransactionManager   provisioning.TransactionManager
	ProvenanceStore      provisioning.ProvisioningStore
	RuleStore            store.RuleStore
	InstanceStore        store.InstanceStore
	AlertingStore        AlertingStore
	AdminConfigStore     store.AdminConfigurationStore
	DataProxy            *datasourceproxy.DataSourceProxyService
	MultiOrgAlertmanager *notifier.MultiOrgAlertmanager
	StateManager         *state.Manager
	SecretsService       secrets.Service
	AccessControl        accesscontrol.AccessControl
	Policies             *provisioning.NotificationPolicyService
	ContactPointService  *provisioning.ContactPointService
	Templates            *provisioning.TemplateService
}

// RegisterAPIEndpoints registers API handlers
func (api *API) RegisterAPIEndpoints(m *metrics.API) {
	logger := log.New("ngalert.api")
	proxy := &AlertingProxy{
		DataProxy: api.DataProxy,
	}

	// Register endpoints for proxying to Alertmanager-compatible backends.
	api.RegisterAlertmanagerApiEndpoints(NewForkedAM(
		api.DatasourceCache,
		NewLotexAM(proxy, logger),
		&AlertmanagerSrv{crypto: api.MultiOrgAlertmanager.Crypto, log: logger, ac: api.AccessControl, mam: api.MultiOrgAlertmanager},
	), m)
	// Register endpoints for proxying to Prometheus-compatible backends.
	api.RegisterPrometheusApiEndpoints(NewForkedProm(
		api.DatasourceCache,
		NewLotexProm(proxy, logger),
		&PrometheusSrv{log: logger, manager: api.StateManager, store: api.RuleStore, ac: api.AccessControl},
	), m)
	// Register endpoints for proxying to Cortex Ruler-compatible backends.
	api.RegisterRulerApiEndpoints(NewForkedRuler(
		api.DatasourceCache,
		NewLotexRuler(proxy, logger),
		&RulerSrv{
			DatasourceCache: api.DatasourceCache,
			QuotaService:    api.QuotaService,
			scheduleService: api.Schedule,
			store:           api.RuleStore,
			provenanceStore: api.ProvenanceStore,
			xactManager:     api.TransactionManager,
			log:             logger,
			cfg:             &api.Cfg.UnifiedAlerting,
			ac:              api.AccessControl,
		},
	), m)
	api.RegisterTestingApiEndpoints(NewForkedTestingApi(
		&TestingApiSrv{
			AlertingProxy:     proxy,
			ExpressionService: api.ExpressionService,
			DatasourceCache:   api.DatasourceCache,
			log:               logger,
			accessControl:     api.AccessControl,
			evaluator:         eval.NewEvaluator(api.Cfg, log.New("ngalert.eval"), api.DatasourceCache, api.SecretsService),
		}), m)
	api.RegisterConfigurationApiEndpoints(NewForkedConfiguration(
		&AdminSrv{
			store:     api.AdminConfigStore,
			log:       logger,
			scheduler: api.Schedule,
		},
	), m)

	if api.Cfg.IsFeatureToggleEnabled(featuremgmt.FlagAlertProvisioning) {
		api.RegisterProvisioningApiEndpoints(NewForkedProvisioningApi(&ProvisioningSrv{
			log:                 logger,
			policies:            api.Policies,
			contactPointService: api.ContactPointService,
			templates:           api.Templates,
		}), m)
	}
}
