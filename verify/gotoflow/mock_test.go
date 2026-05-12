package gotoflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

func TestGotoflow_Mock(t *testing.T) {
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

		mock.MockTaskA(func(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error) {
			return
		})
		var loops int
		_, err := mock.TaskA(ctx, nil, loops)
		assert.NoError(err)
	})

	t.Run("task_b", func(t *testing.T) { // MARKER: TaskB
		assert := testarossa.For(t)

		mock.MockTaskB(func(ctx context.Context, flow *workflow.Flow, loops int, target int) (visited bool, err error) {
			return
		})
		var loops int
		var target int
		_, err := mock.TaskB(ctx, nil, loops, target)
		assert.NoError(err)
	})

	t.Run("task_c", func(t *testing.T) { // MARKER: TaskC
		assert := testarossa.For(t)

		mock.MockTaskC(func(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error) {
			return
		})
		var loops int
		_, err := mock.TaskC(ctx, nil, loops)
		assert.NoError(err)
	})

	t.Run("bad_gotoer", func(t *testing.T) { // MARKER: BadGotoer
		assert := testarossa.For(t)

		mock.MockBadGotoer(func(ctx context.Context, flow *workflow.Flow) (stamp bool, err error) {
			return
		})
		_, err := mock.BadGotoer(ctx, nil)
		assert.NoError(err)
	})

	t.Run("goto", func(t *testing.T) { // MARKER: Goto
		assert := testarossa.For(t)

		mock.MockGoto(func(ctx context.Context, flow *workflow.Flow, target int) (finalLoops int, err error) {
			return
		})
		graph, err := mock.Goto(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

	t.Run("bad_goto", func(t *testing.T) { // MARKER: BadGoto
		assert := testarossa.For(t)

		mock.MockBadGoto(func(ctx context.Context, flow *workflow.Flow) (stamp bool, err error) {
			return
		})
		graph, err := mock.BadGoto(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
