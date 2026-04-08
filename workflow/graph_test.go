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

package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestGraph_BuilderAndMarshal(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("create-order")
	g.AddTransitionWhen("order.service/validate", "payment.service/charge", "valid == true")
	g.AddTransitionWhen("order.service/validate", "order.service/reject", "valid != true")
	g.SetReducer("messages", ReducerAppend)

	assert.Equal("create-order", g.Name())
	assert.Equal("order.service/validate", g.EntryPoint())
	assert.Equal(3, len(g.Nodes()))

	data, err := json.Marshal(g)
	assert.NoError(err)

	var restored Graph
	err = json.Unmarshal(data, &restored)
	assert.NoError(err)

	assert.Equal("create-order", restored.Name())
	assert.Equal("order.service/validate", restored.EntryPoint())
	assert.Equal(3, len(restored.Nodes()))
	assert.Equal(2, len(restored.Transitions()))
	assert.Equal("valid == true", restored.Transitions()[0].When)
	assert.Equal(ReducerAppend, restored.reducers["messages"])
}

func TestGraph_EmptyReducers(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("simple")
	g.AddTransition("svc/start", "svc/end")

	data, err := json.Marshal(g)
	assert.NoError(err)

	// Reducers should be omitted when empty
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	assert.NoError(err)
	_, ok := raw["reducers"]
	assert.False(ok)
}

func TestGraph_DefaultEntryPoint(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTask("svc/first")
	g.AddTask("svc/second")

	assert.Equal("svc/first", g.EntryPoint())
}

func TestGraph_ExplicitEntryPoint(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTask("svc/first")
	g.AddTask("svc/second")
	g.SetEntryPoint("svc/second")

	assert.Equal("svc/second", g.EntryPoint())
}

func TestGraph_AutoRegisterTasks(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTransition("svc/a", "svc/b")
	g.AddTransitionWhen("svc/b", "svc/c", "done == true")

	tasks := g.Nodes()
	assert.Equal(3, len(tasks))
	assert.Equal("svc/a", tasks[0].Name)
	assert.Equal("svc/b", tasks[1].Name)
	assert.Equal("svc/c", tasks[2].Name)
}

func TestGraph_DuplicateTaskIgnored(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTask("svc/a")
	g.AddTask("svc/a")
	g.AddTransition("svc/a", "svc/b")

	assert.Equal(2, len(g.Nodes()))
}

func TestGraph_Validate(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Valid graph
	g := NewGraph("test")
	g.AddTransition("svc/a", "svc/b")
	g.AddTransition("svc/b", END)
	assert.NoError(g.Validate())

	// Empty name
	g2 := NewGraph("")
	g2.AddTask("svc/a")
	assert.Error(g2.Validate())

	// No tasks
	g3 := NewGraph("test")
	assert.Error(g3.Validate())

	// Entry point not in task list
	g4 := NewGraph("test")
	g4.AddTask("svc/a")
	g4.SetEntryPoint("svc/missing")
	assert.Error(g4.Validate())

	// Unreachable task
	g5 := NewGraph("test")
	g5.AddTransition("svc/a", "svc/b")
	g5.AddTask("svc/c")
	assert.Error(g5.Validate())

	// Reachable via goto
	g6 := NewGraph("test")
	g6.AddTransition("svc/a", "svc/b")
	g6.AddTransition("svc/b", END)
	g6.AddTransitionGoto("svc/a", "svc/c")
	g6.AddTransition("svc/c", END)
	assert.NoError(g6.Validate())

	// No END transition
	g7 := NewGraph("test")
	g7.AddTransition("svc/a", "svc/b")
	g7.AddTransition("svc/b", "svc/a")
	assert.Error(g7.Validate())
}

