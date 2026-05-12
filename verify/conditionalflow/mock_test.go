package conditionalflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestConditionalflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error) {
			return
		})
		var score int
		_, err := mock.TaskA(ctx, nil, score)
		assert.NoError(err)
	})

	t.Run("task_high", func(t *testing.T) { // MARKER: TaskHigh
		assert := testarossa.For(t)

		mock.MockTaskHigh(func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) {
			return
		})
		var score int
		_, err := mock.TaskHigh(ctx, nil, score)
		assert.NoError(err)
	})

	t.Run("task_low", func(t *testing.T) { // MARKER: TaskLow
		assert := testarossa.For(t)

		mock.MockTaskLow(func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) {
			return
		})
		var score int
		_, err := mock.TaskLow(ctx, nil, score)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error) {
			return
		})
		var branch string
		_, err := mock.TaskC(ctx, nil, branch)
		assert.NoError(err)
	})

	t.Run("conditional", func(t *testing.T) { // MARKER: Conditional
		assert := testarossa.For(t)

		mock.MockConditional(func(ctx context.Context, flow *workflow.Flow, score int) (finalBranch string, err error) {
			return
		})
		graph, err := mock.Conditional(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
