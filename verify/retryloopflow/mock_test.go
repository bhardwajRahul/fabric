package retryloopflow

import (
	"context"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestRetryloopflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error) {
			return
		})
		var target int
		_, err := mock.TaskA(ctx, nil, target)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, attempts int, target int) (succeeded bool, err error) {
			return
		})
		var attempts int
		var target int
		_, err := mock.TaskB(ctx, nil, attempts, target)
		assert.NoError(err)
	})

	t.Run("handler", func(t *testing.T) { // MARKER: Handler
		assert := testarossa.For(t)

		mock.MockHandler(func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError, attempts int) (attemptsOut int, err error) {
			return
		})
		var onErr *errors.TracedError
		var attempts int
		_, err := mock.Handler(ctx, nil, onErr, attempts)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error) {
			return
		})
		var attempts int
		_, err := mock.TaskC(ctx, nil, attempts)
		assert.NoError(err)
	})

	t.Run("retry_loop", func(t *testing.T) { // MARKER: RetryLoop
		assert := testarossa.For(t)

		mock.MockRetryLoop(func(ctx context.Context, flow *workflow.Flow, target int) (finalAttempts int, err error) {
			return
		})
		graph, err := mock.RetryLoop(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
