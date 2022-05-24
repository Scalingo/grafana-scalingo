package bus

import (
	"context"
	"testing"

	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/stretchr/testify/require"
)

type testQuery struct {
	ID   int64
	Resp string
}

func TestEventPublish(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	var invoked bool

	bus.AddEventListener(func(ctx context.Context, query *testQuery) error {
		invoked = true
		return nil
	})

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")

	require.True(t, invoked)
}

func TestEventPublish_NoRegisteredListener(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")
}

func TestEventCtxPublishCtx(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	var invoked bool

	bus.AddEventListener(func(ctx context.Context, query *testQuery) error {
		invoked = true
		return nil
	})

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")

	require.True(t, invoked)
}

func TestEventPublishCtx_NoRegisteredListener(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")
}

func TestEventPublishCtx(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	var invoked bool

	bus.AddEventListener(func(ctx context.Context, query *testQuery) error {
		invoked = true
		return nil
	})

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")

	require.True(t, invoked)
}

func TestEventCtxPublish(t *testing.T) {
	bus := New()
	tracer, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)
	bus.tracer = tracer

	var invoked bool

	bus.AddEventListener(func(ctx context.Context, query *testQuery) error {
		invoked = true
		return nil
	})

	err = bus.Publish(context.Background(), &testQuery{})
	require.NoError(t, err, "unable to publish event")

	require.True(t, invoked)
}
