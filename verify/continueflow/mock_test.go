package continueflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestContinueflow_Mock(t *testing.T) {
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

	t.Run("increment", func(t *testing.T) { // MARKER: Increment
		assert := testarossa.For(t)

		mock.MockIncrement(func(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) {
			return
		})
		var counter int
		_, err := mock.Increment(ctx, nil, counter)
		assert.NoError(err)
	})

	t.Run("counting", func(t *testing.T) { // MARKER: Counting
		assert := testarossa.For(t)

		mock.MockCounting(func(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) {
			return
		})
		graph, err := mock.Counting(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
