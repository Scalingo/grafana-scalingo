package alerting

import (
	"context"
	"fmt"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/null"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/log"
	m "github.com/grafana/grafana/pkg/models"
)

type NotificationTestCommand struct {
	State    m.AlertStateType
	Name     string
	Type     string
	Settings *simplejson.Json
}

func init() {
	bus.AddHandler("alerting", handleNotificationTestCommand)

}

func handleNotificationTestCommand(cmd *NotificationTestCommand) error {
	notifier := newNotificationService()

	model := &m.AlertNotification{
		Name:     cmd.Name,
		Type:     cmd.Type,
		Settings: cmd.Settings,
	}

	notifiers, err := notifier.createNotifierFor(model)

	if err != nil {
		log.Error2("Failed to create notifier", "error", err.Error())
		return err
	}

	return notifier.sendNotifications(createTestEvalContext(cmd), []Notifier{notifiers})
}

func createTestEvalContext(cmd *NotificationTestCommand) *EvalContext {
	testRule := &Rule{
		DashboardId: 1,
		PanelId:     1,
		Name:        "Test notification",
		Message:     "Someone is testing the alert notification within grafana.",
		State:       m.AlertStateAlerting,
	}

	ctx := NewEvalContext(context.Background(), testRule)
	if cmd.Settings.Get("uploadImage").MustBool(true) {
		ctx.ImagePublicUrl = "http://grafana.org/assets/img/blog/mixed_styles.png"
	}
	ctx.IsTestRun = true
	ctx.Firing = true
	ctx.Error = fmt.Errorf("This is only a test")
	ctx.EvalMatches = evalMatchesBasedOnState()

	return ctx
}

func evalMatchesBasedOnState() []*EvalMatch {
	matches := make([]*EvalMatch, 0)
	matches = append(matches, &EvalMatch{
		Metric: "High value",
		Value:  null.FloatFrom(100),
	})

	matches = append(matches, &EvalMatch{
		Metric: "Higher Value",
		Value:  null.FloatFrom(200),
	})

	return matches
}
