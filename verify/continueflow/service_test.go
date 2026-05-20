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

package continueflow

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

	"github.com/microbus-io/fabric/verify/continueflow/continueflowapi"
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
	_ continueflowapi.Client
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

func TestContinueflow_Counting(t *testing.T) { // MARKER: Counting
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

	t.Run("counter_persists_across_continue_turns", func(t *testing.T) {
		assert := testarossa.For(t)

		// Turn 1: create + start a flow starting from counter=0.
		flowKey1, err := foremanClient.Create(ctx, continueflowapi.Counting.URL(), map[string]any{"counter": 0}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey1)
		if !assert.NoError(err) {
			return
		}
		outcome, err := foremanClient.Await(ctx, flowKey1)

		status, state := outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)
		// counter advanced 0 -> 1
		assert.Expect(state["counter"], 1.0) // JSON unmarshal turns ints into float64

		// Turn 2: continue from the thread, no additional state.
		flowKey2, err := foremanClient.Continue(ctx, flowKey1, map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey2)
		if !assert.NoError(err) {
			return
		}
		outcome, err = foremanClient.Await(ctx, flowKey2)

		status, state = outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)
		// counter advanced 1 -> 2 by carrying forward across the Continue boundary
		assert.Expect(state["counter"], 2.0)
	})
}
