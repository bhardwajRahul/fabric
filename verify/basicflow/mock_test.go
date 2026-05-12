package basicflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestBasicflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow) (path string, err error) {
			return
		})
		_, err := mock.TaskA(ctx, nil)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) {
			return
		})
		var path string
		_, err := mock.TaskB(ctx, nil, path)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) {
			return
		})
		var path string
		_, err := mock.TaskC(ctx, nil, path)
		assert.NoError(err)
	})

	t.Run("basic", func(t *testing.T) { // MARKER: Basic
		assert := testarossa.For(t)

		mock.MockBasic(func(ctx context.Context, flow *workflow.Flow) (path string, err error) {
			return
		})
		graph, err := mock.Basic(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
