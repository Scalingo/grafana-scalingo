package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	gfcore "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/provisioning"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/services/secrets"
	secrets_fakes "github.com/grafana/grafana/pkg/services/secrets/fakes"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/web"
	prometheus "github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/timeinterval"
	"github.com/stretchr/testify/require"
)

func TestProvisioningApi(t *testing.T) {
	t.Run("policies", func(t *testing.T) {
		t.Run("successful GET returns 200", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()

			response := sut.RouteGetPolicyTree(&rc)

			require.Equal(t, 200, response.Status())
		})

		t.Run("successful PUT returns 202", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			tree := definitions.Route{}

			response := sut.RoutePutPolicyTree(&rc, tree)

			require.Equal(t, 202, response.Status())
		})

		t.Run("successful DELETE returns 202", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()

			response := sut.RouteResetPolicyTree(&rc)

			require.Equal(t, 202, response.Status())
		})

		t.Run("when new policy tree is invalid", func(t *testing.T) {
			t.Run("PUT returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				sut.policies = &fakeRejectingNotificationPolicyService{}
				rc := createTestRequestCtx()
				tree := definitions.Route{}

				response := sut.RoutePutPolicyTree(&rc, tree)

				require.Equal(t, 400, response.Status())
				expBody := `{"error":"invalid object specification: invalid policy tree","message":"invalid object specification: invalid policy tree"}`
				require.Equal(t, expBody, string(response.Body()))
			})
		})

		t.Run("when org has no AM config", func(t *testing.T) {
			t.Run("GET returns 404", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				rc.SignedInUser.OrgId = 2

				response := sut.RouteGetPolicyTree(&rc)

				require.Equal(t, 404, response.Status())
			})

			t.Run("POST returns 404", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				rc.SignedInUser.OrgId = 2

				response := sut.RouteGetPolicyTree(&rc)

				require.Equal(t, 404, response.Status())
			})
		})

		t.Run("when an unspecified error occurrs", func(t *testing.T) {
			t.Run("GET returns 500", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				sut.policies = &fakeFailingNotificationPolicyService{}
				rc := createTestRequestCtx()

				response := sut.RouteGetPolicyTree(&rc)

				require.Equal(t, 500, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "something went wrong")
			})

			t.Run("PUT returns 500", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				sut.policies = &fakeFailingNotificationPolicyService{}
				rc := createTestRequestCtx()
				tree := definitions.Route{}

				response := sut.RoutePutPolicyTree(&rc, tree)

				require.Equal(t, 500, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "something went wrong")
			})

			t.Run("DELETE returns 500", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				sut.policies = &fakeFailingNotificationPolicyService{}
				rc := createTestRequestCtx()

				response := sut.RouteResetPolicyTree(&rc)

				require.Equal(t, 500, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "something went wrong")
			})
		})
	})

	t.Run("contact points", func(t *testing.T) {
		t.Run("are invalid", func(t *testing.T) {
			t.Run("POST returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				cp := createInvalidContactPoint()

				response := sut.RoutePostContactPoint(&rc, cp)

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "recipient must be specified")
			})

			t.Run("PUT returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				cp := createInvalidContactPoint()

				response := sut.RoutePutContactPoint(&rc, cp, "email-uid")

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "recipient must be specified")
			})
		})

		t.Run("are missing, PUT returns 404", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			cp := createInvalidContactPoint()

			response := sut.RoutePutContactPoint(&rc, cp, "does not exist")

			require.Equal(t, 404, response.Status())
		})
	})

	t.Run("templates", func(t *testing.T) {
		t.Run("are invalid", func(t *testing.T) {
			t.Run("PUT returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				tmpl := definitions.MessageTemplateContent{Template: ""}

				response := sut.RoutePutTemplate(&rc, tmpl, "test")

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "template must have content")
			})
		})
	})

	t.Run("mute timings", func(t *testing.T) {
		t.Run("are invalid", func(t *testing.T) {
			t.Run("POST returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				mti := createInvalidMuteTiming()

				response := sut.RoutePostMuteTiming(&rc, mti)

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "invalid")
			})

			t.Run("PUT returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				mti := createInvalidMuteTiming()

				response := sut.RoutePutMuteTiming(&rc, mti, "interval")

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "invalid")
			})
		})

		t.Run("are missing, PUT returns 404", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			mti := definitions.MuteTimeInterval{}

			response := sut.RoutePutMuteTiming(&rc, mti, "does not exist")

			require.Equal(t, 404, response.Status())
		})
	})

	t.Run("alert rules", func(t *testing.T) {
		t.Run("are invalid", func(t *testing.T) {
			t.Run("POST returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				rule := createInvalidAlertRule()

				response := sut.RoutePostAlertRule(&rc, rule)

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "invalid alert rule")
			})

			t.Run("PUT returns 400", func(t *testing.T) {
				sut := createProvisioningSrvSut(t)
				rc := createTestRequestCtx()
				insertRule(t, sut, createTestAlertRule("rule", 1))
				rule := createInvalidAlertRule()

				response := sut.RoutePutAlertRule(&rc, rule, "rule")

				require.Equal(t, 400, response.Status())
				require.NotEmpty(t, response.Body())
				require.Contains(t, string(response.Body()), "invalid alert rule")
			})
		})

		t.Run("are missing, PUT returns 404", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			rule := createTestAlertRule("rule", 1)

			response := sut.RoutePutAlertRule(&rc, rule, "does not exist")

			require.Equal(t, 404, response.Status())
		})

		t.Run("have reached the rule quota, POST returns 403", func(t *testing.T) {
			env := createTestEnv(t)
			quotas := provisioning.MockQuotaChecker{}
			quotas.EXPECT().LimitExceeded()
			env.quotas = &quotas
			sut := createProvisioningSrvSutFromEnv(t, &env)
			rule := createTestAlertRule("rule", 1)
			rc := createTestRequestCtx()

			response := sut.RoutePostAlertRule(&rc, rule)

			require.Equal(t, 403, response.Status())
		})
	})

	t.Run("alert rule groups", func(t *testing.T) {
		t.Run("are present, GET returns 200", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			insertRule(t, sut, createTestAlertRule("rule", 1))

			response := sut.RouteGetAlertRuleGroup(&rc, "folder-uid", "my-cool-group")

			require.Equal(t, 200, response.Status())
		})

		t.Run("are missing, GET returns 404", func(t *testing.T) {
			sut := createProvisioningSrvSut(t)
			rc := createTestRequestCtx()
			insertRule(t, sut, createTestAlertRule("rule", 1))

			response := sut.RouteGetAlertRuleGroup(&rc, "folder-uid", "does not exist")

			require.Equal(t, 404, response.Status())
		})
	})
}

