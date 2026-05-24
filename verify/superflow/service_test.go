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

package superflow

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

	"github.com/microbus-io/fabric/verify/superflow/superflowapi"
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
	_ superflowapi.Client
)

// harness bundles the per-shape app and tester that subtests share.
type harness struct {
	svc *Service
	fm  foremanapi.Client
	ctx context.Context
}

// newHarness builds and starts an application containing the superflow service,
// a foreman pinned to numShards, and a tester. Subtests reset visit counters
// between runs.

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

func newHarness(t *testing.T, numShards int) *harness {
	t.Helper()

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
		foreman.NewService().Init(func(f *foreman.Service) error {
			f.SetNumShards(numShards)
			f.SetSQLConnectionPool(1)
			// Override any operator-local config so the test always runs against
			// an isolated in-memory SQLite per (shard, test).
			f.SetSQLDataSourceName("file:shard_%d.local.sqlite")
			return nil
		}),
		tester,
	)
	app.RunInTest(t)

	return &harness{svc: svc, fm: fm, ctx: t.Context()}
}

// runFlow creates, starts, and awaits one Super flow with the given state,
// then returns the per-task visit snapshot and the flow's terminal status.
// The harness's counters are reset before the flow is created.
func (h *harness) runFlow(t *testing.T, initialState map[string]any) (visits map[string]int64, finalState map[string]any, status string) {
	t.Helper()
	h.svc.ResetVisitCounters()
	flowKey, err := h.fm.Create(h.ctx, superflowapi.Super.URL(), initialState, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	err = h.fm.Start(h.ctx, flowKey)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	outcome, err := h.fm.Await(h.ctx, flowKey)

	status, finalState = outcomeStatusState(outcome)
	if err != nil {
		t.Fatalf("await: %v", err)
	}
	return h.svc.AllVisits(), finalState, status
}

// runWith builds a single-shape app and runs one flow against it. Convenience
// wrapper for tests that don't reuse the harness across subtests.
func runWith(t *testing.T, numShards int, initialState map[string]any) (visits map[string]int64, finalState map[string]any, status string) {
	t.Helper()
	h := newHarness(t, numShards)
	return h.runFlow(t, initialState)
}

func TestSuperflow_Sequential(t *testing.T) {

	t.Run("happy_path_1shard", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 1, map[string]any{"items": []string{"x", "y", "z"}})
		assert.Expect(
			status, workflow.StatusCompleted,
			v["TaskA"], int64(1),
			v["TaskB"], int64(1),
			v["TaskC"], int64(3),
			v["TaskD"], int64(1),
			v["TaskE"], int64(1),
			v["TaskZ"], int64(0),
			v["ErrorHandler"], int64(0),
			v["SubTaskA"], int64(0),
			v["SubTaskB"], int64(0),
		)
	})

	t.Run("happy_path_4shards", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 4, map[string]any{"items": []string{"x", "y", "z"}})
		assert.Expect(
			status, workflow.StatusCompleted,
			v["TaskA"], int64(1),
			v["TaskB"], int64(1),
			v["TaskC"], int64(3),
			v["TaskD"], int64(1),
			v["TaskE"], int64(1),
		)
	})
}

func TestSuperflow_Subgraph(t *testing.T) {

	t.Run("subgraph_branch_1shard", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 1, map[string]any{
			"items":       []string{"x"},
			"useSubgraph": true,
		})
		assert.Expect(
			status, workflow.StatusCompleted,
			v["TaskA"], int64(1),
			v["TaskB"], int64(1),
			v["TaskC"], int64(1),
			v["TaskD"], int64(1),
			v["SubTaskA"], int64(1),
			v["SubTaskB"], int64(1),
			v["TaskE"], int64(1),
		)
	})

	t.Run("subgraph_branch_4shards", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 4, map[string]any{
			"items":       []string{"x"},
			"useSubgraph": true,
		})
		assert.Expect(
			status, workflow.StatusCompleted,
			v["SubTaskA"], int64(1),
			v["SubTaskB"], int64(1),
			v["TaskE"], int64(1),
		)
	})
}

func TestSuperflow_Goto(t *testing.T) {

	t.Run("goto_to_taskZ_1shard", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 1, map[string]any{
			"items": []string{"x"},
			"behaviors": map[string]superflowapi.TaskBehavior{
				"TaskE": {Goto: "taskZ"},
			},
		})
		assert.Expect(
			status, workflow.StatusCompleted,
			v["TaskE"], int64(1),
			v["TaskZ"], int64(1),
		)
	})
}

func TestSuperflow_OnError(t *testing.T) {

	// Sibling-cancel races OnError: when one TaskC errors, the foreman cancels its
	// in-flight siblings. Depending on timing, one or both siblings may reach
	// ErrorHandler. We assert only that the OnError path was taken (handler ran
	// at least once) and the flow recovered to completion via TaskD/E - same shape
	// as verify/fanouterrorflow.

	t.Run("forEach_branch_errors_routed_to_handler_1shard", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 1, map[string]any{
			"items": []string{"x", "y"},
			"behaviors": map[string]superflowapi.TaskBehavior{
				"TaskC": {ErrorStatus: 500},
			},
		})
		assert.Expect(status, workflow.StatusCompleted)
		assert.True(v["ErrorHandler"] >= 1, "ErrorHandler must run at least once, got %d", v["ErrorHandler"])
		assert.Expect(v["TaskD"], int64(1), v["TaskE"], int64(1))
	})

	t.Run("forEach_branch_errors_routed_to_handler_4shards", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 4, map[string]any{
			"items": []string{"x", "y"},
			"behaviors": map[string]superflowapi.TaskBehavior{
				"TaskC": {ErrorStatus: 500},
			},
		})
		assert.Expect(status, workflow.StatusCompleted)
		assert.True(v["ErrorHandler"] >= 1, "ErrorHandler must run at least once, got %d", v["ErrorHandler"])
		assert.Expect(v["TaskD"], int64(1), v["TaskE"], int64(1))
	})
}

func TestSuperflow_Sleep(t *testing.T) {

	t.Run("sleep_in_forEach_branch_1shard", func(t *testing.T) {
		assert := testarossa.For(t)
		v, _, status := runWith(t, 1, map[string]any{
			"items": []string{"x", "y", "z"},
			"behaviors": map[string]superflowapi.TaskBehavior{
				"TaskC": {SleepMs: 50},
			},
		})
		// Sleep is per-branch; fan-in still fires after all sleeps drain.
		assert.Expect(
			status, workflow.StatusCompleted,
			v["TaskC"], int64(3),
			v["TaskD"], int64(1),
		)
	})
}
