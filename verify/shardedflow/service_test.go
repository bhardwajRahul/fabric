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

package shardedflow

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
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

	"github.com/microbus-io/fabric/verify/shardedflow/shardedflowapi"
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
	_ shardedflowapi.Client
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

func TestShardedflow_Sharded(t *testing.T) { // MARKER: Sharded
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
		// 8 shards + 1 worker: top-level Create scatters flows randomly across
		// the 8 shards, so correctness here proves the refiller aggregates all
		// shards into one global population (not per-shard round-robin). The
		// %d DSN is required when NumShards>1; in TESTING it is still an
		// isolated in-memory SQLite per (shard, test). SetConfig only in .Init.
		foreman.NewService().Init(func(f *foreman.Service) error {
			f.SetNumShards(8)
			f.SetWorkers(1)
			f.SetSQLConnectionPool(1)
			// Override any operator-local config so the test always runs against
			// an isolated in-memory SQLite per (shard, test).
			f.SetSQLDataSourceName("file:shard_%d.local.sqlite")
			return nil
		}),
		tester,
	)
	app.RunInTest(t)

	startFlow := func(assert *testarossa.Asserter, tag string, priority int, delayMs int, key string, weight float64) string {
		flowKey, err := fm.Create(ctx, shardedflowapi.Sharded.URL(),
			map[string]any{"tag": tag, "delayMs": delayMs},
			&workflow.FlowOptions{Priority: priority, FairnessKey: key, FairnessWeight: weight},
		)
		if !assert.NoError(err) {
			return ""
		}
		err = fm.Start(ctx, flowKey)
		assert.NoError(err)
		return flowKey
	}
	awaitAll := func(assert *testarossa.Asserter, keys []string) {
		for _, k := range keys {
			outcome, err := fm.Await(ctx, k)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.True(status == workflow.StatusCompleted)
		}
	}

	t.Run("strict_priority_across_shards", func(t *testing.T) {
		assert := testarossa.For(t)
		svc.mu.Lock()
		svc.order = nil
		svc.mu.Unlock()

		// Priority-1 holder occupies the lone worker through creation; the 8
		// test flows have distinct priorities 2..9 and scatter across 8 shards.
		// Distinct priority per flow => global-min selection is fully
		// deterministic regardless of shard or created_at ties.
		holder := startFlow(assert, "_holder", 1, 1500, "", 1)
		var keys []string
		for p := 2; p <= 9; p++ {
			keys = append(keys, startFlow(assert, "p"+strconv.Itoa(p), p, 50, "", 1))
		}
		time.Sleep(300 * time.Millisecond)

		outcome, err := fm.Await(ctx, holder)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
		awaitAll(assert, keys)

		order := svc.Order()
		if assert.True(len(order) >= 1) {
			assert.Expect(order[0], "_holder")
		}
		assert.Expect(order[1:], []string{"p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9"})
	})

	t.Run("starvation_across_shards", func(t *testing.T) {
		assert := testarossa.For(t)
		svc.mu.Lock()
		svc.order = nil
		svc.mu.Unlock()

		holder := startFlow(assert, "_holder", 1, 1500, "", 1)
		low := startFlow(assert, "low", 9, 50, "", 1)
		var highs []string
		for i := 0; i < 8; i++ {
			highs = append(highs, startFlow(assert, "h"+strconv.Itoa(i), 2, 50, "", 1))
		}
		time.Sleep(300 * time.Millisecond)

		outcome, err := fm.Await(ctx, holder)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
		awaitAll(assert, append(highs, low))

		order := svc.Order()
		// Cross-band strict priority across shards: holder first, then all 8
		// priority-2 highs (in some order), then the priority-9 low last.
		if assert.Expect(len(order), 10) {
			assert.Expect(order[0], "_holder")
			assert.Expect(order[9], "low")
			mid := map[string]bool{}
			for _, t := range order[1:9] {
				mid[t] = true
			}
			assert.Expect(len(mid), 8)
			for i := 0; i < 8; i++ {
				assert.True(mid["h"+strconv.Itoa(i)])
			}
		}
	})

	t.Run("weighted_fairness_over_keys_not_shards", func(t *testing.T) {
		assert := testarossa.For(t)
		svc.mu.Lock()
		svc.order = nil
		svc.mu.Unlock()

		const n = 40
		holder := startFlow(assert, "_holder", 1, 1500, "_holder", 1)
		var keys []string
		for i := 0; i < n; i++ {
			// Default priority (0 -> DefaultPriority): same band, so only the
			// weighted key pick orders them. Flows scatter across 8 shards.
			keys = append(keys, startFlow(assert, "heavy:"+strconv.Itoa(i), 0, 8, "heavy", 4))
			keys = append(keys, startFlow(assert, "light:"+strconv.Itoa(i), 0, 8, "light", 1))
		}
		time.Sleep(400 * time.Millisecond)

		outcome, err := fm.Await(ctx, holder)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
		awaitAll(assert, keys)

		order := svc.Order()
		if len(order) > 0 && order[0] == "_holder" {
			order = order[1:]
		}
		var heavyTotal, lightTotal int
		for _, tag := range order {
			if strings.HasPrefix(tag, "heavy:") {
				heavyTotal++
			} else if strings.HasPrefix(tag, "light:") {
				lightTotal++
			}
		}
		assert.Expect(heavyTotal, n) // liveness
		assert.Expect(lightTotal, n)

		// Weighted share over the contended prefix must reflect the 4:1 weight
		// across keys despite the flows being scattered over 8 shards (the old
		// per-shard round-robin would skew this toward ~1:1).
		var ph, pl int
		for _, tag := range order[:n] {
			if strings.HasPrefix(tag, "heavy:") {
				ph++
			} else {
				pl++
			}
		}
		assert.True(pl >= 1)                        // light not starved
		assert.True(float64(ph) >= 0.55*float64(n)) // heavy is the clear majority (expected ~0.8)
	})

	t.Run("random_shard_distribution", func(t *testing.T) {
		assert := testarossa.For(t)
		// Create-time sharding is rand.IntN(NumShards)+1 (1-based). Create (do NOT
		// Start, so the flows stay inert in 'created' - no dispatch, no drain) and
		// parse the shard from the flowKey prefix ("{shard}-{flowID}-{token}").
		// Shards are 1-indexed; counts[0] stays unused.
		const m = 400
		counts := make([]int, 9)
		for i := 0; i < m; i++ {
			flowKey, err := fm.Create(ctx, shardedflowapi.Sharded.URL(),
				map[string]any{"tag": "d", "delayMs": 0}, nil)
			if !assert.NoError(err) {
				return
			}
			shard, err := strconv.Atoi(strings.SplitN(flowKey, "-", 2)[0])
			if !assert.NoError(err) {
				return
			}
			if assert.True(shard >= 1 && shard <= 8) {
				counts[shard]++
			}
		}
		// Approximately uniform over 8 shards (mean 50, std ~6.6). Generous
		// ~5-sigma bands: effectively flake-free, but a gross skew (e.g. all on
		// one shard) still fails.
		mean := m / 8
		for s := 1; s <= 8; s++ {
			c := counts[s]
			assert.True(c > 0)       // every shard is used
			assert.True(c >= mean/3) // ~uniform lower bound (~16)
			assert.True(c <= mean*3) // ~uniform upper bound (~150)
		}
	})

	t.Run("fifo_within_fairness_key", func(t *testing.T) {
		assert := testarossa.For(t)
		svc.mu.Lock()
		svc.order = nil
		svc.mu.Unlock()

		// Multiple fairness keys, all at one priority band, created in random
		// interleave. The weighted key pick randomizes the order *across* keys,
		// so the global recorded order is not creation order - but within each
		// key the refiller always takes that key's oldest pending step, so the
		// recorded subsequence of any one key must equal that key's creation
		// order (FIFO per key). Flows are created >1 DATETIME(3) tick apart and
		// scatter randomly over the 8 shards, so a pass proves cross-shard FIFO
		// by per-shard ageMs; a (shard, step_id) order would interleave by shard
		// and fail this. A priority-1 holder occupies the lone worker through
		// the whole creation window so nothing drains until all are pending.
		fkeys := []string{"alpha", "beta", "gamma", "delta"}
		const perKey = 8

		// Build the creation schedule (perKey copies of each key) and shuffle
		// it so keys are created in random interleave.
		sched := make([]string, 0, len(fkeys)*perKey)
		for _, k := range fkeys {
			for i := 0; i < perKey; i++ {
				sched = append(sched, k)
			}
		}
		rand.Shuffle(len(sched), func(i, j int) { sched[i], sched[j] = sched[j], sched[i] })

		holder := startFlow(assert, "_holder", 1, 2500, "_h", 1)
		expected := map[string][]string{}
		next := map[string]int{}
		var keys []string
		for _, k := range sched {
			seq := next[k]
			next[k]++
			tag := k + ":" + strconv.Itoa(seq)
			expected[k] = append(expected[k], tag)
			keys = append(keys, startFlow(assert, tag, 5, 5, k, 1))
			time.Sleep(12 * time.Millisecond) // exceed DATETIME(3) resolution
		}

		outcome, err := fm.Await(ctx, holder)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
		awaitAll(assert, keys)

		order := svc.Order()
		if assert.Expect(len(order), len(sched)+1) {
			assert.Expect(order[0], "_holder")
			// Project the global order onto each key; each projection must be
			// that key's creation order.
			got := map[string][]string{}
			for _, tag := range order[1:] {
				k := strings.SplitN(tag, ":", 2)[0]
				got[k] = append(got[k], tag)
			}
			for _, k := range fkeys {
				assert.Expect(got[k], expected[k])
			}
		}
	})
}
