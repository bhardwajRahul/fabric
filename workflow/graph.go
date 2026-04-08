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
	"fmt"
	"strings"
	"time"

	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
)

// END is a pseudo-node indicating that the workflow should terminate.
// Use it as the target of a transition to mark a terminal path.
const END = "END"

// Node describes a task or subgraph node registered in a workflow graph.
type Node struct {
	Name       string
	TimeBudget time.Duration
	Subgraph   bool
}

// Transition defines a possible transition between two tasks in a workflow graph.
type Transition struct {
	From     string `json:"from"`
	To       string `json:"to"`
	When     string `json:"when,omitzero"`
	WithGoto bool   `json:"withGoto,omitzero"`
	ForEach  string `json:"forEach,omitzero"` // dynamic fan-out over a state field
	As       string `json:"as,omitzero"`      // alias for the current element during forEach fan-out
	OnError  bool   `json:"onError,omitzero"` // taken when the source task returns an error
}

// Graph is the definition of a workflow. It describes the tasks, transitions between them,
// and reducers for merging state during fan-in.
//
// Use NewGraph to create a new graph, then AddTask, AddTransition,
// AddTransitionWhen, SetEntryPoint, and SetReducer to build it up.
type Graph struct {
	name        string
	entryPoint  string
	nodes       []Node
	transitions []Transition
	reducers    map[string]Reducer
	inputs      []string // nil/[] = pass nothing, ["*"] = pass everything, ["a","b"] = named
	outputs     []string // nil/[] = pass nothing, ["*"] = pass everything, ["a","b"] = named
}

// NewGraph creates a new workflow graph with the given name.
func NewGraph(name string) *Graph {
	return &Graph{
		name: name,
	}
}

// Name returns the name of the graph.
func (g *Graph) Name() string {
	return g.name
}

// EntryPoint returns the entry point task URL of the graph.
func (g *Graph) EntryPoint() string {
	return g.entryPoint
}

// Nodes returns the list of nodes in the graph.
func (g *Graph) Nodes() []Node {
	result := make([]Node, len(g.nodes))
	copy(result, g.nodes)
	return result
}

// Transitions returns the list of transitions in the graph.
func (g *Graph) Transitions() []Transition {
	result := make([]Transition, len(g.transitions))
	copy(result, g.transitions)
	return result
}

// DeclareInputs declares which state fields are passed into this graph when used as a subgraph.
// No arguments means nothing is passed in. Use "*" to pass everything.
// Named fields pass only those fields.
func (g *Graph) DeclareInputs(fields ...string) {
	g.inputs = fields
}

// DeclareOutputs declares which state fields are returned from this graph on completion.
// No arguments means nothing is returned. Use "*" to return everything.
// Named fields return only those fields.
// Output filtering applies both when used as a subgraph and as a root flow.
func (g *Graph) DeclareOutputs(fields ...string) {
	g.outputs = fields
}

// Inputs returns the declared input fields.
func (g *Graph) Inputs() []string {
	return g.inputs
}

// Outputs returns the declared output fields.
func (g *Graph) Outputs() []string {
	return g.outputs
}

// AddTask registers a task node in the graph. The first node added becomes the default
// entry point unless SetEntryPoint is called explicitly. Duplicate nodes are ignored.
// The pseudo-node END is not registered.
func (g *Graph) AddTask(taskURL string) {
	if taskURL == END {
		return
	}
	for i := range g.nodes {
		if g.nodes[i].Name == taskURL {
			return
		}
	}
	g.nodes = append(g.nodes, Node{Name: taskURL})
	if g.entryPoint == "" {
		g.entryPoint = taskURL
	}
}

// AddSubgraph registers a child workflow as a subgraph node in the graph.
// Transitions can target a subgraph name just like a task name. When the foreman
// reaches a subgraph transition, it creates and runs the child workflow, blocking
// the parent until the child completes. Duplicate names are ignored.
func (g *Graph) AddSubgraph(workflowName string) {
	if workflowName == END {
		return
	}
	for i := range g.nodes {
		if g.nodes[i].Name == workflowName {
			g.nodes[i].Subgraph = true
			return
		}
	}
	g.nodes = append(g.nodes, Node{Name: workflowName, Subgraph: true})
}

// IsSubgraph returns true if the given name is registered as a subgraph.
func (g *Graph) IsSubgraph(name string) bool {
	for _, n := range g.nodes {
		if n.Name == name {
			return n.Subgraph
		}
	}
	return false
}

// SetEntryPoint sets the entry point of the graph explicitly, overriding the default
// (first task added).
func (g *Graph) SetEntryPoint(taskURL string) {
	g.entryPoint = taskURL
}

