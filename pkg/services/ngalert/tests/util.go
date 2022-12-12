package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/appcontext"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	acmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/annotations/annotationstest"
	"github.com/grafana/grafana/pkg/services/dashboards"
	databasestore "github.com/grafana/grafana/pkg/services/dashboards/database"
	dashboardservice "github.com/grafana/grafana/pkg/services/dashboards/service"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/folder"
	"github.com/grafana/grafana/pkg/services/folder/folderimpl"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/ngalert"
	"github.com/grafana/grafana/pkg/services/ngalert/metrics"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/quota/quotatest"
	"github.com/grafana/grafana/pkg/services/secrets/database"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
	"github.com/grafana/grafana/pkg/services/tag/tagimpl"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

type FakeFeatures struct {
	BigTransactions bool
}

func (f *FakeFeatures) IsEnabled(feature string) bool {
	if feature == featuremgmt.FlagAlertingBigTransactions {
		return f.BigTransactions
	}

	return false
}

// SetupTestEnv initializes a store to used by the tests.
func SetupTestEnv(tb testing.TB, baseInterval time.Duration) (*ngalert.AlertNG, *store.DBstore) {
	tb.Helper()
	origNewGuardian := guardian.New
	guardian.MockDashboardGuardian(&guardian.FakeDashboardGuardian{
		CanSaveValue:  true,
		CanViewValue:  true,
		CanAdminValue: true,
	})
	tb.Cleanup(func() {
		guardian.New = origNewGuardian
	})

	cfg := setting.NewCfg()
	cfg.IsFeatureToggleEnabled = featuremgmt.WithFeatures().IsEnabled
	cfg.UnifiedAlerting = setting.UnifiedAlertingSettings{
		BaseInterval: setting.SchedulerBaseInterval,
	}
	// AlertNG database migrations run and the relative database tables are created only when it's enabled
	cfg.UnifiedAlerting.Enabled = new(bool)
	*cfg.UnifiedAlerting.Enabled = true

	m := metrics.NewNGAlert(prometheus.NewRegistry())
	sqlStore := db.InitTestDB(tb)
	secretsService := secretsManager.SetupTestService(tb, database.ProvideSecretsStore(sqlStore))
	quotaService := quotatest.New(false, nil)
	dashboardStore, err := databasestore.ProvideDashboardStore(sqlStore, sqlStore.Cfg, featuremgmt.WithFeatures(), tagimpl.ProvideService(sqlStore, sqlStore.Cfg), quotaService)
	require.NoError(tb, err)

	ac := acmock.New()
	features := featuremgmt.WithFeatures()
	folderPermissions := acmock.NewMockedPermissionsService()
	folderPermissions.On("SetPermissions", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]accesscontrol.ResourcePermission{}, nil)
	dashboardPermissions := acmock.NewMockedPermissionsService()

	dashboardService := dashboardservice.ProvideDashboardService(
		cfg, dashboardStore, nil,
		features, folderPermissions, dashboardPermissions, ac,
	)

	bus := bus.ProvideBus(tracing.InitializeTracerForTest())
	folderService := folderimpl.ProvideService(ac, bus, cfg, dashboardService, dashboardStore, nil, features, folderPermissions, nil)

	ng, err := ngalert.ProvideService(
		cfg, &FakeFeatures{}, nil, nil, routing.NewRouteRegister(), sqlStore, nil, nil, nil, quotatest.New(false, nil),
		secretsService, nil, m, folderService, ac, &dashboards.FakeDashboardService{}, nil, bus, ac, annotationstest.NewFakeAnnotationsRepo(),
	)
	require.NoError(tb, err)
	return ng, &store.DBstore{
		FeatureToggles: ng.FeatureToggles,
		SQLStore:       ng.SQLStore,
		Cfg: setting.UnifiedAlertingSettings{
			BaseInterval: baseInterval * time.Second,
		},
		Logger:           log.New("ngalert-test"),
		DashboardService: dashboardService,
		FolderService:    folderService,
	}
}

// CreateTestAlertRule creates a dummy alert definition to be used by the tests.
func CreateTestAlertRule(t testing.TB, ctx context.Context, dbstore *store.DBstore, intervalSeconds int64, orgID int64) *models.AlertRule {
	return CreateTestAlertRuleWithLabels(t, ctx, dbstore, intervalSeconds, orgID, nil)
}

func CreateTestAlertRuleWithLabels(t testing.TB, ctx context.Context, dbstore *store.DBstore, intervalSeconds int64, orgID int64, labels map[string]string) *models.AlertRule {
	ruleGroup := fmt.Sprintf("ruleGroup-%s", util.GenerateShortUID())
	folderUID := "namespace"
	user := &user.SignedInUser{
		UserID:         1,
		OrgID:          orgID,
		OrgRole:        org.RoleAdmin,
		IsGrafanaAdmin: true,
	}

	ctx = appcontext.WithUser(ctx, user)
	_, err := dbstore.FolderService.Create(ctx, &folder.CreateFolderCommand{OrgID: orgID, Title: "FOLDER-" + util.GenerateShortUID(), UID: folderUID})
	// var foldr *folder.Folder
	if errors.Is(err, dashboards.ErrFolderWithSameUIDExists) || errors.Is(err, dashboards.ErrFolderVersionMismatch) {
		_, err = dbstore.FolderService.Get(ctx, &folder.GetFolderQuery{OrgID: orgID, UID: &folderUID})
	}
	require.NoError(t, err)

	_, err = dbstore.InsertAlertRules(ctx, []models.AlertRule{
		{

			ID:        0,
			OrgID:     orgID,
			Title:     fmt.Sprintf("an alert definition %s", util.GenerateShortUID()),
			Condition: "A",
			Data: []models.AlertQuery{
				{
					Model: json.RawMessage(`{
										"datasourceUid": "-100",
										"type":"math",
										"expression":"2 + 2 > 1"
									}`),
					RelativeTimeRange: models.RelativeTimeRange{
						From: models.Duration(5 * time.Hour),
						To:   models.Duration(3 * time.Hour),
					},
					RefID: "A",
				},
			},
			Labels:          labels,
			Annotations:     map[string]string{"testAnnoKey": "testAnnoValue"},
			IntervalSeconds: intervalSeconds,
			NamespaceUID:    folderUID,
			RuleGroup:       ruleGroup,
			NoDataState:     models.NoData,
			ExecErrState:    models.AlertingErrState,
		},
	})
	require.NoError(t, err)

	q := models.ListAlertRulesQuery{
		OrgID:         orgID,
		NamespaceUIDs: []string{folderUID},
		RuleGroup:     ruleGroup,
	}
	err = dbstore.ListAlertRules(ctx, &q)
	require.NoError(t, err)
	require.NotEmpty(t, q.Result)

	rule := q.Result[0]
	t.Logf("alert definition: %v with title: %q interval: %d folder: %s created", rule.GetKey(), rule.Title, rule.IntervalSeconds, folderUID)
	return rule
}
