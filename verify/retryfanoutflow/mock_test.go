package retryfanoutflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestRetryfanoutflow_Mock(t *testing.T) {
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

	t.Run("enter", func(t *testing.T) { // MARKER: Enter
		assert := testarossa.For(t)

		mock.MockEnter(func(ctx context.Context, flow *workflow.Flow, elements []int) (elementsOut []int, err error) {
			return
		})
		var elements []int
		_, err := mock.Enter(ctx, nil, elements)
		assert.NoError(err)
	})

	t.Run("increment", func(t *testing.T) { // MARKER: Increment
		assert := testarossa.For(t)

		mock.MockIncrement(func(ctx context.Context, flow *workflow.Flow, element int) (listResultOut []int, err error) {
			return
		})
		var element int
		_, err := mock.Increment(ctx, nil, element)
		assert.NoError(err)
	})

	t.Run("join", func(t *testing.T) { // MARKER: Join
		assert := testarossa.For(t)

		mock.MockJoin(func(ctx context.Context, flow *workflow.Flow, listResult []int) (listResultOut []int, err error) {
			return
		})
		var listResult []int
		_, err := mock.Join(ctx, nil, listResult)
		assert.NoError(err)
	})

	t.Run("retry_fan_out", func(t *testing.T) { // MARKER: RetryFanOut
		assert := testarossa.For(t)

		mock.MockRetryFanOut(func(ctx context.Context, flow *workflow.Flow, elements []int) (listResult []int, err error) {
			return
		})
		graph, err := mock.RetryFanOut(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
