package aliasflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestAliasflow_Mock(t *testing.T) {
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

	t.Run("task_s", func(t *testing.T) { // MARKER: TaskS
		assert := testarossa.For(t)

		mock.MockTaskS(func(ctx context.Context, flow *workflow.Flow, branch string) (branchOut string, err error) {
			return
		})
		var branch string
		_, err := mock.TaskS(ctx, nil, branch)
		assert.NoError(err)
	})

	t.Run("task_a", func(t *testing.T) { // MARKER: TaskA
		assert := testarossa.For(t)

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) {
			return
		})
		var path string
		_, err := mock.TaskA(ctx, nil, path)
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

	t.Run("task_d", func(t *testing.T) { // MARKER: TaskD
		assert := testarossa.For(t)

		mock.MockTaskD(func(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) {
			return
		})
		var path string
		_, err := mock.TaskD(ctx, nil, path)
		assert.NoError(err)
	})

	t.Run("alias", func(t *testing.T) { // MARKER: Alias
		assert := testarossa.For(t)

		mock.MockAlias(func(ctx context.Context, flow *workflow.Flow, branch string) (path string, err error) {
			return
		})
		graph, err := mock.Alias(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
