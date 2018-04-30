package alerting

import (
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/imguploader"
	"github.com/grafana/grafana/pkg/components/renderer"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/metrics"

	m "github.com/grafana/grafana/pkg/models"
)

type NotifierPlugin struct {
	Type            string          `json:"type"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	OptionsTemplate string          `json:"optionsTemplate"`
	Factory         NotifierFactory `json:"-"`
}

type NotificationService interface {
	SendIfNeeded(context *EvalContext) error
}

func NewNotificationService() NotificationService {
	return newNotificationService()
}

type notificationService struct {
	log log.Logger
}

func newNotificationService() *notificationService {
	return &notificationService{
		log: log.New("alerting.notifier"),
	}
}

func (n *notificationService) SendIfNeeded(context *EvalContext) error {
	notifiers, err := n.getNeededNotifiers(context.Rule.OrgId, context.Rule.Notifications, context)
	if err != nil {
		return err
	}

	if len(notifiers) == 0 {
		return nil
	}

	if notifiers.ShouldUploadImage() {
		if err = n.uploadImage(context); err != nil {
			n.log.Error("Failed to upload alert panel image.", "error", err)
		}
	}

	return n.sendNotifications(context, notifiers)
}

func (n *notificationService) sendNotifications(context *EvalContext, notifiers []Notifier) error {
	g, _ := errgroup.WithContext(context.Ctx)

	for _, notifier := range notifiers {
		not := notifier //avoid updating scope variable in go routine
		n.log.Debug("Sending notification", "type", not.GetType(), "id", not.GetNotifierId(), "isDefault", not.GetIsDefault())
		metrics.M_Alerting_Notification_Sent.WithLabelValues(not.GetType()).Inc()
		g.Go(func() error { return not.Notify(context) })
	}

	return g.Wait()
}

func (n *notificationService) uploadImage(context *EvalContext) (err error) {
	uploader, err := imguploader.NewImageUploader()
	if err != nil {
		return err
	}

	renderOpts := &renderer.RenderOpts{
		Width:          "800",
		Height:         "400",
		Timeout:        "30",
		OrgId:          context.Rule.OrgId,
		IsAlertContext: true,
	}

	ref, err := context.GetDashboardUID()
	if err != nil {
		return err
	}
	renderOpts.Path = fmt.Sprintf("d-solo/%s/%s?panelId=%d", ref.Uid, ref.Slug, context.Rule.PanelId)

	imagePath, err := renderer.RenderToPng(renderOpts)
	if err != nil {
		return err
	}
	context.ImageOnDiskPath = imagePath

	context.ImagePublicUrl, err = uploader.Upload(context.Ctx, context.ImageOnDiskPath)
	if err != nil {
		return err
	}

	n.log.Info("uploaded", "url", context.ImagePublicUrl)
	return nil
}

func (n *notificationService) getNeededNotifiers(orgId int64, notificationIds []int64, context *EvalContext) (NotifierSlice, error) {
	query := &m.GetAlertNotificationsToSendQuery{OrgId: orgId, Ids: notificationIds}

	if err := bus.Dispatch(query); err != nil {
		return nil, err
	}

	var result []Notifier
	for _, notification := range query.Result {
		not, err := n.createNotifierFor(notification)
		if err != nil {
			return nil, err
		}
		if not.ShouldNotify(context) {
			result = append(result, not)
		}
	}

	return result, nil
}

func (n *notificationService) createNotifierFor(model *m.AlertNotification) (Notifier, error) {
	notifierPlugin, found := notifierFactories[model.Type]
	if !found {
		return nil, errors.New("Unsupported notification type")
	}

	return notifierPlugin.Factory(model)
}

type NotifierFactory func(notification *m.AlertNotification) (Notifier, error)

var notifierFactories = make(map[string]*NotifierPlugin)

func RegisterNotifier(plugin *NotifierPlugin) {
	notifierFactories[plugin.Type] = plugin
}

func GetNotifiers() []*NotifierPlugin {
	list := make([]*NotifierPlugin, 0)

	for _, value := range notifierFactories {
		list = append(list, value)
	}

	return list
}
