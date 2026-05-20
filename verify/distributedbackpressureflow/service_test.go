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

package distributedbackpressureflow

import (
	"strconv"
	"strings"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/distributedbackpressureflow/distributedbackpressureflowapi"
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

func TestDistributedbackpressureflow_DistributedBackpressure(t *testing.T) { // MARKER: DistributedBackpressure
	t.Parallel()
	ctx := t.Context()

	const (
		cap       = 4
		workers   = 3 // per replica
		nReplicas = 2
		nShards   = 2
		flows     = 24
	)

	// Initialize the microservice under test
	svc := NewService()
	svc.SetCap(cap)

	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	// Two foreman replicas, same DSN and NumShards. In TESTING the plane is
	// derived from the test name (same across replicas), and OpenTesting keys
	// the in-memory SQLite by (plane, shard), so both replicas hit the same
	// underlying DB per shard. cache=shared lets concurrent sql.DB pools see
	// each other's writes.
	dsn := "file:dbpf%d?mode=memory&cache=shared"
	fm1 := foreman.NewService().Init(func(f *foreman.Service) error {
		f.SetSQLDataSourceName(dsn)
		f.SetNumShards(nShards)
		f.SetWorkers(workers)
		f.SetSQLConnectionPool(workers)
		return nil
	})
	fm2 := foreman.NewService().Init(func(f *foreman.Service) error {
		f.SetSQLDataSourceName(dsn)
		f.SetNumShards(nShards)
		f.SetWorkers(workers)
		f.SetSQLConnectionPool(workers)
		return nil
	})

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		fm1,
		fm2,
		tester,
	)
	app.RunInTest(t)

	t.Run("multi_replica_multi_shard_backpressure", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKeys := make([]string, 0, flows)
		for i := range flows {
			fk, err := fm.Create(ctx, distributedbackpressureflowapi.DistributedBackpressure.URL(),
				map[string]any{"tag": "f" + strconv.Itoa(i)}, nil)
			if !assert.NoError(err) {
				return
			}
			if err := fm.Start(ctx, fk); !assert.NoError(err) {
				return
			}
			flowKeys = append(flowKeys, fk)
		}

		// Every flow must complete: 429s bounce steps, not flows.
		for _, fk := range flowKeys {
			outcome, err := fm.Await(ctx, fk)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.Expect(status, workflow.StatusCompleted)
		}

		peak, rejections, completions := svc.Observed()
		t.Logf("peak=%d cap=%d rejections=%d completions=%d (workers=%d, replicas=%d, shards=%d)",
			peak, cap, rejections, completions, workers, nReplicas, nShards)

		// Every flow's Bounded task must have completed exactly once.
		assert.Expect(completions, flows)

		// With two replicas trying to dispatch against a shared cap, at least
		// one 429 must have fired. Otherwise the test never exercised the
		// distributed backpressure path.
		assert.True(rejections >= 1)

		// Hard cap on peak in-flight: each replica can dispatch at most
		// `workers` concurrent steps, so the cluster cannot exceed
		// `workers * nReplicas` regardless of how badly the adaptive limit
		// drifts. This is the natural ceiling the worker pool imposes.
		assert.True(peak <= workers*nReplicas)

		// Across both shards, the flows must have distributed across shards.
		// Top-level Create picks a shard at random; with 24 flows and 2 shards
		// the chance both shards see work is overwhelming, so the assertion
		// is exact rather than probabilistic.
		shards := map[int]bool{}
		for _, fk := range flowKeys {
			parts := strings.SplitN(fk, "-", 2)
			if len(parts) != 2 {
				continue
			}
			shardIdx, err := strconv.Atoi(parts[0])
			if err == nil {
				shards[shardIdx] = true
			}
		}
		assert.Expect(len(shards), nShards)

		// Cross-replica gossip: a cut on one replica must propagate to the
		// other via SyncValve. Both replicas must carry the taskValves
		// entry for Bounded by end of run. This catches the "SyncValve
		// is a no-op" regression that the rest of the assertions cannot.
		assert.True(fm1.ValveCount() >= 1)
		assert.True(fm2.ValveCount() >= 1)
	})
}