// AddTransition adds an unconditional transition between two nodes.
// Both endpoints are auto-registered as tasks if not already present.
func (g *Graph) AddTransition(from, to string) {
	g.AddTask(from)
	g.AddTask(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to})
}

// AddTransitionWhen adds a conditional transition between two nodes.
// The when expression is evaluated against the flow state to determine if the
// transition should be taken. Both endpoints are auto-registered as tasks if not already present.
func (g *Graph) AddTransitionWhen(from, to string, when string) {
	g.AddTask(from)
	g.AddTask(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, When: when})
}

// AddTransitionGoto adds a transition that is only taken when the task explicitly
// calls flow.Goto with the target task URL. Both endpoints are auto-registered as
// tasks if not already present.
func (g *Graph) AddTransitionGoto(from, to string) {
	g.AddTask(from)
	g.AddTask(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, WithGoto: true})
}

// AddTransitionForEach adds a dynamic fan-out transition. The foreman iterates over the
// state field named by forEach and spawns one instance of the target task per element.
// The as parameter sets the state key name for the current element (defaults to "item"
// if empty). Both endpoints are auto-registered as tasks if not already present.
// If the array is empty, no tasks are spawned. When this is the only outgoing transition
// from a task, an empty array causes the flow to complete at that point.
func (g *Graph) AddTransitionForEach(from, to string, forEach string, as string) {
	g.AddTask(from)
	g.AddTask(to)
	if as == "" {
		as = "item"
	}
	g.transitions = append(g.transitions, Transition{From: from, To: to, ForEach: forEach, As: as})
}

// AddErrorTransition adds a transition that is taken when the source task returns an error.
// The error is serialized as a TracedError into the state field "onErr" of the target task.
// Error transitions cannot be combined with When, ForEach, or WithGoto.
// Both endpoints are auto-registered as tasks if not already present.
func (g *Graph) AddErrorTransition(from, to string) {
	g.AddTask(from)
	g.AddTask(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, OnError: true})
}

// ErrorTransition returns the error transition from the given task, if one exists.
func (g *Graph) ErrorTransition(taskName string) (Transition, bool) {
	for _, tr := range g.transitions {
		if tr.From == taskName && tr.OnError {
			return tr, true
		}
	}
	return Transition{}, false
}

// SetReducer sets the merge strategy for a state field during fan-in.
func (g *Graph) SetReducer(field string, reducer Reducer) {
	if g.reducers == nil {
		g.reducers = make(map[string]Reducer)
	}
	g.reducers[field] = reducer
}

// Reducers returns the reducer map for state fields.
func (g *Graph) Reducers() map[string]Reducer {
	return g.reducers
}

// SetTimeBudget sets the execution time budget for a specific task. If not set,
// the foreman's DefaultTimeBudget config is used. The task must already be registered.
func (g *Graph) SetTimeBudget(taskURL string, budget time.Duration) {
	for i := range g.nodes {
		if g.nodes[i].Name == taskURL {
			g.nodes[i].TimeBudget = budget
			return
		}
	}
}

// TimeBudget returns the execution time budget for a specific task, or 0 if not set.
func (g *Graph) TimeBudget(taskURL string) time.Duration {
	for _, t := range g.nodes {
		if t.Name == taskURL {
			return t.TimeBudget
		}
	}
	return 0
}

