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

package ackdroppedflow

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/ackdroppedflow/ackdroppedflowapi"
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

func TestAckdroppedflow_BreakerTripsAndRecovers(t *testing.T) { // MARKER: AckDropped
	t.Parallel()
	ctx := t.Context()

	const (
		parkedFlows = 20 // flows blocked behind the tripped breaker; small to keep the test fast
		echoFlows   = 5  // unrelated flows that must continue to drain during the trip window
	)

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

	// Park's subscription is deactivated AFTER startup but BEFORE any flow is created. The
	// window between activation and deactivation is microseconds and no flow exists yet, so no
	// dispatch can land on the still-on-bus subscription.
	err := svc.DeactivateSubscription("Park")
	if !testarossa.For(t).NoError(err) {
		return
	}

	t.Run("park_flows_park_in_pending_and_breaker_trips", func(t *testing.T) {
		assert := testarossa.For(t)

		parkedKeys := make([]string, 0, parkedFlows)
		for i := range parkedFlows {
			fk, err := fm.Create(ctx, ackdroppedflowapi.AckDropped.URL(), map[string]any{"tag": "p" + strconv.Itoa(i)}, nil)
			if !assert.NoError(err) {
				return
			}
			if err := fm.Start(ctx, fk); !assert.NoError(err) {
				return
			}
			parkedKeys = append(parkedKeys, fk)
		}

		// Wait for the breaker to trip. First dispatch on an empty cache costs one refiller
		// scan, so the trip can take a few refill cycles to land. Poll up to a few seconds.
		tripped := waitFor(t, 5*time.Second, func() bool {
			return foremanSvc.BreakerTripped("park")
		})
		assert.True(tripped)

		// Park's handler must not have run even once - the subscription is off-bus.
		assert.Expect(svc.ParkHits(), int64(0))

		// AckDropped flows are still in flight (parked in pending, not failed). Sample one.
		outcome, err := fm.Snapshot(ctx, parkedKeys[0])

		status := outcomeStatus(outcome)
		if assert.NoError(err) {
			assert.True(status == workflow.StatusRunning || status == workflow.StatusCreated)
		}

		t.Run("echo_flows_drain_unimpeded_while_park_is_tripped", func(t *testing.T) {
			assert := testarossa.For(t)

			echoKeys := make([]string, 0, echoFlows)
			for i := range echoFlows {
				fk, err := fm.Create(ctx, ackdroppedflowapi.Echo.URL(), map[string]any{"tag": "e" + strconv.Itoa(i)}, nil)
				if !assert.NoError(err) {
					return
				}
				if err := fm.Start(ctx, fk); !assert.NoError(err) {
					return
				}
				echoKeys = append(echoKeys, fk)
			}

			echoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			for _, fk := range echoKeys {
				outcome, err = fm.Await(echoCtx, fk)

				status := outcomeStatus(outcome)
				assert.NoError(err)
				assert.Expect(status, workflow.StatusCompleted)
			}
			assert.True(svc.PingHits() >= int64(echoFlows))
			// Breaker for park must still be tripped after Echo drained.
			assert.True(foremanSvc.BreakerTripped("park"))
		})

		t.Run("reactivating_park_drains_blocked_flows", func(t *testing.T) {
			assert := testarossa.For(t)

			err := svc.ActivateSubscription("Park")
			if !assert.NoError(err) {
				return
			}

			drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			for _, fk := range parkedKeys {
				outcome, err = fm.Await(drainCtx, fk)

				status := outcomeStatus(outcome)
				assert.NoError(err)
				assert.Expect(status, workflow.StatusCompleted)
			}
			assert.True(svc.ParkHits() >= int64(parkedFlows))

			// Probe success reopens the breaker locally. Recovery is single-replica here,
			// so it should be observed by the time all flows have drained.
			open := waitFor(t, 5*time.Second, func() bool {
				return !foremanSvc.BreakerTripped("park")
			})
			assert.True(open)
		})
	})
}

// waitFor polls cond until it returns true or timeout elapses. Returns the final cond value.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}
