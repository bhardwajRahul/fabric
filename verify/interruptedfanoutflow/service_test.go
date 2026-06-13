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

package interruptedfanoutflow

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

	"github.com/microbus-io/fabric/verify/interruptedfanoutflow/interruptedfanoutflowapi"
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
	_ interruptedfanoutflowapi.Client
)

// asInt coerces a state map value (absent -> nil, JSON number -> float64) to int.

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

func asInt(v any) int {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}

func TestInterruptedfanoutflow_InterruptedFanOut(t *testing.T) { // MARKER: InterruptedFanOut
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("interrupt_then_resume_completes_with_sum_3", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := fm.Create(ctx, interruptedfanoutflowapi.InterruptedFanOut.URL(), map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
		err = fm.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		// B interrupts on its first run; the fan-in never reaches all three
		// arrivals, so the flow parks as interrupted.
		outcome, err := fm.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		assert.Expect(
			err, nil,
			status, workflow.StatusInterrupted,
		)

		// Resume B with resumed=true. It re-runs, falls through, contributes 1.
		err = fm.Resume(ctx, flowKey, map[string]any{"resumed": true})
		if !assert.NoError(err) {
			return
		}

		outcome, err = fm.Await(ctx, flowKey)


		status, state := outcomeStatusState(outcome)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
		)
		// A + B + C each contributed 1; the Add reducer on `executed` sums them at fan-in.
		assert.Expect(asInt(state["executed"]), 3)
		// J surfaced the summed value, proving the fan-in saw 3.
		assert.Expect(asInt(state["totalExecuted"]), 3)
	})
}
