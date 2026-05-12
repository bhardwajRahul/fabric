package breakpointflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestBreakpointflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow) (stepA bool, err error) {
			return
		})
		_, err := mock.TaskA(ctx, nil)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error) {
			return
		})
		var stepA bool
		_, err := mock.TaskB(ctx, nil, stepA)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error) {
			return
		})
		var stepB bool
		_, err := mock.TaskC(ctx, nil, stepB)
		assert.NoError(err)
	})

	t.Run("breakpoint", func(t *testing.T) { // MARKER: Breakpoint
		assert := testarossa.For(t)

		mock.MockBreakpoint(func(ctx context.Context, flow *workflow.Flow) (stepC bool, err error) {
			return
		})
		graph, err := mock.Breakpoint(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
