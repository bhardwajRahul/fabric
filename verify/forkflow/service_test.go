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

package forkflow

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

	"github.com/microbus-io/fabric/verify/forkflow/forkflowapi"
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
	_ forkflowapi.Client
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

func TestForkflow_Pipe(t *testing.T) { // MARKER: Pipe
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

	t.Run("fork_at_step_b_with_override_runs_remainder", func(t *testing.T) {
		assert := testarossa.For(t)

		// Run the original flow with value=5: A passes through, B doubles to 10, C adds to 11.
		flowKey, err := foremanClient.Create(ctx, forkflowapi.Pipe.URL(), map[string]any{"value": 5}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		outcome, err := foremanClient.Await(ctx, flowKey)

		status, state := outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)
		assert.Expect(state["value"], 11.0)

		// Retrieve History; pick the stepKey of TaskB.
		history, err := foremanClient.History(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		// History.TaskName is the graph node name (not the URL).
		var stepBKey string
		for _, s := range history {
			if s.TaskName == "taskB" {
				stepBKey = s.StepKey
				break
			}
		}
		if !assert.Expect(stepBKey != "", true) {
			return
		}

		// Fork from B with value=100. The forked flow re-creates B with value=100, runs B (->200) and C (->201).
		forkedKey, err := foremanClient.Fork(ctx, stepBKey, map[string]any{"value": 100}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, forkedKey)
		if !assert.NoError(err) {
			return
		}
		outcome, err = foremanClient.Await(ctx, forkedKey)

		status, state = outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)
		assert.Expect(state["value"], 201.0)

		// The original flow is unaffected.
		outcome, err = foremanClient.Snapshot(ctx, flowKey)

		_, origState := outcomeStatusState(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(origState["value"], 11.0)
	})
}
