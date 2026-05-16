package interruptedfanoutflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestInterruptedfanoutflow_Mock(t *testing.T) {
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

	t.Run("src", func(t *testing.T) { // MARKER: Src
		assert := testarossa.For(t)

		mock.MockSrc(func(ctx context.Context, flow *workflow.Flow) (started bool, err error) {
			return
		})
		_, err := mock.Src(ctx, nil)
		assert.NoError(err)
	})

	t.Run("a", func(t *testing.T) { // MARKER: A
		assert := testarossa.For(t)

		mock.MockA(func(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) {
			return
		})
		_, err := mock.A(ctx, nil)
		assert.NoError(err)
	})

	t.Run("b", func(t *testing.T) { // MARKER: B
		assert := testarossa.For(t)

		mock.MockB(func(ctx context.Context, flow *workflow.Flow, resumed bool) (sumExecutedOut int, err error) {
			return
		})
		var resumed bool
		_, err := mock.B(ctx, nil, resumed)
		assert.NoError(err)
	})

	t.Run("c", func(t *testing.T) { // MARKER: C
		assert := testarossa.For(t)

		mock.MockC(func(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) {
			return
		})
		_, err := mock.C(ctx, nil)
		assert.NoError(err)
	})

	t.Run("j", func(t *testing.T) { // MARKER: J
		assert := testarossa.For(t)

		mock.MockJ(func(ctx context.Context, flow *workflow.Flow, sumExecuted int) (totalExecuted int, err error) {
			return
		})
		var sumExecuted int
		_, err := mock.J(ctx, nil, sumExecuted)
		assert.NoError(err)
	})

	t.Run("interrupted_fan_out", func(t *testing.T) { // MARKER: InterruptedFanOut
		assert := testarossa.For(t)

		mock.MockInterruptedFanOut(func(ctx context.Context, flow *workflow.Flow) (sumExecuted int, totalExecuted int, err error) {
			return
		})
		graph, err := mock.InterruptedFanOut(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
