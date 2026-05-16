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

// edge returns the exact Mermaid edge line renderMermaidSteps emits for step ids.
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
		var buf strings.Builder
		renderMermaidSteps(&buf, "", steps, time.Time{})
		out := buf.String()

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
		var buf strings.Builder
		renderMermaidSteps(&buf, "", steps, time.Time{})
		out := buf.String()

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
