package store

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
)

type EntityEventType string

const (
	EntityEventTypeDelete EntityEventType = "delete"
	EntityEventTypeCreate EntityEventType = "create"
	EntityEventTypeUpdate EntityEventType = "update"
)

type EntityType string

const (
	EntityTypeDashboard EntityType = "dashboard"
	EntityTypeFolder    EntityType = "folder"
)

// CreateDatabaseEntityId creates entityId for entities stored in the existing SQL tables
func CreateDatabaseEntityId(internalId interface{}, orgId int64, entityType EntityType) string {
	var internalIdAsString string
	switch id := internalId.(type) {
	case string:
		internalIdAsString = id
	default:
		internalIdAsString = fmt.Sprintf("%#v", internalId)
	}

	return fmt.Sprintf("database/%d/%s/%s", orgId, entityType, internalIdAsString)
}

type EntityEvent struct {
	Id        int64
	EventType EntityEventType
	EntityId  string
	Created   int64
}

type SaveEventCmd struct {
	EntityId  string
	EventType EntityEventType
}

type EventHandler func(ctx context.Context, e *EntityEvent) error

// EntityEventsService is a temporary solution to support change notifications in an HA setup
// With this service each system can query for any events that have happened since a fixed time
//go:generate mockery --name EntityEventsService --structname MockEntityEventsService --inpackage --filename entity_events_mock.go
type EntityEventsService interface {
	registry.BackgroundService
	registry.CanBeDisabled
	SaveEvent(ctx context.Context, cmd SaveEventCmd) error
	GetLastEvent(ctx context.Context) (*EntityEvent, error)
	GetAllEventsAfter(ctx context.Context, id int64) ([]*EntityEvent, error)
	OnEvent(handler EventHandler)

	deleteEventsOlderThan(ctx context.Context, duration time.Duration) error
}

func ProvideEntityEventsService(cfg *setting.Cfg, sqlStore *sqlstore.SQLStore, features featuremgmt.FeatureToggles) EntityEventsService {
	if !features.IsEnabled(featuremgmt.FlagPanelTitleSearch) {
		return &dummyEntityEventsService{}
	}

	return &entityEventService{
		sql:           sqlStore,
		features:      features,
		log:           log.New("entity-events"),
		eventHandlers: make([]EventHandler, 0),
	}
}

type entityEventService struct {
	sql           *sqlstore.SQLStore
	log           log.Logger
	features      featuremgmt.FeatureToggles
	eventHandlers []EventHandler
}

func (e *entityEventService) SaveEvent(ctx context.Context, cmd SaveEventCmd) error {
	entityEvent := &EntityEvent{
		EventType: cmd.EventType,
		EntityId:  cmd.EntityId,
		Created:   time.Now().Unix(),
	}
	err := e.sql.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		_, err := sess.Insert(entityEvent)
		return err
	})
	if err != nil {
		return err
	}
	return e.broadcastEvent(ctx, entityEvent)
}

func (e *entityEventService) broadcastEvent(ctx context.Context, event *EntityEvent) error {
	for _, h := range e.eventHandlers {
		err := h(ctx, event)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *entityEventService) OnEvent(handler EventHandler) {
	e.eventHandlers = append(e.eventHandlers, handler)
}

func (e *entityEventService) GetLastEvent(ctx context.Context) (*EntityEvent, error) {
	var entityEvent *EntityEvent
	err := e.sql.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		bean := &EntityEvent{}
		found, err := sess.OrderBy("id desc").Get(bean)
		if found {
			entityEvent = bean
		}
		return err
	})

	return entityEvent, err
}

func (e *entityEventService) GetAllEventsAfter(ctx context.Context, id int64) ([]*EntityEvent, error) {
	var evs = make([]*EntityEvent, 0)
	err := e.sql.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		return sess.OrderBy("id asc").Where("id > ?", id).Find(&evs)
	})

	return evs, err
}

func (e *entityEventService) deleteEventsOlderThan(ctx context.Context, duration time.Duration) error {
	return e.sql.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		maxCreated := time.Now().Add(-duration)
		deletedCount, err := sess.Where("created < ?", maxCreated.Unix()).Delete(&EntityEvent{})
		e.log.Info("deleting old events", "count", deletedCount, "maxCreated", maxCreated)
		return err
	})
}

func (e *entityEventService) IsDisabled() bool {
	return false
}

func (e *entityEventService) Run(ctx context.Context) error {
	clean := time.NewTicker(1 * time.Hour)

	for {
		select {
		case <-clean.C:
			go func() {
				err := e.deleteEventsOlderThan(context.Background(), 24*time.Hour)
				if err != nil {
					e.log.Info("failed to delete old entity events", "error", err)
				}
			}()
		case <-ctx.Done():
			e.log.Debug("Grafana is shutting down - stopping entity events service")
			clean.Stop()
			return nil
		}
	}
}

type dummyEntityEventsService struct {
}

func (d dummyEntityEventsService) Run(ctx context.Context) error {
	return nil
}

func (d dummyEntityEventsService) IsDisabled() bool {
	return false
}

func (d dummyEntityEventsService) SaveEvent(ctx context.Context, cmd SaveEventCmd) error {
	return nil
}

func (d dummyEntityEventsService) OnEvent(handler EventHandler) {
}

func (d dummyEntityEventsService) GetLastEvent(ctx context.Context) (*EntityEvent, error) {
	return nil, nil
}

func (d dummyEntityEventsService) GetAllEventsAfter(ctx context.Context, id int64) ([]*EntityEvent, error) {
	return make([]*EntityEvent, 0), nil
}

func (d dummyEntityEventsService) deleteEventsOlderThan(ctx context.Context, duration time.Duration) error {
	return nil
}

var _ EntityEventsService = &dummyEntityEventsService{}
