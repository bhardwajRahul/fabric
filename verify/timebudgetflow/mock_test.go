package timebudgetflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestTimebudgetflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow) (started bool, err error) {
			return
		})
		_, err := mock.TaskA(ctx, nil)
		assert.NoError(err)
	})

	t.Run("slow", func(t *testing.T) { // MARKER: Slow
		assert := testarossa.For(t)

		mock.MockSlow(func(ctx context.Context, flow *workflow.Flow) (done bool, err error) {
			return
		})
		_, err := mock.Slow(ctx, nil)
		assert.NoError(err)
	})

	t.Run("time_budget", func(t *testing.T) { // MARKER: TimeBudget
		assert := testarossa.For(t)

		mock.MockTimeBudget(func(ctx context.Context, flow *workflow.Flow) (done bool, err error) {
			return
		})
		graph, err := mock.TimeBudget(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
