package pipeline

import (
	"context"

	"github.com/grafana/grafana/pkg/services/live/livecontext"
	"github.com/grafana/grafana/pkg/services/live/managedstream"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana/pkg/models"
)

type ManagedStreamSubscriber struct {
	managedStream *managedstream.Runner
}

const SubscriberTypeManagedStream = "managedStream"

func NewManagedStreamSubscriber(managedStream *managedstream.Runner) *ManagedStreamSubscriber {
	return &ManagedStreamSubscriber{managedStream: managedStream}
}

func (s *ManagedStreamSubscriber) Type() string {
	return SubscriberTypeManagedStream
}

func (s *ManagedStreamSubscriber) Subscribe(ctx context.Context, vars Vars) (models.SubscribeReply, backend.SubscribeStreamStatus, error) {
	stream, err := s.managedStream.GetOrCreateStream(vars.OrgID, vars.Scope, vars.Namespace)
	if err != nil {
		logger.Error("Error getting managed stream", "error", err)
		return models.SubscribeReply{}, 0, err
	}
	u, ok := livecontext.GetContextSignedUser(ctx)
	if !ok {
		return models.SubscribeReply{}, backend.SubscribeStreamStatusPermissionDenied, nil
	}
	return stream.OnSubscribe(ctx, u, models.SubscribeEvent{
		Channel: vars.Channel,
		Path:    vars.Path,
	})
}