// Mermaid returns a Mermaid flowchart representation of the graph.
func (g *Graph) Mermaid() string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	// Map task and subgraph names to short IDs and display labels
	ids := make(map[string]string, len(g.nodes))
	labels := make(map[string]string, len(g.nodes))
	for i, t := range g.nodes {
		ids[t.Name] = fmt.Sprintf("t%d", i)
		labels[t.Name] = strings.TrimPrefix(t.Name, "https://")
	}

	// Entry point arrow
	if ep, ok := ids[g.entryPoint]; ok {
		fmt.Fprintf(&b, "    _start(( )) --> %s[\"%s\"]\n", ep, labels[g.entryPoint])
	}

	// Group transitions by (from, to) and combine labels
	type edge struct {
		from string
		to   string
	}
	edgeOrder := []edge{}
	edgeLabels := map[edge][]string{}
	for _, tr := range g.transitions {
		e := edge{tr.From, tr.To}
		if _, ok := edgeLabels[e]; !ok {
			edgeOrder = append(edgeOrder, e)
		}
		label := ""
		if tr.When != "" {
			label = "if " + tr.When
		}
		if tr.ForEach != "" {
			forEachLabel := "for each in " + tr.ForEach
			if label != "" {
				label = forEachLabel + "; " + label
			} else {
				label = forEachLabel
			}
		}
		if tr.WithGoto {
			if label != "" {
				label = "goto; " + label
			} else {
				label = "goto"
			}
		}
		if tr.OnError {
			if label != "" {
				label = "on err; " + label
			} else {
				label = "on err"
			}
		}
		if label != "" {
			edgeLabels[e] = append(edgeLabels[e], label)
		}
	}

	// nodeShape returns the Mermaid node declaration for a given name.
	// Tasks use rectangular brackets, subgraphs use double brackets.
	nodeShape := func(name string) string {
		id := ids[name]
		lbl := labels[name]
		if g.IsSubgraph(name) {
			return fmt.Sprintf("%s[[\"%s\"]]", id, lbl)
		}
		return fmt.Sprintf("%s[\"%s\"]", id, lbl)
	}

	// Transitions
	for _, e := range edgeOrder {
		fromID := ids[e.from]
		label := strings.Join(edgeLabels[e], " | ")
		if e.to == END {
			if label != "" {
				fmt.Fprintf(&b, "    %s -->|\"%s\"| _end(( ))\n", fromID, label)
			} else {
				fmt.Fprintf(&b, "    %s --> _end(( ))\n", fromID)
			}
		} else {
			if label != "" {
				fmt.Fprintf(&b, "    %s -->|\"%s\"| %s\n", fromID, label, nodeShape(e.to))
			} else {
				fmt.Fprintf(&b, "    %s --> %s\n", fromID, nodeShape(e.to))
			}
		}
	}

	return b.String()
}

// Validate checks the graph for structural errors.
func (g *Graph) Validate() error {
	if g.name == "" {
		return errors.New("graph name is required")
	}
	if len(g.nodes) == 0 {
		return errors.New("graph '%s' has no tasks", g.name)
	}
	nodeSet := make(map[string]bool, len(g.nodes)+1)
	nodeSet[END] = true
	for _, t := range g.nodes {
		if nodeSet[t.Name] {
			return errors.New("duplicate node '%s' in graph '%s'", t.Name, g.name)
		}
		nodeSet[t.Name] = true
	}
	if !nodeSet[g.entryPoint] || g.IsSubgraph(g.entryPoint) {
		return errors.New("entry point '%s' is not a registered task in graph '%s'", g.entryPoint, g.name)
	}
	for _, tr := range g.transitions {
		if !nodeSet[tr.From] {
			return errors.New("transition from unknown node '%s' to '%s' in graph '%s'", tr.From, tr.To, g.name)
		}
		if !nodeSet[tr.To] {
			return errors.New("transition from '%s' to unknown node '%s' in graph '%s'", tr.From, tr.To, g.name)
		}
		if tr.ForEach != "" && tr.WithGoto {
			return errors.New("transition from '%s' to '%s' in graph '%s' cannot combine forEach and withGoto", tr.From, tr.To, g.name)
		}
		if tr.As != "" && tr.ForEach == "" {
			return errors.New("transition from '%s' to '%s' in graph '%s' has 'as' without 'forEach'", tr.From, tr.To, g.name)
		}
		if tr.OnError && (tr.ForEach != "" || tr.WithGoto) {
			return errors.New("transition from '%s' to '%s' in graph '%s' cannot combine onError with forEach or withGoto", tr.From, tr.To, g.name)
		}
		if tr.When != "" {
			if err := boolexp.Validate(tr.When); err != nil {
				return errors.New("transition from '%s' to '%s' in graph '%s' has invalid 'when' expression: %v", stripProto(tr.From), stripProto(tr.To), g.name, err)
			}
		}
	}

	// Check that all nodes are reachable from the entry point
	reachable := make(map[string]bool)
	queue := []string{g.entryPoint}
	reachable[g.entryPoint] = true
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, tr := range g.transitions {
			if tr.From == current && tr.To != END && !reachable[tr.To] {
				reachable[tr.To] = true
				queue = append(queue, tr.To)
			}
		}
	}
	for _, t := range g.nodes {
		if !reachable[t.Name] {
			return errors.New("node '%s' is not reachable from entry point '%s' in graph '%s'", t.Name, g.entryPoint, g.name)
		}
	}

	// Check that at least one transition targets END
	hasEnd := false
	for _, tr := range g.transitions {
		if tr.To == END {
			hasEnd = true
			break
		}
	}
	if !hasEnd {
		return errors.New("graph '%s' has no transition to END", g.name)
	}

	// Check that fan-out siblings have compatible outgoing transitions.
	// When a task has multiple non-goto transitions to different targets, those targets may
	// execute as siblings at the same step_num (conditional transitions are not guaranteed to
	// be mutually exclusive). The foreman evaluates outgoing transitions from only the last
	// sibling to complete, so all potential siblings must have the same set of non-goto
	// outgoing transition targets.
	nonGotoTargets := func(task string) map[string]bool {
		targets := make(map[string]bool)
		for _, tr := range g.transitions {
			if tr.From == task && !tr.WithGoto && !tr.OnError {
				targets[tr.To] = true
			}
		}
		return targets
	}
	for _, task := range g.nodes {
		// Collect distinct non-goto, non-error transition targets from this task (excluding END)
		var siblings []string
		seen := make(map[string]bool)
		for _, tr := range g.transitions {
			if tr.From == task.Name && !tr.WithGoto && !tr.OnError && tr.To != END && !seen[tr.To] {
				seen[tr.To] = true
				siblings = append(siblings, tr.To)
			}
		}
		if len(siblings) < 2 {
			continue // No fan-out, nothing to check
		}
		// All potential siblings must have the same non-goto outgoing transition targets
		refTargets := nonGotoTargets(siblings[0])
		for _, sibling := range siblings[1:] {
			sibTargets := nonGotoTargets(sibling)
			if !mapsEqual(refTargets, sibTargets) {
				return errors.New(
					"fan-out siblings '%s' and '%s' (from '%s') have different outgoing transitions in graph '%s'; "+
						"all siblings in a fan-out must converge to the same next task(s)",
					stripProto(siblings[0]), stripProto(sibling), stripProto(task.Name), g.name,
				)
			}
		}
	}
	return nil
}

