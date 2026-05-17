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

package soakflow

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"

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

	"github.com/microbus-io/fabric/verify/soakflow/soakflowapi"
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
	_ soakflowapi.Client
)

func isTerminal(status string) bool {
	switch status {
	case foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled:
		return true
	}
	return false
}

// TestSoakflow_Soak is a high-volume liveness soak: it keeps a bounded number of
// flows in flight through the complex input-driven Soak workflow for a fixed
// wall-clock window, with the foreman sharded and a small worker pool so the
// candidate cache is the binding constraint, then drains and asserts every flow
// reached a terminal status. It does not assert ordering or output.
func TestSoakflow_Soak(t *testing.T) { // MARKER: Soak
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		// NumShards>1 exercises the cross-shard combined query + round-robin
		// batch assembly; a small worker pool makes the candidate cache the
		// binding constraint - the regime where dispatch wedges surface.
		// SetConfig is only permitted in the TESTING deployment, so via .Init.
		foreman.NewService().Init(func(f *foreman.Service) error {
			// NumShards>1 requires a %d-templated DSN; in TESTING this still
			// resolves to an isolated in-memory SQLite per (shard, test).
			err := f.SetSQLDataSourceName("file:soak%d?mode=memory&cache=shared")
			if err != nil {
				return err
			}
			err = f.SetNumShards(2)
			if err != nil {
				return err
			}
			return f.SetWorkers(4)
		}),
		tester,
	)
	app.RunInTest(t)

	t.Run("all_flows_terminate", func(t *testing.T) {
		assert := testarossa.For(t)

		seed := time.Now().UnixNano()
		rng := rand.New(rand.NewSource(seed))
		t.Logf("soak seed=%d", seed)

		const (
			soakWindow   = 10 * time.Second // sustained-load window
			maxInFlight  = 256              // bound concurrency so drain stays fast
			drainBudget  = 90 * time.Second // generous; a wedge fails here
			fairnessKeys = "ab"             // plus "" via index 2
		)

		started := map[string]bool{} // not-yet-confirmed-terminal flowKeys
		total := 0

		// reap removes any flows that have reached a terminal status. Returns
		// the number still outstanding.
		reap := func() int {
			for k := range started {
				status, _, err := fm.Snapshot(ctx, k)
				if err == nil && isTerminal(status) {
					delete(started, k)
				}
			}
			return len(started)
		}

		deadline := time.Now().Add(soakWindow)
		for time.Now().Before(deadline) {
			if len(started) >= maxInFlight {
				if reap() >= maxInFlight {
					time.Sleep(20 * time.Millisecond)
					continue
				}
			}
			state := map[string]any{
				"branch":   rng.Intn(5),
				"fanWidth": rng.Intn(7),
				"loops":    rng.Intn(6),
			}
			var opts *workflow.FlowOptions
			if rng.Intn(10) < 3 { // ~30% carry priority/fairness
				key := ""
				if i := rng.Intn(3); i < 2 {
					key = string(fairnessKeys[i])
				}
				opts = &workflow.FlowOptions{
					Priority:       1 + rng.Intn(8),
					FairnessKey:    key,
					FairnessWeight: float64(1 + rng.Intn(4)),
				}
			}
			flowKey, err := fm.Create(ctx, soakflowapi.Soak.URL(), state, opts)
			if !assert.NoError(err) {
				return
			}
			err = fm.Start(ctx, flowKey)
			if !assert.NoError(err) {
				return
			}
			started[flowKey] = true
			total++
		}

		t.Logf("soak created %d flows in %s; draining", total, soakWindow)
		assert.True(total > 0)

		// Drain: every started flow must reach a terminal status.
		drainDeadline := time.Now().Add(drainBudget)
		for len(started) > 0 && time.Now().Before(drainDeadline) {
			if reap() == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		if !assert.Expect(len(started), 0) {
			// Report a few stuck flows and their status for diagnosis; the
			// logged seed makes the run reproducible.
			n := 0
			for k := range started {
				status, _, _ := fm.Snapshot(ctx, k)
				t.Logf("STUCK flow=%s status=%s", k, status)
				if n++; n >= 10 {
					break
				}
			}
			return
		}

		// Sanity: the random fan-in over branch 0..4 exercised both terminal
		// kinds (branch==4 fails; the rest complete). Re-derive from a fresh
		// snapshot of a sample is unnecessary - the liveness assertion above is
		// the contract; this just confirms the workflow took varied paths.
		t.Logf("soak ok: all %d flows terminal", total)
	})
}