// testEnvironment binds together common dependencies for testing alerting APIs.
type testEnvironment struct {
	secrets secrets.Service
	log     log.Logger
	store   store.DBstore
	configs provisioning.AMConfigStore
	xact    provisioning.TransactionManager
	quotas  provisioning.QuotaChecker
	prov    provisioning.ProvisioningStore
}

func createTestEnv(t *testing.T) testEnvironment {
	t.Helper()

	secrets := secrets_fakes.NewFakeSecretsService()
	log := log.NewNopLogger()
	configs := &provisioning.MockAMConfigStore{}
	configs.EXPECT().
		GetsConfig(models.AlertConfiguration{
			AlertmanagerConfiguration: testConfig,
		})
	sqlStore := sqlstore.InitTestDB(t)
	store := store.DBstore{
		SQLStore:     sqlStore,
		BaseInterval: time.Second * 10,
	}
	quotas := &provisioning.MockQuotaChecker{}
	quotas.EXPECT().LimitOK()
	xact := &provisioning.NopTransactionManager{}
	prov := &provisioning.MockProvisioningStore{}
	prov.EXPECT().SaveSucceeds()
	prov.EXPECT().GetReturns(models.ProvenanceNone)

	return testEnvironment{
		secrets: secrets,
		log:     log,
		configs: configs,
		store:   store,
		xact:    xact,
		prov:    prov,
		quotas:  quotas,
	}
}

func createProvisioningSrvSut(t *testing.T) ProvisioningSrv {
	t.Helper()

	env := createTestEnv(t)
	return createProvisioningSrvSutFromEnv(t, &env)
}

func createProvisioningSrvSutFromEnv(t *testing.T, env *testEnvironment) ProvisioningSrv {
	t.Helper()

	return ProvisioningSrv{
		log:                 env.log,
		policies:            newFakeNotificationPolicyService(),
		contactPointService: provisioning.NewContactPointService(env.configs, env.secrets, env.prov, env.xact, env.log),
		templates:           provisioning.NewTemplateService(env.configs, env.prov, env.xact, env.log),
		muteTimings:         provisioning.NewMuteTimingService(env.configs, env.prov, env.xact, env.log),
		alertRules:          provisioning.NewAlertRuleService(env.store, env.prov, env.quotas, env.xact, 60, 10, env.log),
	}
}

func createTestRequestCtx() gfcore.ReqContext {
	return gfcore.ReqContext{
		Context: &web.Context{
			Req: &http.Request{},
		},
		SignedInUser: &gfcore.SignedInUser{
			OrgId: 1,
		},
	}
}

