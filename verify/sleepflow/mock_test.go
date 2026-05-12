package sleepflow

import (
	"context"
	"testing"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestSleepflow_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("task_a", func(t *testing.T) { // MARKER: TaskA
		assert := testarossa.For(t)

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (sleepForOut time.Duration, err error) {
			return
		})
		var sleepFor time.Duration
		_, err := mock.TaskA(ctx, nil, sleepFor)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (marked bool, err error) {
			return
		})
		var sleepFor time.Duration
		_, err := mock.TaskB(ctx, nil, sleepFor)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, marked bool) (completed bool, err error) {
			return
		})
		var marked bool
		_, err := mock.TaskC(ctx, nil, marked)
		assert.NoError(err)
	})

	t.Run("delay", func(t *testing.T) { // MARKER: Delay
		assert := testarossa.For(t)

		mock.MockDelay(func(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (completed bool, err error) {
			return
		})
		graph, err := mock.Delay(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