func TestGraph_ValidateFanOutSiblings(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Valid: fan-out siblings converge to the same next task
	g1 := NewGraph("valid-fanout")
	g1.AddTransition("svc/start", "svc/a")
	g1.AddTransition("svc/start", "svc/b")
	g1.AddTransition("svc/a", "svc/end")
	g1.AddTransition("svc/b", "svc/end")
	g1.AddTransition("svc/end", END)
	assert.NoError(g1.Validate())

	// Invalid: fan-out siblings have different outgoing transitions
	g2 := NewGraph("divergent-fanout")
	g2.AddTransition("svc/start", "svc/a")
	g2.AddTransition("svc/start", "svc/b")
	g2.AddTransition("svc/a", "svc/x") // a → x
	g2.AddTransition("svc/b", "svc/y") // b → y (different!)
	g2.AddTransition("svc/x", END)
	g2.AddTransition("svc/y", END)
	err := g2.Validate()
	assert.Error(err)
	assert.Contains(err.Error(), "fan-out siblings")

	// Invalid: one sibling goes to END, other goes to a task
	g3 := NewGraph("mixed-end")
	g3.AddTransition("svc/start", "svc/a")
	g3.AddTransition("svc/start", "svc/b")
	g3.AddTransition("svc/a", END)        // a terminates
	g3.AddTransition("svc/b", "svc/next") // b continues
	g3.AddTransition("svc/next", END)
	assert.Error(g3.Validate())

	// Valid: conditional transitions with matching targets
	g4 := NewGraph("conditional-matching")
	g4.AddTransitionWhen("svc/start", "svc/a", "x >= 5")
	g4.AddTransitionWhen("svc/start", "svc/b", "x < 5")
	g4.AddTransition("svc/a", "svc/end")
	g4.AddTransition("svc/b", "svc/end")
	g4.AddTransition("svc/end", END)
	assert.NoError(g4.Validate())

	// Invalid: conditional transitions with diverging targets
	// (conditions are not guaranteed to be mutually exclusive)
	g4b := NewGraph("conditional-divergent")
	g4b.AddTransitionWhen("svc/start", "svc/a", "x >= 5")
	g4b.AddTransitionWhen("svc/start", "svc/b", "y >= 5")
	g4b.AddTransition("svc/a", "svc/x")
	g4b.AddTransition("svc/b", "svc/y")
	g4b.AddTransition("svc/x", END)
	g4b.AddTransition("svc/y", END)
	assert.Error(g4b.Validate())

	// Valid: forEach fan-out siblings converge to the same task
	g5 := NewGraph("foreach-fanout")
	g5.AddTransition("svc/start", "svc/a")
	g5.AddTransitionForEach("svc/start", "svc/b", "items", "item")
	g5.AddTransition("svc/a", "svc/end")
	g5.AddTransition("svc/b", "svc/end")
	g5.AddTransition("svc/end", END)
	assert.NoError(g5.Validate())

	// Invalid: forEach sibling diverges from unconditional sibling
	g6 := NewGraph("foreach-divergent")
	g6.AddTransition("svc/start", "svc/a")
	g6.AddTransitionForEach("svc/start", "svc/b", "items", "item")
	g6.AddTransition("svc/a", "svc/x")
	g6.AddTransition("svc/b", "svc/y") // different target!
	g6.AddTransition("svc/x", END)
	g6.AddTransition("svc/y", END)
	assert.Error(g6.Validate())

	// Valid: siblings have the same set of targets (multiple targets)
	g7 := NewGraph("multi-target-match")
	g7.AddTransition("svc/start", "svc/a")
	g7.AddTransition("svc/start", "svc/b")
	g7.AddTransition("svc/a", "svc/x")
	g7.AddTransition("svc/a", "svc/y")
	g7.AddTransition("svc/b", "svc/x")
	g7.AddTransition("svc/b", "svc/y")
	g7.AddTransition("svc/x", END)
	g7.AddTransition("svc/y", END)
	assert.NoError(g7.Validate())
}

func TestGraph_END(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTransition("svc/a", "svc/b")
	g.AddTransitionWhen("svc/b", END, "done == true")
	g.AddTransition("svc/b", "svc/c")
	g.AddTransition("svc/c", END)

	// END should not appear in the task list
	tasks := g.Nodes()
	assert.Equal(3, len(tasks))
	for _, task := range tasks {
		assert.NotEqual(END, task.Name)
	}

	// Graph should validate successfully
	assert.NoError(g.Validate())

	// END should appear in JSON transitions
	data, err := json.Marshal(g)
	assert.NoError(err)
	var restored Graph
	err = json.Unmarshal(data, &restored)
	assert.NoError(err)
	assert.Equal(4, len(restored.Transitions()))
}

