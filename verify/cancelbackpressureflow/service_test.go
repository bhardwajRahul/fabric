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

package cancelbackpressureflow

import (
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/cancelbackpressureflow/cancelbackpressureflowapi"
)

// TestCancelbackpressureflow_StatusGuardRace covers the race between Cancel
// committing and a 429 returning from the same step. The bounce path's UPDATE
// is gated by WHERE status='running'; once Cancel has set the step to
// cancelled, the UPDATE must affect zero rows so the step is not resurrected
// into pending.

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

func TestCancelbackpressureflow_StatusGuardRace(t *testing.T) { // MARKER: CancelBackpressure
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()
	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	foremanSvc := foreman.NewService().Init(func(f *foreman.Service) error {
		f.SetSQLConnectionPool(1)
		return nil
	})

	// Run the testing app
	app := application.New()
	app.Add(svc, foremanSvc, tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// Create and start a flow. The task signals ready once it begins running.
	fk, err := fm.Create(ctx, cancelbackpressureflowapi.CancelBackpressure.URL(), map[string]any{"tag": "race"}, nil)
	if !assert.NoError(err) {
		return
	}
	err = fm.Start(ctx, fk)
	if !assert.NoError(err) {
		return
	}

	// Wait for the task to be parked in its release-wait. At this point the
	// foreman has the step in 'running' status under a lease.
	select {
	case <-svc.Ready():
	case <-time.After(10 * time.Second):
		t.Fatal("task did not enter parked state")
	}

	// Cancel the flow. This commits a transaction that sets the step (and
	// flow) to cancelled. After Cancel returns, the step row is cancelled.
	err = fm.Cancel(ctx, fk, "")
	if !assert.NoError(err) {
		return
	}

	// Release the parked task so it returns 429. The dispatch path will now
	// invoke handleBackpressure; its UPDATE WHERE status='running' must find
	// zero rows because Cancel already moved the step out of 'running'.
	svc.Release()

	// Await terminal status - should be cancelled, not anything else, and the
	// flow must not be stuck.
	outcome, err := fm.Await(ctx, fk)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Expect(status, workflow.StatusCancelled)

	// Give the bounce path a moment to (try to) UPDATE.
	time.Sleep(200 * time.Millisecond)

	// Inspect the step row through History. The bounce must not have written
	// status back to pending - the status-guard predicate prevents that.
	steps, err := fm.History(ctx, fk)
	if !assert.NoError(err) {
		return
	}
	if !assert.True(len(steps) >= 1) {
		return
	}
	for _, s := range steps {
		if s.TaskName == "bounce-and-cancel" {
			assert.Expect(s.Status, workflow.StatusCancelled)
		}
	}
}
