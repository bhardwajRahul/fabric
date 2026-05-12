package breakpointflow

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

	"github.com/microbus-io/fabric/verify/breakpointflow/breakpointflowapi"
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
	_ breakpointflowapi.Client
)

func TestBreakpointflow_Breakpoint(t *testing.T) { // MARKER: Breakpoint
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("breakpoint_pauses_before_TaskB_then_resume_completes_flow", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create the flow.
		flowKey, err := foremanClient.Create(ctx, breakpointflowapi.Breakpoint.URL(), map[string]any{})
		if !assert.NoError(err) {
			return
		}

		// Set a breakpoint on TaskB. The task name is its URL.
		err = foremanClient.BreakBefore(ctx, flowKey, breakpointflowapi.TaskB.URL(), true)
		if !assert.NoError(err) {
			return
		}

		// Start the flow. It runs A, hits the breakpoint before B, and parks.
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		status, state, err := foremanClient.Await(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusInterrupted)
		// TaskA ran (stepA in state), TaskB and TaskC did not yet.
		assert.Expect(state["stepA"], true)
		assert.Expect(state["stepB"] == nil || state["stepB"] == false, true)
		assert.Expect(state["stepC"] == nil || state["stepC"] == false, true)

		// Resume past the breakpoint.
		err = foremanClient.Resume(ctx, flowKey, map[string]any{})
		if !assert.NoError(err) {
			return
		}

		status, state, err = foremanClient.Await(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusCompleted)
		// All three steps ran.
		assert.Expect(state["stepC"], true)
	})
}
