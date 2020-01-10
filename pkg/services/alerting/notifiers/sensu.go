package notifiers

import (
	"strconv"
	"strings"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
)

func init() {
	alerting.RegisterNotifier(&alerting.NotifierPlugin{
		Type:        "sensu",
		Name:        "Sensu",
		Description: "Sends HTTP POST request to a Sensu API",
		Factory:     NewSensuNotifier,
		OptionsTemplate: `
      <h3 class="page-heading">Sensu settings</h3>
      <div class="gf-form">
        <span class="gf-form-label width-10">Url</span>
				<input type="text" required class="gf-form-input max-width-26" ng-model="ctrl.model.settings.url" placeholder="http://sensu-api.local:4567/results"></input>
      </div>
      <div class="gf-form">
        <span class="gf-form-label width-10">Source</span>
        <input type="text" class="gf-form-input max-width-14" ng-model="ctrl.model.settings.source" bs-tooltip="'If empty rule id will be used'" data-placement="right"></input>
      </div>
      <div class="gf-form">
        <span class="gf-form-label width-10">Handler</span>
        <input type="text" class="gf-form-input max-width-14" ng-model="ctrl.model.settings.handler" placeholder="default"></input>
      </div>
      <div class="gf-form">
        <span class="gf-form-label width-10">Username</span>
        <input type="text" class="gf-form-input max-width-14" ng-model="ctrl.model.settings.username"></input>
      </div>
      <div class="gf-form">
        <span class="gf-form-label width-10">Password</span>
        <input type="text" class="gf-form-input max-width-14" ng-model="ctrl.model.settings.password"></input>
      </div>
    `,
	})

}

// NewSensuNotifier is the constructor for the Sensu Notifier.
func NewSensuNotifier(model *models.AlertNotification) (alerting.Notifier, error) {
	url := model.Settings.Get("url").MustString()
	if url == "" {
		return nil, alerting.ValidationError{Reason: "Could not find url property in settings"}
	}

	return &SensuNotifier{
		NotifierBase: NewNotifierBase(model),
		URL:          url,
		User:         model.Settings.Get("username").MustString(),
		Source:       model.Settings.Get("source").MustString(),
		Password:     model.Settings.Get("password").MustString(),
		Handler:      model.Settings.Get("handler").MustString(),
		log:          log.New("alerting.notifier.sensu"),
	}, nil
}

// SensuNotifier is responsible for sending
// alert notifications to Sensu.
type SensuNotifier struct {
	NotifierBase
	URL      string
	Source   string
	User     string
	Password string
	Handler  string
	log      log.Logger
}

// Notify send alert notification to Sensu
func (sn *SensuNotifier) Notify(evalContext *alerting.EvalContext) error {
	sn.log.Info("Sending sensu result")

	bodyJSON := simplejson.New()
	bodyJSON.Set("ruleId", evalContext.Rule.ID)
	// Sensu alerts cannot have spaces in them
	bodyJSON.Set("name", strings.Replace(evalContext.Rule.Name, " ", "_", -1))
	// Sensu alerts require a source. We set it to the user-specified value (optional),
	// else we fallback and use the grafana ruleID.
	if sn.Source != "" {
		bodyJSON.Set("source", sn.Source)
	} else {
		bodyJSON.Set("source", "grafana_rule_"+strconv.FormatInt(evalContext.Rule.ID, 10))
	}
	// Finally, sensu expects an output
	// We set it to a default output
	bodyJSON.Set("output", "Grafana Metric Condition Met")
	bodyJSON.Set("evalMatches", evalContext.EvalMatches)

	if evalContext.Rule.State == "alerting" {
		bodyJSON.Set("status", 2)
	} else if evalContext.Rule.State == "no_data" {
		bodyJSON.Set("status", 1)
	} else {
		bodyJSON.Set("status", 0)
	}

	if sn.Handler != "" {
		bodyJSON.Set("handler", sn.Handler)
	}

	ruleURL, err := evalContext.GetRuleURL()
	if err == nil {
		bodyJSON.Set("ruleUrl", ruleURL)
	}

	if evalContext.ImagePublicURL != "" {
		bodyJSON.Set("imageUrl", evalContext.ImagePublicURL)
	}

	if evalContext.Rule.Message != "" {
		bodyJSON.Set("output", evalContext.Rule.Message)
	}

	body, _ := bodyJSON.MarshalJSON()

	cmd := &models.SendWebhookSync{
		Url:        sn.URL,
		User:       sn.User,
		Password:   sn.Password,
		Body:       string(body),
		HttpMethod: "POST",
	}

	if err := bus.DispatchCtx(evalContext.Ctx, cmd); err != nil {
		sn.log.Error("Failed to send sensu event", "error", err, "sensu", sn.Name)
		return err
	}

	return nil
}
