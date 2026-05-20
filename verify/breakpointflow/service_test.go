/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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


// outcomeStatus extracts the Status from a FlowOutcome, returning "" on nil.
func outcomeStatus(o *workflow.FlowOutcome) string {
	if o == nil {
		return ""
	}
	return o.Status
}

// outcomeState extracts the State from a FlowOutcome, returning nil on nil.
func outcomeState(o *workflow.FlowOutcome) map[string]any {
	if o == nil {
		return nil
	}
	return o.State
}

// outcomeStatusState extracts the Status and State from a FlowOutcome.
func outcomeStatusState(o *workflow.FlowOutcome) (string, map[string]any) {
	if o == nil {
		return "", nil
	}
	return o.Status, o.State
}

func TestBreakpointflow_Breakpoint(t *testing.T) { // MARKER: Breakpoint
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("breakpoint_pauses_before_TaskB_then_resume_completes_flow", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create the flow.
		flowKey, err := foremanClient.Create(ctx, breakpointflowapi.Breakpoint.URL(), map[string]any{}, nil)
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

		outcome, err := foremanClient.Await(ctx, flowKey)


		status, state := outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusInterrupted)
		// TaskA ran (stepA in state), TaskB and TaskC did not yet.
		assert.Expect(state["stepA"], true)
		assert.Expect(state["stepB"] == nil || state["stepB"] == false, true)
		assert.Expect(state["stepC"] == nil || state["stepC"] == false, true)

		// Resume past the breakpoint.
		err = foremanClient.Resume(ctx, flowKey, map[string]any{})
		if !assert.NoError(err) {
			return
		}

		outcome, err = foremanClient.Await(ctx, flowKey)


		status, state = outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)
		// All three steps ran.
		assert.Expect(state["stepC"], true)
	})
}