// mapsEqual returns true if two sets (maps to bool) have the same keys.
func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// stripProto removes the "https://" prefix from a task URL for cleaner error messages.
func stripProto(s string) string {
	return strings.TrimPrefix(s, "https://")
}

// MarshalJSON serializes the graph to JSON.
func (g *Graph) MarshalJSON() ([]byte, error) {
	type jsonTask struct {
		Name       string `json:"name"`
		TimeBudget string `json:"timeBudget,omitzero"`
		Subgraph   bool   `json:"subgraph,omitzero"`
	}
	jsonTasks := make([]jsonTask, len(g.nodes))
	for i, t := range g.nodes {
		jt := jsonTask{Name: t.Name, Subgraph: t.Subgraph}
		if t.TimeBudget > 0 {
			jt.TimeBudget = t.TimeBudget.String()
		}
		jsonTasks[i] = jt
	}
	type jsonGraph struct {
		Name        string             `json:"name"`
		EntryPoint  string             `json:"entryPoint"`
		Tasks       []jsonTask         `json:"tasks"`
		Transitions []Transition       `json:"transitions"`
		Reducers    map[string]Reducer `json:"reducers,omitzero"`
		Inputs      []string           `json:"inputs"`
		Outputs     []string           `json:"outputs"`
	}
	jg := jsonGraph{
		Name:        g.name,
		EntryPoint:  g.entryPoint,
		Tasks:       jsonTasks,
		Transitions: g.transitions,
		Reducers:    g.reducers,
		Inputs:      g.inputs,
		Outputs:     g.outputs,
	}
	if jg.Tasks == nil {
		jg.Tasks = []jsonTask{}
	}
	if jg.Transitions == nil {
		jg.Transitions = []Transition{}
	}
	return json.Marshal(jg)
}

// UnmarshalJSON deserializes the graph from JSON.
func (g *Graph) UnmarshalJSON(data []byte) error {
	type jsonTask struct {
		Name       string `json:"name"`
		TimeBudget string `json:"timeBudget,omitzero"`
		Subgraph   bool   `json:"subgraph,omitzero"`
	}
	type jsonGraph struct {
		Name        string             `json:"name"`
		EntryPoint  string             `json:"entryPoint"`
		Tasks       []jsonTask         `json:"tasks"`
		Transitions []Transition       `json:"transitions"`
		Reducers    map[string]Reducer `json:"reducers,omitzero"`
		Inputs      []string           `json:"inputs"`
		Outputs     []string           `json:"outputs"`
	}
	var jg jsonGraph
	if err := json.Unmarshal(data, &jg); err != nil {
		return err
	}
	g.name = jg.Name
	g.entryPoint = jg.EntryPoint
	g.nodes = make([]Node, len(jg.Tasks))
	for i, jt := range jg.Tasks {
		g.nodes[i].Name = jt.Name
		g.nodes[i].Subgraph = jt.Subgraph
		if jt.TimeBudget != "" {
			g.nodes[i].TimeBudget, _ = time.ParseDuration(jt.TimeBudget)
		}
	}
	g.transitions = jg.Transitions
	g.reducers = jg.Reducers
	g.inputs = jg.Inputs
	g.outputs = jg.Outputs
	return nil
}
