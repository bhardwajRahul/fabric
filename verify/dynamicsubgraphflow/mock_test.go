package dynamicsubgraphflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestDynamicsubgraphflow_Mock(t *testing.T) {
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

	t.Run("parent", func(t *testing.T) { // MARKER: Parent
		assert := testarossa.For(t)

		mock.MockParent(func(ctx context.Context, flow *workflow.Flow, value int, innerDone bool, innerResult int) (parentResult string, err error) {
			return
		})
		var value int
		var innerDone bool
		var innerResult int
		_, err := mock.Parent(ctx, nil, value, innerDone, innerResult)
		assert.NoError(err)
	})

	t.Run("inner_a", func(t *testing.T) { // MARKER: InnerA
		assert := testarossa.For(t)

		mock.MockInnerA(func(ctx context.Context, flow *workflow.Flow, value int) (innerStage int, err error) {
			return
		})
		var value int
		_, err := mock.InnerA(ctx, nil, value)
		assert.NoError(err)
	})

	t.Run("inner_b", func(t *testing.T) { // MARKER: InnerB
		assert := testarossa.For(t)

		mock.MockInnerB(func(ctx context.Context, flow *workflow.Flow, innerStage int) (innerResult int, innerDone bool, err error) {
			return
		})
		var innerStage int
		_, _, err := mock.InnerB(ctx, nil, innerStage)
		assert.NoError(err)
	})

	t.Run("inner", func(t *testing.T) { // MARKER: Inner
		assert := testarossa.For(t)

		mock.MockInner(func(ctx context.Context, flow *workflow.Flow, value int) (innerResult int, innerDone bool, err error) {
			return
		})
		graph, err := mock.Inner(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

	t.Run("dynamic_subgraph", func(t *testing.T) { // MARKER: DynamicSubgraph
		assert := testarossa.For(t)

		mock.MockDynamicSubgraph(func(ctx context.Context, flow *workflow.Flow, value int) (parentResult string, err error) {
			return
		})
		graph, err := mock.DynamicSubgraph(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