type fakeNotificationPolicyService struct {
	tree definitions.Route
	prov models.Provenance
}

func newFakeNotificationPolicyService() *fakeNotificationPolicyService {
	return &fakeNotificationPolicyService{
		tree: definitions.Route{
			Receiver: "some-receiver",
		},
		prov: models.ProvenanceNone,
	}
}

func (f *fakeNotificationPolicyService) GetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	if orgID != 1 {
		return definitions.Route{}, store.ErrNoAlertmanagerConfiguration
	}
	result := f.tree
	result.Provenance = f.prov
	return result, nil
}

func (f *fakeNotificationPolicyService) UpdatePolicyTree(ctx context.Context, orgID int64, tree definitions.Route, p models.Provenance) error {
	if orgID != 1 {
		return store.ErrNoAlertmanagerConfiguration
	}
	f.tree = tree
	f.prov = p
	return nil
}

func (f *fakeNotificationPolicyService) ResetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	f.tree = definitions.Route{} // TODO
	return f.tree, nil
}

type fakeFailingNotificationPolicyService struct{}

func (f *fakeFailingNotificationPolicyService) GetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	return definitions.Route{}, fmt.Errorf("something went wrong")
}

func (f *fakeFailingNotificationPolicyService) UpdatePolicyTree(ctx context.Context, orgID int64, tree definitions.Route, p models.Provenance) error {
	return fmt.Errorf("something went wrong")
}

func (f *fakeFailingNotificationPolicyService) ResetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	return definitions.Route{}, fmt.Errorf("something went wrong")
}

type fakeRejectingNotificationPolicyService struct{}

func (f *fakeRejectingNotificationPolicyService) GetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	return definitions.Route{}, nil
}

func (f *fakeRejectingNotificationPolicyService) UpdatePolicyTree(ctx context.Context, orgID int64, tree definitions.Route, p models.Provenance) error {
	return fmt.Errorf("%w: invalid policy tree", provisioning.ErrValidation)
}

func (f *fakeRejectingNotificationPolicyService) ResetPolicyTree(ctx context.Context, orgID int64) (definitions.Route, error) {
	return definitions.Route{}, nil
}

func createInvalidContactPoint() definitions.EmbeddedContactPoint {
	settings, _ := simplejson.NewJson([]byte(`{}`))
	return definitions.EmbeddedContactPoint{
		Name:     "test-contact-point",
		Type:     "slack",
		Settings: settings,
	}
}

func createInvalidMuteTiming() definitions.MuteTimeInterval {
	return definitions.MuteTimeInterval{
		MuteTimeInterval: prometheus.MuteTimeInterval{
			Name: "interval",
			TimeIntervals: []timeinterval.TimeInterval{
				{
					Weekdays: []timeinterval.WeekdayRange{
						{
							InclusiveRange: timeinterval.InclusiveRange{
								Begin: -1,
								End:   7,
							},
						},
					},
				},
			},
		},
	}
}

func createInvalidAlertRule() definitions.AlertRule {
	return definitions.AlertRule{}
}

func createTestAlertRule(title string, orgID int64) definitions.AlertRule {
	return definitions.AlertRule{
		OrgID:     orgID,
		Title:     title,
		Condition: "A",
		Data: []models.AlertQuery{
			{
				RefID: "A",
				Model: json.RawMessage("{}"),
				RelativeTimeRange: models.RelativeTimeRange{
					From: models.Duration(60),
					To:   models.Duration(0),
				},
			},
		},
		RuleGroup:    "my-cool-group",
		FolderUID:    "folder-uid",
		For:          time.Second * 60,
		NoDataState:  models.OK,
		ExecErrState: models.OkErrState,
	}
}

func insertRule(t *testing.T, srv ProvisioningSrv, rule definitions.AlertRule) {
	t.Helper()

	rc := createTestRequestCtx()
	resp := srv.RoutePostAlertRule(&rc, rule)
	require.Equal(t, 201, resp.Status())
}

var testConfig = `
{
	"template_files": {
		"a": "template"
	},
	"alertmanager_config": {
		"route": {
			"receiver": "grafana-default-email"
		},
		"receivers": [{
			"name": "grafana-default-email",
			"grafana_managed_receiver_configs": [{
				"uid": "email-uid",
				"name": "email receiver",
				"type": "email",
				"isDefault": true,
				"settings": {
					"addresses": "<example@email.com>"
				}
			}]
		}],
		"mute_time_intervals": [{
			"name": "interval",
			"time_intervals": []
		}]
	}
}
`
