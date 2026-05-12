package errorflow

import (
	"context"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestErrorflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error) {
			return
		})
		var trigger string
		_, err := mock.TaskA(ctx, nil, trigger)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error) {
			return
		})
		var trigger string
		_, err := mock.TaskB(ctx, nil, trigger)
		assert.NoError(err)
	})

	t.Run("handler", func(t *testing.T) { // MARKER: Handler
		assert := testarossa.For(t)

		mock.MockHandler(func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error) {
			return
		})
		var onErr *errors.TracedError
		_, err := mock.Handler(ctx, nil, onErr)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error) {
			return
		})
		var result string
		_, err := mock.TaskC(ctx, nil, result)
		assert.NoError(err)
	})

	t.Run("error", func(t *testing.T) { // MARKER: Error
		assert := testarossa.For(t)

		mock.MockError(func(ctx context.Context, flow *workflow.Flow, trigger string) (finalResult string, err error) {
			return
		})
		graph, err := mock.Error(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