func TestGraph_Mermaid(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("create-order")
	g.AddTransitionWhen("order.service/validate", "payment.service/charge", "valid == true")
	g.AddTransitionWhen("order.service/validate", "order.service/reject", "valid != true")
	g.AddTransition("payment.service/charge", END)
	g.AddTransition("order.service/reject", END)

	mmd := g.Mermaid()

	assert.Contains(mmd, "graph TD")
	assert.Contains(mmd, "_start(( ))")
	assert.Contains(mmd, "_end(( ))")
	assert.Contains(mmd, `"if valid == true"`)
	assert.Contains(mmd, "order.service/validate")
	assert.Contains(mmd, "payment.service/charge")
}

func TestGraph_GotoTransition(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTransition("svc/a", "svc/b")
	g.AddTransition("svc/b", END)
	g.AddTransitionGoto("svc/a", "svc/c")
	g.AddTransition("svc/c", END)

	transitions := g.Transitions()
	assert.Equal(4, len(transitions))
	assert.False(transitions[0].WithGoto) // svc/a → svc/b
	assert.False(transitions[1].WithGoto) // svc/b → END
	assert.True(transitions[2].WithGoto)  // svc/a → svc/c (goto)
	assert.False(transitions[3].WithGoto) // svc/c → END

	// Goto transitions should have a "goto" label in Mermaid
	mmd := g.Mermaid()
	assert.Contains(mmd, `"goto"`)

	// Should validate successfully
	assert.NoError(g.Validate())

	// Should round-trip through JSON
	data, err := json.Marshal(g)
	assert.NoError(err)
	var restored Graph
	err = json.Unmarshal(data, &restored)
	assert.NoError(err)
	assert.True(restored.Transitions()[2].WithGoto)
}

func TestGraph_TransitionNoWhen(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTransition("a", "b")

	data, err := json.Marshal(g)
	assert.NoError(err)

	// When should be omitted in JSON
	var raw struct {
		Transitions []map[string]json.RawMessage `json:"transitions"`
	}
	err = json.Unmarshal(data, &raw)
	assert.NoError(err)
	assert.Equal(1, len(raw.Transitions))
	_, ok := raw.Transitions[0]["when"]
	assert.False(ok)
}

func TestGraph_ValidateWhenExpression(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Valid expression
	g1 := NewGraph("test")
	g1.AddTransitionWhen("svc/a", "svc/b", "valid == true")
	g1.AddTransitionWhen("svc/a", "svc/c", "score > 5 && !guest")
	g1.AddTransition("svc/b", END)
	g1.AddTransition("svc/c", END)
	assert.NoError(g1.Validate())

	// Invalid expression
	g2 := NewGraph("test")
	g2.AddTransitionWhen("svc/a", "svc/b", "(((")
	g2.AddTransition("svc/b", END)
	err := g2.Validate()
	assert.Error(err)
	assert.Contains(err.Error(), "invalid 'when' expression")
}

func TestGraph_TimeBudget(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("test")
	g.AddTransition("svc/a", "svc/b")
	g.SetTimeBudget("svc/a", 30*time.Second)
	g.SetTimeBudget("svc/b", 2*time.Minute)

	assert.Equal(30*time.Second, g.TimeBudget("svc/a"))
	assert.Equal(2*time.Minute, g.TimeBudget("svc/b"))
	assert.Equal(time.Duration(0), g.TimeBudget("svc/unknown"))

	// TimeBudget should be reflected in Nodes()
	tasks := g.Nodes()
	assert.Equal(30*time.Second, tasks[0].TimeBudget)
	assert.Equal(2*time.Minute, tasks[1].TimeBudget)

	// Round-trip through JSON
	data, err := json.Marshal(g)
	assert.NoError(err)
	var restored Graph
	err = json.Unmarshal(data, &restored)
	assert.NoError(err)
	assert.Equal(30*time.Second, restored.TimeBudget("svc/a"))
	assert.Equal(2*time.Minute, restored.TimeBudget("svc/b"))
}

func TestGraph_EmptyTimeBudgets(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	g := NewGraph("simple")
	g.AddTransition("svc/start", "svc/end")

	data, err := json.Marshal(g)
	assert.NoError(err)

	// Tasks without time budgets should omit the timeBudget field
	var raw struct {
		Tasks []map[string]json.RawMessage `json:"tasks"`
	}
	err = json.Unmarshal(data, &raw)
	assert.NoError(err)
	for _, task := range raw.Tasks {
		_, ok := task["timeBudget"]
		assert.False(ok)
	}
}
