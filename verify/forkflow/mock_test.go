package forkflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestForkflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) {
			return
		})
		var value int
		_, err := mock.TaskA(ctx, nil, value)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) {
			return
		})
		var value int
		_, err := mock.TaskB(ctx, nil, value)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) {
			return
		})
		var value int
		_, err := mock.TaskC(ctx, nil, value)
		assert.NoError(err)
	})

	t.Run("pipe", func(t *testing.T) { // MARKER: Pipe
		assert := testarossa.For(t)

		mock.MockPipe(func(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) {
			return
		})
		graph, err := mock.Pipe(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
