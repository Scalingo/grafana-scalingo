package schedule

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/services/ngalert/models"
)

func TestSchedule_alertRuleInfo(t *testing.T) {
	t.Run("when rule evaluation is not stopped", func(t *testing.T) {
		t.Run("Update should send to updateCh", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			resultCh := make(chan bool)
			go func() {
				resultCh <- r.update()
			}()
			select {
			case <-r.updateCh:
				require.True(t, <-resultCh)
			case <-time.After(5 * time.Second):
				t.Fatal("No message was received on update channel")
			}
		})
		t.Run("eval should send to evalCh", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			expected := time.Now()
			resultCh := make(chan bool)
			version := rand.Int63()
			go func() {
				resultCh <- r.eval(expected, version)
			}()
			select {
			case ctx := <-r.evalCh:
				require.Equal(t, version, ctx.version)
				require.Equal(t, expected, ctx.scheduledAt)
				require.True(t, <-resultCh)
			case <-time.After(5 * time.Second):
				t.Fatal("No message was received on eval channel")
			}
		})
		t.Run("eval should exit when context is cancelled", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			resultCh := make(chan bool)
			go func() {
				resultCh <- r.eval(time.Now(), rand.Int63())
			}()
			runtime.Gosched()
			r.stop()
			select {
			case result := <-resultCh:
				require.False(t, result)
			case <-time.After(5 * time.Second):
				t.Fatal("No message was received on eval channel")
			}
		})
	})
	t.Run("when rule evaluation is stopped", func(t *testing.T) {
		t.Run("Update should do nothing", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			r.stop()
			require.False(t, r.update())
		})
		t.Run("eval should do nothing", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			r.stop()
			require.False(t, r.eval(time.Now(), rand.Int63()))
		})
		t.Run("stop should do nothing", func(t *testing.T) {
			r := newAlertRuleInfo(context.Background())
			r.stop()
			r.stop()
		})
	})
	t.Run("should be thread-safe", func(t *testing.T) {
		r := newAlertRuleInfo(context.Background())
		wg := sync.WaitGroup{}
		go func() {
			for {
				select {
				case <-r.evalCh:
					time.Sleep(time.Microsecond)
				case <-r.updateCh:
					time.Sleep(time.Microsecond)
				case <-r.ctx.Done():
					return
				}
			}
		}()

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				for i := 0; i < 20; i++ {
					max := 3
					if i <= 10 {
						max = 2
					}
					switch rand.Intn(max) + 1 {
					case 1:
						r.update()
					case 2:
						r.eval(time.Now(), rand.Int63())
					case 3:
						r.stop()
					}
				}
				wg.Done()
			}()
		}

		wg.Wait()
	})
}

func TestSchedulableAlertRulesRegistry(t *testing.T) {
	r := schedulableAlertRulesRegistry{rules: make(map[models.AlertRuleKey]*models.SchedulableAlertRule)}
	assert.Len(t, r.all(), 0)

	// replace all rules in the registry with foo
	r.set([]*models.SchedulableAlertRule{{OrgID: 1, UID: "foo", Version: 1}})
	assert.Len(t, r.all(), 1)
	foo := r.get(models.AlertRuleKey{OrgID: 1, UID: "foo"})
	require.NotNil(t, foo)
	assert.Equal(t, models.SchedulableAlertRule{OrgID: 1, UID: "foo", Version: 1}, *foo)

	// update foo to a newer version
	r.update(&models.SchedulableAlertRule{OrgID: 1, UID: "foo", Version: 2})
	assert.Len(t, r.all(), 1)
	foo = r.get(models.AlertRuleKey{OrgID: 1, UID: "foo"})
	require.NotNil(t, foo)
	assert.Equal(t, models.SchedulableAlertRule{OrgID: 1, UID: "foo", Version: 2}, *foo)

	// update bar which does not exist in the registry
	r.update(&models.SchedulableAlertRule{OrgID: 1, UID: "bar", Version: 1})
	assert.Len(t, r.all(), 2)
	foo = r.get(models.AlertRuleKey{OrgID: 1, UID: "foo"})
	require.NotNil(t, foo)
	assert.Equal(t, models.SchedulableAlertRule{OrgID: 1, UID: "foo", Version: 2}, *foo)
	bar := r.get(models.AlertRuleKey{OrgID: 1, UID: "bar"})
	require.NotNil(t, foo)
	assert.Equal(t, models.SchedulableAlertRule{OrgID: 1, UID: "bar", Version: 1}, *bar)

	// replace all rules in the registry with baz
	r.set([]*models.SchedulableAlertRule{{OrgID: 1, UID: "baz", Version: 1}})
	assert.Len(t, r.all(), 1)
	baz := r.get(models.AlertRuleKey{OrgID: 1, UID: "baz"})
	require.NotNil(t, baz)
	assert.Equal(t, models.SchedulableAlertRule{OrgID: 1, UID: "baz", Version: 1}, *baz)
	assert.Nil(t, r.get(models.AlertRuleKey{OrgID: 1, UID: "foo"}))
	assert.Nil(t, r.get(models.AlertRuleKey{OrgID: 1, UID: "bar"}))

	// delete baz
	deleted, ok := r.del(models.AlertRuleKey{OrgID: 1, UID: "baz"})
	assert.True(t, ok)
	require.NotNil(t, deleted)
	assert.Equal(t, *deleted, *baz)
	assert.Len(t, r.all(), 0)
	assert.Nil(t, r.get(models.AlertRuleKey{OrgID: 1, UID: "baz"}))

	// baz cannot be deleted twice
	deleted, ok = r.del(models.AlertRuleKey{OrgID: 1, UID: "baz"})
	assert.False(t, ok)
	assert.Nil(t, deleted)
}
