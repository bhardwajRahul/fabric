package gotoflow

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/gotoflow/gotoflowapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ httpx.BodyReader
	_ *workflow.Flow
	_ testarossa.Asserter
	_ gotoflowapi.Client
)

func TestGotoflow_Goto(t *testing.T) { // MARKER: Goto
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := gotoflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("loops_one_then_falls_through", func(t *testing.T) {
		// target=1: TaskA runs (loops=1), TaskB sees loops>=target, falls through to TaskC.
		assert := testarossa.For(t)

		finalLoops, status, err := exec.Goto(ctx, 1)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			finalLoops, 1,
		)
	})

	t.Run("loops_three_times_via_goto", func(t *testing.T) {
		// target=3: TaskA runs three times (loops goes 0->1->2->3), TaskB gotos twice
		// then falls through.
		assert := testarossa.For(t)

		finalLoops, status, err := exec.Goto(ctx, 3)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			finalLoops, 3,
		)
	})
}

func TestGotoflow_BadGoto(t *testing.T) { // MARKER: BadGoto
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := gotoflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("goto_to_unregistered_target_fails_flow", func(t *testing.T) {
		// BadGotoer calls flow.Goto with a target that has no AddTransitionGoto for it.
		// evaluateTransitions surfaces this as an error and failStep fails the flow.
		assert := testarossa.For(t)

		_, status, err := exec.BadGoto(ctx)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusFailed)
	})
}
