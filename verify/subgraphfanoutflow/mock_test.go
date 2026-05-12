package subgraphfanoutflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestSubgraphfanoutflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow) (started bool, err error) {
			return
		})
		_, err := mock.TaskA(ctx, nil)
		assert.NoError(err)
	})

	t.Run("normal_b", func(t *testing.T) { // MARKER: NormalB
		assert := testarossa.For(t)

		mock.MockNormalB(func(ctx context.Context, flow *workflow.Flow) (resultB string, err error) {
			return
		})
		_, err := mock.NormalB(ctx, nil)
		assert.NoError(err)
	})

	t.Run("task_x", func(t *testing.T) { // MARKER: TaskX
		assert := testarossa.For(t)

		mock.MockTaskX(func(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error) {
			return
		})
		_, err := mock.TaskX(ctx, nil)
		assert.NoError(err)
	})

	t.Run("task_y", func(t *testing.T) { // MARKER: TaskY
		assert := testarossa.For(t)

		mock.MockTaskY(func(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error) {
			return
		})
		var xPassed bool
		_, err := mock.TaskY(ctx, nil, xPassed)
		assert.NoError(err)
	})

	t.Run("normal_d", func(t *testing.T) { // MARKER: NormalD
		assert := testarossa.For(t)

		mock.MockNormalD(func(ctx context.Context, flow *workflow.Flow) (resultD string, err error) {
			return
		})
		_, err := mock.NormalD(ctx, nil)
		assert.NoError(err)
	})

	t.Run("task_e", func(t *testing.T) { // MARKER: TaskE
		assert := testarossa.For(t)

		mock.MockTaskE(func(ctx context.Context, flow *workflow.Flow, resultB string, subResult string, resultD string) (finalResult string, err error) {
			return
		})
		var resultB string
		var subResult string
		var resultD string
		_, err := mock.TaskE(ctx, nil, resultB, subResult, resultD)
		assert.NoError(err)
	})

	t.Run("sub", func(t *testing.T) { // MARKER: Sub
		assert := testarossa.For(t)

		mock.MockSub(func(ctx context.Context, flow *workflow.Flow) (subResult string, err error) {
			return
		})
		graph, err := mock.Sub(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

	t.Run("sub_fan_out", func(t *testing.T) { // MARKER: SubFanOut
		assert := testarossa.For(t)

		mock.MockSubFanOut(func(ctx context.Context, flow *workflow.Flow) (finalResult string, err error) {
			return
		})
		graph, err := mock.SubFanOut(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
