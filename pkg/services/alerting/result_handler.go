package alerting

import (
	"time"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/metrics"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/annotations"
)

type ResultHandler interface {
	Handle(evalContext *EvalContext) error
}

type DefaultResultHandler struct {
	notifier NotificationService
	log      log.Logger
}

func NewResultHandler() *DefaultResultHandler {
	return &DefaultResultHandler{
		log:      log.New("alerting.resultHandler"),
		notifier: NewNotificationService(),
	}
}

func (handler *DefaultResultHandler) Handle(evalContext *EvalContext) error {
	executionError := ""
	annotationData := simplejson.New()

	if len(evalContext.EvalMatches) > 0 {
		annotationData.Set("evalMatches", simplejson.NewFromAny(evalContext.EvalMatches))
	}

	if evalContext.Error != nil {
		executionError = evalContext.Error.Error()
		annotationData.Set("error", executionError)
	} else if evalContext.NoDataFound {
		annotationData.Set("noData", true)
	}

	metrics.M_Alerting_Result_State.WithLabelValues(string(evalContext.Rule.State)).Inc()
	if evalContext.ShouldUpdateAlertState() {
		handler.log.Info("New state change", "alertId", evalContext.Rule.Id, "newState", evalContext.Rule.State, "prev state", evalContext.PrevAlertState)

		cmd := &m.SetAlertStateCommand{
			AlertId:  evalContext.Rule.Id,
			OrgId:    evalContext.Rule.OrgId,
			State:    evalContext.Rule.State,
			Error:    executionError,
			EvalData: annotationData,
		}

		if err := bus.Dispatch(cmd); err != nil {
			if err == m.ErrCannotChangeStateOnPausedAlert {
				handler.log.Error("Cannot change state on alert thats pause", "error", err)
				return err
			}

			if err == m.ErrRequiresNewState {
				handler.log.Info("Alert already updated")
				return nil
			}

			handler.log.Error("Failed to save state", "error", err)
		}

		// save annotation
		item := annotations.Item{
			OrgId:       evalContext.Rule.OrgId,
			DashboardId: evalContext.Rule.DashboardId,
			PanelId:     evalContext.Rule.PanelId,
			Type:        annotations.AlertType,
			AlertId:     evalContext.Rule.Id,
			Title:       evalContext.Rule.Name,
			Text:        evalContext.GetStateModel().Text,
			NewState:    string(evalContext.Rule.State),
			PrevState:   string(evalContext.PrevAlertState),
			Epoch:       time.Now().Unix(),
			Data:        annotationData,
		}

		annotationRepo := annotations.GetRepository()
		if err := annotationRepo.Save(&item); err != nil {
			handler.log.Error("Failed to save annotation for new alert state", "error", err)
		}

		if evalContext.ShouldSendNotification() {
			handler.notifier.Send(evalContext)
		}
	}

	return nil
}
