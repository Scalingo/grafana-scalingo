package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/serviceaccounts"
	"github.com/grafana/grafana/pkg/services/serviceaccounts/api"
	"github.com/grafana/grafana/pkg/services/serviceaccounts/secretscan"
	"github.com/grafana/grafana/pkg/setting"
)

const (
	metricsCollectionInterval = time.Minute * 30
	defaultSecretScanInterval = time.Minute * 5
)

type ServiceAccountsService struct {
	store             serviceaccounts.Store
	log               log.Logger
	backgroundLog     log.Logger
	secretScanService secretscan.Checker

	secretScanEnabled  bool
	secretScanInterval time.Duration
}

func ProvideServiceAccountsService(
	cfg *setting.Cfg,
	ac accesscontrol.AccessControl,
	routeRegister routing.RouteRegister,
	usageStats usagestats.Service,
	serviceAccountsStore serviceaccounts.Store,
	permissionService accesscontrol.ServiceAccountPermissionsService,
	accesscontrolService accesscontrol.Service,
) (*ServiceAccountsService, error) {
	s := &ServiceAccountsService{
		store:         serviceAccountsStore,
		log:           log.New("serviceaccounts"),
		backgroundLog: log.New("serviceaccounts.background"),
	}

	if err := RegisterRoles(accesscontrolService); err != nil {
		s.log.Error("Failed to register roles", "error", err)
	}

	usageStats.RegisterMetricsFunc(s.getUsageMetrics)

	serviceaccountsAPI := api.NewServiceAccountsAPI(cfg, s, ac, accesscontrolService, routeRegister, s.store, permissionService)
	serviceaccountsAPI.RegisterAPIEndpoints()

	s.secretScanEnabled = cfg.SectionWithEnvOverrides("secretscan").Key("enabled").MustBool(false)
	s.secretScanInterval = cfg.SectionWithEnvOverrides("secretscan").
		Key("interval").MustDuration(defaultSecretScanInterval)
	if s.secretScanEnabled {
		s.secretScanService = secretscan.NewService(s.store, cfg)
	}

	return s, nil
}

func (sa *ServiceAccountsService) Run(ctx context.Context) error {
	sa.backgroundLog.Debug("service initialized")

	if _, err := sa.getUsageMetrics(ctx); err != nil {
		sa.log.Warn("Failed to get usage metrics", "error", err.Error())
	}

	updateStatsTicker := time.NewTicker(metricsCollectionInterval)
	defer updateStatsTicker.Stop()

	// Enforce a minimum interval of 1 minute.
	if sa.secretScanEnabled && sa.secretScanInterval < time.Minute {
		sa.backgroundLog.Warn("secret scan interval is too low, increasing to " +
			defaultSecretScanInterval.String())

		sa.secretScanInterval = defaultSecretScanInterval
	}

	tokenCheckTicker := time.NewTicker(sa.secretScanInterval)

	if !sa.secretScanEnabled {
		tokenCheckTicker.Stop()
	} else {
		sa.backgroundLog.Debug("enabled token secret check and executing first check")
		if err := sa.secretScanService.CheckTokens(ctx); err != nil {
			sa.backgroundLog.Warn("Failed to check for leaked tokens", "error", err.Error())
		}

		defer tokenCheckTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context error in service account background service: %w", ctx.Err())
			}

			sa.backgroundLog.Debug("stopped service account background service")

			return nil
		case <-updateStatsTicker.C:
			sa.backgroundLog.Debug("updating usage metrics")

			if _, err := sa.getUsageMetrics(ctx); err != nil {
				sa.backgroundLog.Warn("Failed to get usage metrics", "error", err.Error())
			}
		case <-tokenCheckTicker.C:
			sa.backgroundLog.Debug("checking for leaked tokens")

			if err := sa.secretScanService.CheckTokens(ctx); err != nil {
				sa.backgroundLog.Warn("Failed to check for leaked tokens", "error", err.Error())
			}
		}
	}
}

func (sa *ServiceAccountsService) CreateServiceAccount(ctx context.Context, orgID int64, saForm *serviceaccounts.CreateServiceAccountForm) (*serviceaccounts.ServiceAccountDTO, error) {
	return sa.store.CreateServiceAccount(ctx, orgID, saForm)
}

func (sa *ServiceAccountsService) DeleteServiceAccount(ctx context.Context, orgID, serviceAccountID int64) error {
	return sa.store.DeleteServiceAccount(ctx, orgID, serviceAccountID)
}

func (sa *ServiceAccountsService) RetrieveServiceAccountIdByName(ctx context.Context, orgID int64, name string) (int64, error) {
	return sa.store.RetrieveServiceAccountIdByName(ctx, orgID, name)
}
