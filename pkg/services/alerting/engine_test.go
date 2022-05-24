package alerting

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/encryption/ossencryption"
	"github.com/grafana/grafana/pkg/setting"

	"github.com/stretchr/testify/require"
)

type FakeEvalHandler struct {
	SuccessCallID int // 0 means never success
	CallNb        int
}

func NewFakeEvalHandler(successCallID int) *FakeEvalHandler {
	return &FakeEvalHandler{
		SuccessCallID: successCallID,
		CallNb:        0,
	}
}

func (handler *FakeEvalHandler) Eval(evalContext *EvalContext) {
	handler.CallNb++
	if handler.CallNb != handler.SuccessCallID {
		evalContext.Error = errors.New("Fake evaluation failure")
	}
}

type FakeResultHandler struct{}

func (handler *FakeResultHandler) handle(evalContext *EvalContext) error {
	return nil
}

// A mock implementation of the AlertStore interface, allowing to override certain methods individually
type AlertStoreMock struct {
	getAllAlerts                       func(context.Context, *models.GetAllAlertsQuery) error
	getDataSource                      func(context.Context, *models.GetDataSourceQuery) error
	getAlertNotificationsWithUidToSend func(ctx context.Context, query *models.GetAlertNotificationsWithUidToSendQuery) error
	getOrCreateNotificationState       func(ctx context.Context, query *models.GetOrCreateNotificationStateQuery) error
}

func (a *AlertStoreMock) GetDataSource(c context.Context, cmd *models.GetDataSourceQuery) error {
	if a.getDataSource != nil {
		return a.getDataSource(c, cmd)
	}
	return nil
}

func (a *AlertStoreMock) GetAllAlertQueryHandler(c context.Context, cmd *models.GetAllAlertsQuery) error {
	if a.getAllAlerts != nil {
		return a.getAllAlerts(c, cmd)
	}
	return nil
}

func (a *AlertStoreMock) GetAlertNotificationUidWithId(c context.Context, query *models.GetAlertNotificationUidQuery) error {
	return nil
}

func (a *AlertStoreMock) GetAlertNotificationsWithUidToSend(c context.Context, cmd *models.GetAlertNotificationsWithUidToSendQuery) error {
	if a.getAlertNotificationsWithUidToSend != nil {
		return a.getAlertNotificationsWithUidToSend(c, cmd)
	}
	return nil
}

func (a *AlertStoreMock) GetOrCreateAlertNotificationState(c context.Context, cmd *models.GetOrCreateNotificationStateQuery) error {
	if a.getOrCreateNotificationState != nil {
		return a.getOrCreateNotificationState(c, cmd)
	}
	return nil
}

func (a *AlertStoreMock) GetDashboardUIDById(_ context.Context, _ *models.GetDashboardRefByIdQuery) error {
	return nil
}

func (a *AlertStoreMock) SetAlertNotificationStateToCompleteCommand(_ context.Context, _ *models.SetAlertNotificationStateToCompleteCommand) error {
	return nil
}

func (a *AlertStoreMock) SetAlertNotificationStateToPendingCommand(_ context.Context, _ *models.SetAlertNotificationStateToPendingCommand) error {
	return nil
}

func (a *AlertStoreMock) SetAlertState(_ context.Context, _ *models.SetAlertStateCommand) error {
	return nil
}

