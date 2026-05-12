package calculator

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
)

func TestCalculator_Mock(t *testing.T) {
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

	t.Run("on_observe_sum_operations", func(t *testing.T) { // MARKER: SumOperations
		assert := testarossa.For(t)

		mock.MockOnObserveSumOperations(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnObserveSumOperations(ctx)
		assert.NoError(err)
	})

	t.Run("arithmetic", func(t *testing.T) { // MARKER: Arithmetic
		assert := testarossa.For(t)

		mock.MockArithmetic(func(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) {
			return
		})
		var x int
		var op string
		var y int
		_, _, _, _, err := mock.Arithmetic(ctx, x, op, y)
		assert.NoError(err)
	})

	t.Run("square", func(t *testing.T) { // MARKER: Square
		assert := testarossa.For(t)

		mock.MockSquare(func(ctx context.Context, x int) (xEcho int, result int, err error) {
			return
		})
		var x int
		_, _, err := mock.Square(ctx, x)
		assert.NoError(err)
	})

	t.Run("distance", func(t *testing.T) { // MARKER: Distance
		assert := testarossa.For(t)

		mock.MockDistance(func(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error) {
			return
		})
		var p1 calculatorapi.Point
		var p2 calculatorapi.Point
		_, err := mock.Distance(ctx, p1, p2)
		assert.NoError(err)
	})

}
