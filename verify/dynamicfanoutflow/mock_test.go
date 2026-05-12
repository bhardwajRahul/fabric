package dynamicfanoutflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestDynamicfanoutflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) {
			return
		})
		var items []string
		_, err := mock.TaskA(ctx, nil, items)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error) {
			return
		})
		var item string
		_, err := mock.TaskB(ctx, nil, item)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error) {
			return
		})
		var sumProcessed int
		_, err := mock.TaskC(ctx, nil, sumProcessed)
		assert.NoError(err)
	})

	t.Run("dynamic_fan_out", func(t *testing.T) { // MARKER: DynamicFanOut
		assert := testarossa.For(t)

		mock.MockDynamicFanOut(func(ctx context.Context, flow *workflow.Flow, items []string) (processedCount int, err error) {
			return
		})
		graph, err := mock.DynamicFanOut(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
