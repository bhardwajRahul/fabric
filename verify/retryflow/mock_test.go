package retryflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestRetryflow_Mock(t *testing.T) {
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

	t.Run("flaky", func(t *testing.T) { // MARKER: Flaky
		assert := testarossa.For(t)

		mock.MockFlaky(func(ctx context.Context, flow *workflow.Flow, attempts int, target int) (attemptsOut int, err error) {
			return
		})
		var attempts int
		var target int
		_, err := mock.Flaky(ctx, nil, attempts, target)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error) {
			return
		})
		var attempts int
		_, err := mock.TaskB(ctx, nil, attempts)
		assert.NoError(err)
	})

	t.Run("retry", func(t *testing.T) { // MARKER: Retry
		assert := testarossa.For(t)

		mock.MockRetry(func(ctx context.Context, flow *workflow.Flow, target int) (finalAttempts int, err error) {
			return
		})
		graph, err := mock.Retry(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
