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

package foreman

import (
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// timeRef is a fixed origin for FlowStep timestamps in renderer tests; the
// concrete value is unimportant since assertions read relative offsets.
var timeRef = time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

// edge returns the exact Mermaid edge line the renderer emits for step ids.
func edge(from, to int) string {
	return "    s" + itoa(from) + " --> s" + itoa(to) + "\n"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestForeman_RenderMermaidSteps_EdgeModel verifies the history renderer wires edges
// purely from PredecessorID/SuccessorID (the execution DAG), never step_depth - so
// parallel forEach branches and parallel inner fan-ins do not cross-connect.
func TestForeman_RenderMermaidSteps_EdgeModel(t *testing.T) {
	t.Parallel()

	t.Run("per_element_pipeline", func(t *testing.T) {
		assert := testarossa.For(t)

		// S(1) -forEach-> H1(10),H2(11),H3(12); each Hk -> Ak,Bk; Ak,Bk -fanin-> Mk;
		// M1(30),M2(31),M3(32) -fanin-> L(40).
		steps := []foremanapi.FlowStep{
			{StepID: 1, TaskName: "s", SuccessorID: 10},
			{StepID: 10, TaskName: "h", PredecessorID: 1, SuccessorID: 20},
			{StepID: 11, TaskName: "h", PredecessorID: 1, SuccessorID: 22},
			{StepID: 12, TaskName: "h", PredecessorID: 1, SuccessorID: 24},
			{StepID: 20, TaskName: "a", PredecessorID: 10, SuccessorID: 30},
			{StepID: 21, TaskName: "b", PredecessorID: 10, SuccessorID: 30},
			{StepID: 22, TaskName: "a", PredecessorID: 11, SuccessorID: 31},
			{StepID: 23, TaskName: "b", PredecessorID: 11, SuccessorID: 31},
			{StepID: 24, TaskName: "a", PredecessorID: 12, SuccessorID: 32},
			{StepID: 25, TaskName: "b", PredecessorID: 12, SuccessorID: 32},
			{StepID: 30, TaskName: "m", PredecessorID: 21, SuccessorID: 40},
			{StepID: 31, TaskName: "m", PredecessorID: 23, SuccessorID: 40},
			{StepID: 32, TaskName: "m", PredecessorID: 25, SuccessorID: 40},
			{StepID: 40, TaskName: "l", PredecessorID: 32},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Each H connects only to its own A and B.
		for _, e := range []string{
			edge(10, 20), edge(10, 21), edge(11, 22), edge(11, 23), edge(12, 24), edge(12, 25),
			edge(20, 30), edge(21, 30), edge(22, 31), edge(23, 31), edge(24, 32), edge(25, 32),
			edge(30, 40), edge(31, 40), edge(32, 40),
		} {
			assert.Expect(strings.Contains(out, e), true)
		}
		// The reported bug: an H must NOT point at another element's A/B, and an
		// inner fan-in must NOT pull a sibling element's branches.
		for _, e := range []string{
			edge(10, 22), edge(10, 24), edge(11, 20), edge(12, 20),
			edge(20, 31), edge(22, 30), edge(24, 30),
			edge(20, 40), edge(21, 40),
		} {
			assert.Expect(strings.Contains(out, e), false)
		}
	})

	t.Run("foreach_over_sequential_chain", func(t *testing.T) {
		assert := testarossa.For(t)

		// S(1) -> ELEMS(2) -forEach-> {A->B->C} -fanin-> J(40).
		// element 1: A10->B20->C30; element 2: A11->B21->C31; element 3: A12->B22->C32.
		steps := []foremanapi.FlowStep{
			{StepID: 1, TaskName: "s", SuccessorID: 2},
			{StepID: 2, TaskName: "elems", PredecessorID: 1, SuccessorID: 10},
			{StepID: 10, TaskName: "a", PredecessorID: 2, SuccessorID: 20},
			{StepID: 11, TaskName: "a", PredecessorID: 2, SuccessorID: 21},
			{StepID: 12, TaskName: "a", PredecessorID: 2, SuccessorID: 22},
			{StepID: 20, TaskName: "b", PredecessorID: 10, SuccessorID: 30},
			{StepID: 21, TaskName: "b", PredecessorID: 11, SuccessorID: 31},
			{StepID: 22, TaskName: "b", PredecessorID: 12, SuccessorID: 32},
			{StepID: 30, TaskName: "c", PredecessorID: 20, SuccessorID: 40},
			{StepID: 31, TaskName: "c", PredecessorID: 21, SuccessorID: 40},
			{StepID: 32, TaskName: "c", PredecessorID: 22, SuccessorID: 40},
			{StepID: 40, TaskName: "j", PredecessorID: 32},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Only the C's feed J; the chain stays per element.
		for _, e := range []string{
			edge(2, 10), edge(2, 11), edge(2, 12),
			edge(10, 20), edge(20, 30), edge(11, 21), edge(21, 31), edge(12, 22), edge(22, 32),
			edge(30, 40), edge(31, 40), edge(32, 40),
		} {
			assert.Expect(strings.Contains(out, e), true)
		}
		// A and B must never connect to the fan-in, and chains must not cross.
		for _, e := range []string{
			edge(10, 40), edge(20, 40), edge(11, 40), edge(21, 40),
			edge(10, 21), edge(11, 20), edge(20, 31),
		} {
			assert.Expect(strings.Contains(out, e), false)
		}
	})
}

// TestForeman_RenderMermaidSteps_Wrappers verifies that child-workflow (Subgraph) steps get
// wrapped in a visible Mermaid subgraph block, that fan-out cohorts get wrapped in an invisible
// one (layout container only — no label, no fill, no stroke), and that edges still go between
// actual task node IDs rather than block IDs.
func TestForeman_RenderMermaidSteps_Wrappers(t *testing.T) {
	t.Parallel()

	t.Run("fan_out_cohort_invisible_wrapper", func(t *testing.T) {
		assert := testarossa.For(t)

		// S(1) fans out to {A(10), B(11)} which join at J(20).
		steps := []foremanapi.FlowStep{
			{StepID: 1, TaskName: "s", SuccessorID: 10, Status: "completed"},
			{StepID: 10, TaskName: "a", PredecessorID: 1, SuccessorID: 20, Status: "completed"},
			{StepID: 11, TaskName: "b", PredecessorID: 1, SuccessorID: 20, Status: "completed"},
			{StepID: 20, TaskName: "j", PredecessorID: 11, Status: "completed"},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Cohort wrapper exists for Mermaid layout, but is invisible: empty label, no fill,
		// no stroke. No "fan-out" / "for each" labels.
		assert.Expect(strings.Contains(out, `subgraph fo_s1 [" "]`), true)
		assert.Expect(strings.Contains(out, "style fo_s1 fill:none,stroke:none"), true)
		assert.Expect(strings.Contains(out, `"fan-out"`), false)
		assert.Expect(strings.Contains(out, `"for each"`), false)

		// Edges still go between actual task nodes; nothing terminates at fo_s1.
		assert.Expect(strings.Contains(out, edge(1, 10)), true)
		assert.Expect(strings.Contains(out, edge(1, 11)), true)
		assert.Expect(strings.Contains(out, edge(10, 20)), true)
		assert.Expect(strings.Contains(out, edge(11, 20)), true)
		assert.Expect(strings.Contains(out, "--> fo_s1"), false)
		assert.Expect(strings.Contains(out, "fo_s1 -->"), false)
	})

	t.Run("subgraph_caller_node_net_duration", func(t *testing.T) {
		assert := testarossa.For(t)

		// S(1) -> caller(10) [calls subgraph wrapping inner(100)] -> T(20). Times chosen so
		// the net caller cost is computable:
		//   s1: 50ms body
		//   caller (subgraph): started t0+60ms, finished t0+2000ms => total 1940ms call cost
		//     inner: created t0+70ms, finished t0+1800ms => subgraph wall time = 1730ms
		//   => net caller cost = 1940ms - 1730ms = 210ms
		//   s20: 150ms body
		// Edge waits:
		//   s1->s10: s10.StartedAt - s1.UpdatedAt = 60ms - 50ms = 10ms
		//   s10->s20 (visually innerTail->s20): s20.StartedAt - s10.UpdatedAt = 2050ms - 2000ms = 50ms
		t0 := timeRef
		ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
		steps := []foremanapi.FlowStep{
			{StepID: 1, TaskName: "s", SuccessorID: 10, Status: "completed", CreatedAt: t0, StartedAt: t0, UpdatedAt: t0.Add(ms(50))},
			{StepID: 10, TaskName: "callChildWorkflow", PredecessorID: 1, SuccessorID: 20, Subgraph: true, SubWorkflowName: "https://planner.agent/main", Status: "completed",
				CreatedAt: t0.Add(ms(50)), StartedAt: t0.Add(ms(60)), UpdatedAt: t0.Add(ms(2000)),
				SubHistory: []foremanapi.FlowStep{
					{StepID: 100, TaskName: "inner", Status: "completed", CreatedAt: t0.Add(ms(70)), StartedAt: t0.Add(ms(100)), UpdatedAt: t0.Add(ms(1800))},
				}},
			{StepID: 20, TaskName: "t", PredecessorID: 10, Status: "completed", CreatedAt: t0.Add(ms(2000)), StartedAt: t0.Add(ms(2050)), UpdatedAt: t0.Add(ms(2200))},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Per-step durations on task nodes: updated_at - started_at.
		assert.Expect(strings.Contains(out, "s1[\"s\n50ms\"]"), true)
		assert.Expect(strings.Contains(out, "s10_subs100[\"inner\n1.7s\"]"), true)
		assert.Expect(strings.Contains(out, "s20[\"t\n150ms\"]"), true)
		// Caller node shows NET duration = total call cost - subgraph wall time = 1940-1730 = 210ms.
		assert.Expect(strings.Contains(out, "s10[\"callChildWorkflow\n210ms\"]"), true)
		// Wrapper block is labelled with the child workflow URL (proto stripped).
		assert.Expect(strings.Contains(out, `subgraph s10_sg ["planner.agent/main"]`), true)
		assert.Expect(strings.Contains(out, "style s10_sg fill:#32a7c1,fill-opacity:0.03"), true)
		// No return circle - the diagram is just caller + block, with edges threading through.
		assert.Expect(strings.Contains(out, "s10_ret"), false)

		// Plain-node edges. Call edge labeled with the subgraph entry's queue wait
		// (entry.StartedAt - entry.CreatedAt = 100 - 70 = 30ms). Return edge labeled with
		// the transition gap (Y.StartedAt - caller.UpdatedAt = 50ms) even though it visually
		// originates from the inner tail: addEdge reads the label from the step records.
		assert.Expect(strings.Contains(out, `    s1 -- "10ms" --> s10`+"\n"), true)
		assert.Expect(strings.Contains(out, `    s10 -- "30ms" --> s10_subs100`+"\n"), true)
		assert.Expect(strings.Contains(out, `    s10_subs100 -- "50ms" --> s20`+"\n"), true)
		// No direct caller -> successor edge.
		assert.Expect(strings.Contains(out, "    s10 --> s20\n"), false)
	})

	t.Run("subgraph_caller_with_return_fanout", func(t *testing.T) {
		assert := testarossa.For(t)

		// caller(10) calls subgraph (single inner) and its transitions fan out to two siblings
		// T(20) and U(21). Without the return circle, each inner tail fans out directly to each
		// successor: innerTail -> branchA, innerTail -> branchB. For a single inner tail and N
		// successors that's N edges (same as the previous "circle fan-out" form); for M inner
		// tails and N successors it's M x N. Acceptable cost for keeping the diagram simple.
		steps := []foremanapi.FlowStep{
			{StepID: 10, TaskName: "caller", SuccessorID: 20, Subgraph: true, SubWorkflowName: "https://child.agent/flow", Status: "completed", SubHistory: []foremanapi.FlowStep{
				{StepID: 100, TaskName: "inner", Status: "completed"},
			}},
			{StepID: 20, TaskName: "branchA", PredecessorID: 10, Status: "completed"},
			{StepID: 21, TaskName: "branchB", PredecessorID: 10, Status: "completed"},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Call edge into the subgraph.
		assert.Expect(strings.Contains(out, "    s10 --> s10_subs100\n"), true)
		// Inner tail fans out directly to each parent-flow successor (no timestamps in this
		// fixture, so edge labels are empty - just verifying the structural routing).
		assert.Expect(strings.Contains(out, "    s10_subs100 --> s20\n"), true)
		assert.Expect(strings.Contains(out, "    s10_subs100 --> s21\n"), true)
		// Caller itself does NOT directly connect to either branch.
		assert.Expect(strings.Contains(out, "    s10 --> s20\n"), false)
		assert.Expect(strings.Contains(out, "    s10 --> s21\n"), false)
		// No return circle in the diagram.
		assert.Expect(strings.Contains(out, "s10_ret"), false)
	})

	t.Run("subgraph_caller_in_flight_no_duration_label", func(t *testing.T) {
		assert := testarossa.For(t)

		// caller(10) is still running (parked on the subgraph). The net caller cost can't be
		// computed yet because the subgraph wall time isn't finalized, so the caller node
		// renders with just the task name and no duration line.
		t0 := timeRef
		ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
		steps := []foremanapi.FlowStep{
			{StepID: 10, TaskName: "caller", Subgraph: true, SubWorkflowName: "https://child.agent/flow", Status: "running",
				CreatedAt: t0, StartedAt: t0.Add(ms(10)), UpdatedAt: t0.Add(ms(500)),
				SubHistory: []foremanapi.FlowStep{
					{StepID: 100, TaskName: "inner", Status: "running", CreatedAt: t0.Add(ms(100)), StartedAt: t0.Add(ms(120)), UpdatedAt: t0.Add(ms(500))},
				}},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Caller renders without a duration line.
		assert.Expect(strings.Contains(out, `s10["caller"]`), true)
		assert.Expect(strings.Contains(out, "s10[\"caller\n"), false)
		// No return circle.
		assert.Expect(strings.Contains(out, "s10_ret"), false)
	})

	t.Run("pending_step_omits_duration", func(t *testing.T) {
		assert := testarossa.For(t)

		// A pending step has StartedAt defaulted by NOW_UTC() at INSERT, so its UpdatedAt -
		// StartedAt is misleading. HasStarted gates the label so pending/created steps render
		// just the task name.
		t0 := timeRef
		ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
		steps := []foremanapi.FlowStep{
			{StepID: 1, TaskName: "ready", SuccessorID: 2, Status: "pending", CreatedAt: t0, StartedAt: t0.Add(ms(50)), UpdatedAt: t0.Add(ms(50))},
			{StepID: 2, TaskName: "waitingForLease", PredecessorID: 1, Status: "pending", CreatedAt: t0.Add(ms(10)), StartedAt: t0.Add(ms(60)), UpdatedAt: t0.Add(ms(60))},
		}
		out, err := foremanapi.NewFlowRenderer(steps).Render()
		assert.Expect(err, nil)

		// Neither pending step shows a duration line under its task name.
		assert.Expect(strings.Contains(out, `s1["ready"]`), true)
		assert.Expect(strings.Contains(out, "s1[\"ready\n"), false)
		assert.Expect(strings.Contains(out, `s2["waitingForLease"]`), true)
		assert.Expect(strings.Contains(out, "s2[\"waitingForLease\n"), false)
	})
}
