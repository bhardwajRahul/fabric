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

package adaptiveconcurrencyflow

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

	"github.com/microbus-io/fabric/verify/adaptiveconcurrencyflow/adaptiveconcurrencyflowapi"
)

// TestAdaptiveconcurrencyflow_RateConvergence verifies the guarantees of the per-replica rate
// controller against a rate-bounded backend shared by multiple foreman replicas. The backend
// admits 7 ops/sec via an internal sliding-window throttle; 3 foreman replicas drive 49 flows
// against it. The assertions are the controller's actual contract: (1) every flow completes
// (429 bounces never fail steps); (2) at least one rejection fired (the feedback path was
// exercised); (3) every replica's valve recorded the cut (gossip + per-replica observation
// both worked). Wall-clock completion time is *not* asserted - the per-replica controllers
// oscillate around the shared budget and do not provide a tight time-to-completion guarantee.

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

func TestAdaptiveconcurrencyflow_RateConvergence(t *testing.T) { // MARKER: AdaptiveConcurrency
	t.Parallel()
	ctx := t.Context()

	const (
		rate      = 7                     // backend's per-second admission budget
		dwell     = 10 * time.Millisecond // per-step backend work
		nReplicas = 3
		flows     = 49
	)

	// Initialize the microservice under test
	svc := NewService()
	svc.SetRate(rate).SetDwell(dwell)

	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	// Multiple foreman replicas with default config. In TESTING the plane is derived from the
	// test name (same across replicas) and OpenTesting caches by (driver, dsn, plane_shardN),
	// so all replicas reuse the same in-memory SQLite without an explicit DSN. Default Workers
	// (64) is fine - the rate controller bounds emissions regardless of pool size.
	replicas := make([]*foreman.Service, nReplicas)
	for i := range replicas {
		replicas[i] = foreman.NewService().Init(func(f *foreman.Service) error {
			f.SetSQLConnectionPool(1)
			return nil
		})
	}

	// Run the testing app
	app := application.New()
	app.Add(svc, tester)
	for _, r := range replicas {
		app.Add(r)
	}
	app.RunInTest(t)

	assert := testarossa.For(t)

	flowKeys := make([]string, 0, flows)
	for i := range flows {
		fk, err := fm.Create(ctx, adaptiveconcurrencyflowapi.AdaptiveConcurrency.URL(),
			map[string]any{"tag": "f" + strconv.Itoa(i)}, nil)
		if !assert.NoError(err) {
			return
		}
		if err := fm.Start(ctx, fk); !assert.NoError(err) {
			return
		}
		flowKeys = append(flowKeys, fk)
	}

	t0 := time.Now()
	for _, fk := range flowKeys {
		outcome, err := fm.Await(ctx, fk)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
	}
	elapsed := time.Since(t0)

	completions, rejections := svc.Observed()
	t.Logf("rate=%d/s flows=%d replicas=%d elapsed=%s completions=%d rejections=%d",
		rate, flows, nReplicas, elapsed, completions, rejections)

	// Guarantee 1: every flow's Adaptive task completed exactly once. 429s bounce, never fail.
	assert.Expect(completions, flows)

	// Guarantee 2: the controller's feedback path was exercised - at least one 429 fired. With 49
	// flows against a 7/s budget some dispatch must have been refused, otherwise the test never
	// exercised valveRegulate.
	assert.True(rejections >= 1)

	// Guarantee 3: every replica recorded a valve for the task. With gossip (SyncValve) any
	// 429 propagates to all replicas as a (wCong, tCong) anchor; with the per-replica
	// observation any replica that itself emitted a 429-causing dispatch also has a local valve.
	// Across 3 replicas driving the load against a single shared backend, all three should have
	// observed at least one of those two paths.
	for i, r := range replicas {
		assert.True(r.ValveCount() >= 1, "replica %d has no valve", i)
	}
}
