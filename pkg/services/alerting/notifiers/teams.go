package notifiers

import (
	"encoding/json"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/log"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
)

func init() {
	alerting.RegisterNotifier(&alerting.NotifierPlugin{
		Type:        "teams",
		Name:        "Microsoft Teams",
		Description: "Sends notifications using Incoming Webhook connector to Microsoft Teams",
		Factory:     NewTeamsNotifier,
		OptionsTemplate: `
      <h3 class="page-heading">Teams settings</h3>
      <div class="gf-form max-width-30">
        <span class="gf-form-label width-6">Url</span>
        <input type="text" required class="gf-form-input max-width-30" ng-model="ctrl.model.settings.url" placeholder="Teams incoming webhook url"></input>
      </div>
    `,
	})

}

func NewTeamsNotifier(model *m.AlertNotification) (alerting.Notifier, error) {
	url := model.Settings.Get("url").MustString()
	if url == "" {
		return nil, alerting.ValidationError{Reason: "Could not find url property in settings"}
	}

	return &TeamsNotifier{
		NotifierBase: NewNotifierBase(model.Id, model.IsDefault, model.Name, model.Type, model.Settings),
		Url:          url,
		log:          log.New("alerting.notifier.teams"),
	}, nil
}

type TeamsNotifier struct {
	NotifierBase
	Url       string
	Recipient string
	Mention   string
	log       log.Logger
}

func (this *TeamsNotifier) Notify(evalContext *alerting.EvalContext) error {
	this.log.Info("Executing teams notification", "ruleId", evalContext.Rule.Id, "notification", this.Name)

	ruleUrl, err := evalContext.GetRuleUrl()
	if err != nil {
		this.log.Error("Failed get rule link", "error", err)
		return err
	}

	fields := make([]map[string]interface{}, 0)
	fieldLimitCount := 4
	for index, evt := range evalContext.EvalMatches {
		fields = append(fields, map[string]interface{}{
			"name":  evt.Metric,
			"value": evt.Value,
		})
		if index > fieldLimitCount {
			break
		}
	}

	if evalContext.Error != nil {
		fields = append(fields, map[string]interface{}{
			"name":  "Error message",
			"value": evalContext.Error.Error(),
		})
	}

	message := this.Mention
	if evalContext.Rule.State != m.AlertStateOK { //don't add message when going back to alert state ok.
		message += " " + evalContext.Rule.Message
	} else {
		message += " " // summary must not be empty
	}

	body := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    message,
		"title":      evalContext.GetNotificationTitle(),
		"themeColor": evalContext.GetStateModel().Color,
		"sections": []map[string]interface{}{
			{
				"title": "Details",
				"facts": fields,
				"images": []map[string]interface{}{
					{
						"image": evalContext.ImagePublicUrl,
					},
				},
				"text": message,
				"potentialAction": []map[string]interface{}{
					{
						"@context": "http://schema.org",
						"@type":    "ViewAction",
						"name":     "View Rule",
						"target": []string{
							ruleUrl,
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(&body)
	cmd := &m.SendWebhookSync{Url: this.Url, Body: string(data)}

	if err := bus.DispatchCtx(evalContext.Ctx, cmd); err != nil {
		this.log.Error("Failed to send teams notification", "error", err, "webhook", this.Name)
		return err
	}

	return nil
}
