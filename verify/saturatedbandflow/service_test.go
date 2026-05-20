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

package saturatedbandflow

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

	"github.com/microbus-io/fabric/verify/saturatedbandflow/saturatedbandflowapi"
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

func TestSaturatedbandflow_BandFallthrough(t *testing.T) { // MARKER: SaturatedBand
	t.Parallel()
	ctx := t.Context()

	const (
		boundedCap   = 2  // task self-limits at 2 concurrent
		workers      = 6  // pool larger than cap so saturation provokes 429s
		nSat         = 8  // SaturatedBand (high-pri) flows
		nOpen        = 8  // OpenBand (low-pri) flows
		priorityHigh = 1
		priorityLow  = 5
	)

	// Initialize the microservice under test
	svc := NewService()
	svc.SetBoundedCap(boundedCap)

	// Initialize the testers
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error {
			f.SetWorkers(workers)
			f.SetSQLConnectionPool(workers)
			return nil
		}),
		tester,
	)
	app.RunInTest(t)

	t.Run("low_priority_advances_while_high_priority_saturated", func(t *testing.T) {
		assert := testarossa.For(t)

		satKeys := make([]string, 0, nSat)
		openKeys := make([]string, 0, nOpen)

		// Create + start nSat high-priority SaturatedBand flows first so they
		// dominate the pending queue and the refiller sees them before any
		// OpenBand work.
		for i := range nSat {
			tag := "sat:" + strconv.Itoa(i)
			fk, err := fm.Create(ctx, saturatedbandflowapi.SaturatedBand.URL(),
				map[string]any{"tag": tag},
				&workflow.FlowOptions{Priority: priorityHigh},
			)
			if !assert.NoError(err) {
				return
			}
			if err := fm.Start(ctx, fk); !assert.NoError(err) {
				return
			}
			satKeys = append(satKeys, fk)
		}
		// Then create + start nOpen low-priority OpenBand flows.
		for i := range nOpen {
			tag := "open:" + strconv.Itoa(i)
			fk, err := fm.Create(ctx, saturatedbandflowapi.OpenBand.URL(),
				map[string]any{"tag": tag},
				&workflow.FlowOptions{Priority: priorityLow},
			)
			if !assert.NoError(err) {
				return
			}
			if err := fm.Start(ctx, fk); !assert.NoError(err) {
				return
			}
			openKeys = append(openKeys, fk)
		}

		// All flows must complete; a 429 bounces the step, not the flow.
		for _, fk := range satKeys {
			outcome, err := fm.Await(ctx, fk)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.Expect(status, workflow.StatusCompleted)
		}
		for _, fk := range openKeys {
			outcome, err := fm.Await(ctx, fk)

			status := outcomeStatus(outcome)
			assert.NoError(err)
			assert.Expect(status, workflow.StatusCompleted)
		}

		rejections := svc.Rejections()
		completions := svc.Completions()
		assert.Expect(len(completions), nSat+nOpen)
		t.Logf("rejections=%d completions=%d", rejections, len(completions))

		// The Bounded task must have actually 429'd; otherwise nothing
		// saturated the high band and the test is meaningless.
		assert.True(rejections >= 1)

		// Locate the earliest Open completion and the latest Sat completion.
		// Band fallthrough means the foreman admitted Open work while Sat work
		// was still in flight, so the earliest Open finishes strictly before
		// the last Sat.
		var firstOpenAt, lastSatAt int64
		firstOpenAt = -1
		for _, c := range completions {
			switch {
			case strings.HasPrefix(c.tag, "open:"):
				ns := c.at.UnixNano()
				if firstOpenAt < 0 || ns < firstOpenAt {
					firstOpenAt = ns
				}
			case strings.HasPrefix(c.tag, "sat:"):
				ns := c.at.UnixNano()
				if ns > lastSatAt {
					lastSatAt = ns
				}
			}
		}
		assert.True(firstOpenAt > 0)
		assert.True(lastSatAt > 0)
		// The load-bearing assertion: some open flow completed before all
		// sat flows did. Under strict priority without fallthrough this is
		// impossible (every sat step would have to complete before any open
		// step could be dispatched).
		assert.True(firstOpenAt < lastSatAt)
	})
}