func TestEngineProcessJob(t *testing.T) {
	usMock := &usagestats.UsageStatsMock{T: t}
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)

	store := &AlertStoreMock{}
	engine := ProvideAlertEngine(nil, nil, nil, usMock, ossencryption.ProvideService(), nil, tracer, store, setting.NewCfg(), nil)
	setting.AlertingEvaluationTimeout = 30 * time.Second
	setting.AlertingNotificationTimeout = 30 * time.Second
	setting.AlertingMaxAttempts = 3
	engine.resultHandler = &FakeResultHandler{}
	job := &Job{running: true, Rule: &Rule{}}

	t.Run("Should register usage metrics func", func(t *testing.T) {
		store.getAllAlerts = func(ctx context.Context, q *models.GetAllAlertsQuery) error {
			settings, err := simplejson.NewJson([]byte(`{"conditions": [{"query": { "datasourceId": 1}}]}`))
			if err != nil {
				return err
			}
			q.Result = []*models.Alert{{Settings: settings}}
			return nil
		}

		store.getDataSource = func(ctx context.Context, q *models.GetDataSourceQuery) error {
			q.Result = &models.DataSource{Id: 1, Type: models.DS_PROMETHEUS}
			return nil
		}

		report, err := usMock.GetUsageReport(context.Background())
		require.Nil(t, err)

		require.Equal(t, 1, report.Metrics["stats.alerting.ds.prometheus.count"])
		require.Equal(t, 0, report.Metrics["stats.alerting.ds.other.count"])
	})

	t.Run("Should trigger retry if needed", func(t *testing.T) {
		t.Run("error + not last attempt -> retry", func(t *testing.T) {
			engine.evalHandler = NewFakeEvalHandler(0)

			for i := 1; i < setting.AlertingMaxAttempts; i++ {
				attemptChan := make(chan int, 1)
				cancelChan := make(chan context.CancelFunc, setting.AlertingMaxAttempts)

				engine.processJob(i, attemptChan, cancelChan, job)
				nextAttemptID, more := <-attemptChan

				require.Equal(t, i+1, nextAttemptID)
				require.Equal(t, true, more)
				require.NotNil(t, <-cancelChan)
			}
		})

		t.Run("error + last attempt -> no retry", func(t *testing.T) {
			engine.evalHandler = NewFakeEvalHandler(0)
			attemptChan := make(chan int, 1)
			cancelChan := make(chan context.CancelFunc, setting.AlertingMaxAttempts)

			engine.processJob(setting.AlertingMaxAttempts, attemptChan, cancelChan, job)
			nextAttemptID, more := <-attemptChan

			require.Equal(t, 0, nextAttemptID)
			require.Equal(t, false, more)
			require.NotNil(t, <-cancelChan)
		})

		t.Run("no error -> no retry", func(t *testing.T) {
			engine.evalHandler = NewFakeEvalHandler(1)
			attemptChan := make(chan int, 1)
			cancelChan := make(chan context.CancelFunc, setting.AlertingMaxAttempts)

			engine.processJob(1, attemptChan, cancelChan, job)
			nextAttemptID, more := <-attemptChan

			require.Equal(t, 0, nextAttemptID)
			require.Equal(t, false, more)
			require.NotNil(t, <-cancelChan)
		})
	})

	t.Run("Should trigger as many retries as needed", func(t *testing.T) {
		t.Run("never success -> max retries number", func(t *testing.T) {
			expectedAttempts := setting.AlertingMaxAttempts
			evalHandler := NewFakeEvalHandler(0)
			engine.evalHandler = evalHandler

			err := engine.processJobWithRetry(context.Background(), job)
			require.Nil(t, err)
			require.Equal(t, expectedAttempts, evalHandler.CallNb)
		})

		t.Run("always success -> never retry", func(t *testing.T) {
			expectedAttempts := 1
			evalHandler := NewFakeEvalHandler(1)
			engine.evalHandler = evalHandler

			err := engine.processJobWithRetry(context.Background(), job)
			require.Nil(t, err)
			require.Equal(t, expectedAttempts, evalHandler.CallNb)
		})

		t.Run("some errors before success -> some retries", func(t *testing.T) {
			expectedAttempts := int(math.Ceil(float64(setting.AlertingMaxAttempts) / 2))
			evalHandler := NewFakeEvalHandler(expectedAttempts)
			engine.evalHandler = evalHandler

			err := engine.processJobWithRetry(context.Background(), job)
			require.Nil(t, err)
			require.Equal(t, expectedAttempts, evalHandler.CallNb)
		})
	})
}
