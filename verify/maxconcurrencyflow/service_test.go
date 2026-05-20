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

package maxconcurrencyflow

import (
	"strconv"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/maxconcurrencyflow/maxconcurrencyflowapi"
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

func TestMaxconcurrencyflow_MaxConcurrency(t *testing.T) { // MARKER: MaxConcurrency
	t.Parallel()
	ctx := t.Context()

	const (
		cap     = 3                     // task self-limits at 3 concurrent
		workers = 6                     // foreman pool larger than cap so initial dispatch overshoots and provokes 429s
		flows   = 24                    // enough flows to keep refills firing while the cut takes effect
		dwell   = 60 * time.Millisecond // simulated work per task; long enough for concurrent overlap
	)

	// Initialize the microservice under test
	svc := NewService()
	svc.SetCap(cap).SetDwell(dwell)

	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	foremanSvc := foreman.NewService().Init(func(f *foreman.Service) error {
		f.SetWorkers(workers)
		f.SetSQLConnectionPool(workers)
		return nil
	})

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foremanSvc,
		tester,
	)
	app.RunInTest(t)

	t.Run("backpressure_bounds_max_concurrency", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create + start flows.
		flowKeys := make([]string, 0, flows)
		for i := range flows {
			fk, err := fm.Create(ctx, maxconcurrencyflowapi.MaxConcurrency.URL(), map[string]any{"tag": "f" + strconv.Itoa(i)}, nil)
			if !assert.NoError(err) {
				return
			}
			if err := fm.Start(ctx, fk); !assert.NoError(err) {
				return
			}
			flowKeys = append(flowKeys, fk)
		}

		// Every flow must eventually complete. A 429 bounces the step, not the flow.
		for _, fk := range flowKeys {
			outcome, err := fm.Await(ctx, fk)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.Expect(status, workflow.StatusCompleted)
		}

		peak, rejections := svc.Observed()
		t.Logf("peak in-flight observed: %d (cap=%d, workers=%d); rejections emitted: %d", peak, cap, workers, rejections)

		// The 429 path must have actually fired at least once; otherwise the
		// fixture never exercised backpressure.
		assert.True(rejections >= 1)

		// Soft cap upper bound: the peak observation is bounded by the worker
		// pool size for this replica - the foreman cannot dispatch more
		// concurrent steps than it has workers. With one replica, peerCount=1,
		// so the per-replica admission cap equals the cluster headroom.
		assert.True(peak <= workers)

		// The regulate path must have populated taskValves. This catches the
		// "regulate is a no-op" / "SyncValve drops the point" regressions
		// that the rejections+peak checks alone cannot distinguish from a
		// healthy controller.
		assert.True(foremanSvc.ValveCount() >= 1)
	})
}
