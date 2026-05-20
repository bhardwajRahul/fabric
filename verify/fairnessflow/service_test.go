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

package fairnessflow

import (
	"context"
	"io"
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

	"github.com/microbus-io/fabric/verify/fairnessflow/fairnessflowapi"
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
	_ fairnessflowapi.Client
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

func TestFairnessflow_Fairness(t *testing.T) { // MARKER: Fairness
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
		// Pin the foreman to a single worker so dispatch is strictly serialized
		// and the weighted key pick is observable as the dispatch order.
		foreman.NewService().Init(func(f *foreman.Service) error {
			f.SetWorkers(1)
			f.SetSQLConnectionPool(1)
			return nil
		}),
		tester,
	)
	app.RunInTest(t)

	// startFlow creates and starts one Fairness flow under the given fairness
	// key and weight. priority pins selection order: the holder uses a strictly
	// lower priority AND a long delayMs so it is picked first and genuinely
	// occupies the lone worker while the whole two-key set is created; the
	// contended set shares one priority and a short delayMs so only the
	// weighted key pick orders it.
	startFlow := func(assert *testarossa.Asserter, tag string, key string, weight float64, priority int, delayMs int) string {
		flowKey, err := fm.Create(ctx, fairnessflowapi.Fairness.URL(),
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

	t.Run("weighted_share_and_liveness", func(t *testing.T) {
		assert := testarossa.For(t)

		const n = 40
		// Priority 1 + a 1500ms delay: the holder is the global minimum so it is
		// selected first and stays on the lone worker through the entire creation
		// loop and the settle below, so the full two-key set (priority 5, 8ms
		// each) is provably pending before any of it is drained.
		holder := startFlow(assert, "_holder", "_holder", 1, 1, 1500)

		var keys []string
		for i := 0; i < n; i++ {
			// Interleave so neither key is uniformly earlier in step_id order.
			keys = append(keys, startFlow(assert, "heavy:"+strconv.Itoa(i), "heavy", 4, 5, 8))
			keys = append(keys, startFlow(assert, "light:"+strconv.Itoa(i), "light", 1, 5, 8))
		}

		// Settle: the holder (1500ms) is still on the worker, so this guarantees
		// the whole set is committed pending before the worker frees and drains.
		time.Sleep(400 * time.Millisecond)

		outcome, err := fm.Await(ctx, holder)


		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Expect(status, workflow.StatusCompleted)
		for _, k := range keys {
			outcome, err = fm.Await(ctx, k)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.Expect(status, workflow.StatusCompleted)
		}

		order := svc.Order()
		if len(order) > 0 && order[0] == "_holder" {
			order = order[1:]
		}

		// Liveness + intra-key FIFO are exact (not statistical).
		var heavyTotal, lightTotal int
		var heavySeq, lightSeq []int
		firstLightPos, lastHeavyPos, lastLightPos := -1, -1, -1
		for pos, tag := range order {
			idx, _ := strconv.Atoi(tag[strings.IndexByte(tag, ':')+1:])
			switch {
			case strings.HasPrefix(tag, "heavy:"):
				heavyTotal++
				heavySeq = append(heavySeq, idx)
				lastHeavyPos = pos
			case strings.HasPrefix(tag, "light:"):
				lightTotal++
				lightSeq = append(lightSeq, idx)
				if firstLightPos < 0 {
					firstLightPos = pos
				}
				lastLightPos = pos
			}
		}
		assert.Expect(heavyTotal, n)
		assert.Expect(lightTotal, n)
		assert.True(isAscending(heavySeq)) // intra-key FIFO by step_id
		assert.True(isAscending(lightSeq))

		// Weighting: the heavier key is serviced faster, so it exhausts before
		// the lighter key (its last dispatch comes earlier). Robust because the
		// 4:1 weighting makes the reverse vanishingly unlikely.
		assert.True(lastHeavyPos < lastLightPos)

		// No starvation: the light key makes progress well before the heavy key
		// is exhausted - dispatch interleaves, it is not "all heavy then all
		// light".
		assert.True(firstLightPos >= 0 && firstLightPos < lastHeavyPos)

		// Loose share: the heavy key is the clear majority over the contended
		// prefix (expected ~0.8 for 4:1; lower-bounded generously since the
		// per-step key pick is probabilistic, Efraimidis-Spirakis).
		var ph int
		for _, tag := range order[:n] {
			if strings.HasPrefix(tag, "heavy:") {
				ph++
			}
		}
		assert.True(float64(ph) >= 0.55*float64(n))
	})
}

// isAscending reports whether s is strictly increasing.
func isAscending(s []int) bool {
	for i := 1; i < len(s); i++ {
		if s[i] <= s[i-1] {
			return false
		}
	}
	return true
}
