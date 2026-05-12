package perelementpipelineflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestPerelementpipelineflow_Mock(t *testing.T) {
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

		mock.MockTaskS(func(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) {
			return
		})
		var items []string
		_, err := mock.TaskS(ctx, nil, items)
		assert.NoError(err)
	})

	t.Run("task_h", func(t *testing.T) { // MARKER: TaskH
		assert := testarossa.For(t)

		mock.MockTaskH(func(ctx context.Context, flow *workflow.Flow, item string) (itemUpper string, err error) {
			return
		})
		var item string
		_, err := mock.TaskH(ctx, nil, item)
		assert.NoError(err)
	})

	t.Run("task_a", func(t *testing.T) { // MARKER: TaskA
		assert := testarossa.For(t)

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, itemUpper string) (aProcessed string, err error) {
			return
		})
		var itemUpper string
		_, err := mock.TaskA(ctx, nil, itemUpper)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, itemUpper string) (bProcessed string, err error) {
			return
		})
		var itemUpper string
		_, err := mock.TaskB(ctx, nil, itemUpper)
		assert.NoError(err)
	})

	t.Run("task_m", func(t *testing.T) { // MARKER: TaskM
		assert := testarossa.For(t)

		mock.MockTaskM(func(ctx context.Context, flow *workflow.Flow, aProcessed string, bProcessed string) (setMerged []string, err error) {
			return
		})
		var aProcessed string
		var bProcessed string
		_, err := mock.TaskM(ctx, nil, aProcessed, bProcessed)
		assert.NoError(err)
	})

	t.Run("task_l", func(t *testing.T) { // MARKER: TaskL
		assert := testarossa.For(t)

		mock.MockTaskL(func(ctx context.Context, flow *workflow.Flow, setMerged []string) (finalCount int, err error) {
			return
		})
		var setMerged []string
		_, err := mock.TaskL(ctx, nil, setMerged)
		assert.NoError(err)
	})

	t.Run("per_element_pipeline", func(t *testing.T) { // MARKER: PerElementPipeline
		assert := testarossa.For(t)

		mock.MockPerElementPipeline(func(ctx context.Context, flow *workflow.Flow, items []string) (finalCount int, err error) {
			return
		})
		graph, err := mock.PerElementPipeline(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
