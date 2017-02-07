package notifiers

import (
	"os"
	"strings"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/metrics"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/setting"
)

func init() {
	alerting.RegisterNotifier("email", NewEmailNotifier)
}

type EmailNotifier struct {
	NotifierBase
	Addresses []string
	log       log.Logger
}

func NewEmailNotifier(model *m.AlertNotification) (alerting.Notifier, error) {
	addressesString := model.Settings.Get("addresses").MustString()

	if addressesString == "" {
		return nil, alerting.ValidationError{Reason: "Could not find addresses in settings"}
	}

	// split addresses with a few different ways
	addresses := strings.FieldsFunc(addressesString, func(r rune) bool {
		switch r {
		case ',', ';', '\n':
			return true
		}
		return false
	})

	return &EmailNotifier{
		NotifierBase: NewNotifierBase(model.Id, model.IsDefault, model.Name, model.Type, model.Settings),
		Addresses:    addresses,
		log:          log.New("alerting.notifier.email"),
	}, nil
}

func (this *EmailNotifier) Notify(evalContext *alerting.EvalContext) error {
	this.log.Info("Sending alert notification to", "addresses", this.Addresses)
	metrics.M_Alerting_Notification_Sent_Email.Inc(1)

	ruleUrl, err := evalContext.GetRuleUrl()
	if err != nil {
		this.log.Error("Failed get rule link", "error", err)
		return err
	}

	cmd := &m.SendEmailCommandSync{
		SendEmailCommand: m.SendEmailCommand{
			Subject: evalContext.GetNotificationTitle(),
			Data: map[string]interface{}{
				"Title":        evalContext.GetNotificationTitle(),
				"State":        evalContext.Rule.State,
				"Name":         evalContext.Rule.Name,
				"StateModel":   evalContext.GetStateModel(),
				"Message":      evalContext.Rule.Message,
				"RuleUrl":      ruleUrl,
				"ImageLink":    "",
				"EmbededImage": "",
				"AlertPageUrl": setting.AppUrl + "alerting",
				"EvalMatches":  evalContext.EvalMatches,
			},
			To:           this.Addresses,
			Template:     "alert_notification.html",
			EmbededFiles: []string{},
		},
	}

	if evalContext.ImagePublicUrl != "" {
		cmd.Data["ImageLink"] = evalContext.ImagePublicUrl
	} else {
		file, err := os.Stat(evalContext.ImageOnDiskPath)
		if err == nil {
			cmd.EmbededFiles = []string{evalContext.ImageOnDiskPath}
			cmd.Data["EmbededImage"] = file.Name()
		}
	}

	err = bus.DispatchCtx(evalContext.Ctx, cmd)

	if err != nil {
		this.log.Error("Failed to send alert notification email", "error", err)
		return err
	}
	return nil

}
