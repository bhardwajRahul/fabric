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
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
)

// END is a pseudo-node indicating that the workflow should terminate.
// Use it as the target of a transition to mark a terminal path.
const END = "END"

// Node describes a task or subgraph node registered in a workflow graph.
// Name is the node's identifier within the graph and the value stored on
// step rows (microbus_steps.task_name). URL is the dispatch target the
// foreman calls when the node is reached.
type Node struct {
	Name       string
	URL        string
	TimeBudget time.Duration
	Subgraph   bool
}

// Transition defines a possible transition between two nodes in a workflow graph.
// From and To are node names, not URLs.
type Transition struct {
	From       string `json:"from"`
	To         string `json:"to"`
	When       string `json:"when,omitzero"`
	WithGoto   bool   `json:"withGoto,omitzero"`
	ForEach    string `json:"forEach,omitzero"` // dynamic fan-out over a state field
	As         string `json:"as,omitzero"`      // alias for the current element during forEach fan-out
	OnError    bool   `json:"onError,omitzero"` // taken when the source task returns an error
	StatusCode int    `json:"statusCode,omitzero"`
}

// Graph is the definition of a workflow. It describes the tasks, transitions between them,
// and reducers for merging state during fan-in.
type Graph struct {
	name          string
	entryPoint    string
	nodes         []Node
	transitions   []Transition
	reducers      map[string]Reducer
	inputs        []string // nil/[] = pass nothing, ["*"] = pass everything, ["a","b"] = named
	outputs       []string // nil/[] = pass nothing, ["*"] = pass everything, ["a","b"] = named
	fanInNodes    map[string]bool
	fanOutToFanIn map[string]string // populated by Validate
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

// EntryPoint returns the node name of the entry point of the graph.
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
func (g *Graph) DeclareInputs(fields ...string) {
	g.inputs = fields
}

// DeclareOutputs declares which state fields are returned from this graph on completion.
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

// AddTask registers a task node in the graph under the given name, with the given URL as
// the dispatch target. The first node added becomes the default entry point unless
// SetEntryPoint is called explicitly. The pseudo-node END is not registered. Re-registering
// the same name is a no-op.
//
// The same URL may be registered under multiple names. This is how a workflow author
// reuses the same task code at distinct positions in the graph with different downstream
// transitions per position.
func (g *Graph) AddTask(name, url string) {
	if name == END {
		return
	}
	for i := range g.nodes {
		if g.nodes[i].Name == name {
			return
		}
	}
	g.nodes = append(g.nodes, Node{Name: name, URL: url})
	if g.entryPoint == "" {
		g.entryPoint = name
	}
}

// AddSubgraph registers a child workflow as a subgraph node in the graph under the given
// name, with the given URL as the dispatch target. Same registration semantics as AddTask.
func (g *Graph) AddSubgraph(name, workflowURL string) {
	if name == END {
		return
	}
	for i := range g.nodes {
		if g.nodes[i].Name == name {
			g.nodes[i].Subgraph = true
			return
		}
	}
	g.nodes = append(g.nodes, Node{Name: name, URL: workflowURL, Subgraph: true})
}

// IsSubgraph returns true if the given node name is registered as a subgraph.
func (g *Graph) IsSubgraph(name string) bool {
	for _, n := range g.nodes {
		if n.Name == name {
			return n.Subgraph
		}
	}
	return false
}

// URLOf returns the dispatch URL for a node identified by name. Returns the empty string
// if the name is not registered. END maps to itself.
func (g *Graph) URLOf(name string) string {
	if name == END {
		return END
	}
	for _, n := range g.nodes {
		if n.Name == name {
			return n.URL
		}
	}
	return ""
}

// NamesForURL returns all node names whose dispatch URL matches the given URL.
// Empty result means no node uses that URL. Multiple results mean the URL is reused
// at distinct graph positions.
func (g *Graph) NamesForURL(url string) []string {
	var names []string
	for _, n := range g.nodes {
		if n.URL == url {
			names = append(names, n.Name)
		}
	}
	return names
}

// SetEntryPoint sets the entry point of the graph explicitly, overriding the default
// (first task added). The argument is a node name.
func (g *Graph) SetEntryPoint(name string) {
	g.entryPoint = name
}

// AddTransition adds an unconditional transition between two nodes. Both endpoints are
// auto-registered as tasks if not already present (see autoRegister).
func (g *Graph) AddTransition(from, to string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to})
}

// AddTransitionWhen adds a conditional transition between two nodes.
func (g *Graph) AddTransitionWhen(from, to string, when string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, When: when})
}

// AddTransitionGoto adds a transition that is only taken when the source task calls
// flow.Goto with a target that resolves to this transition's destination.
func (g *Graph) AddTransitionGoto(from, to string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, WithGoto: true})
}

// AddTransitionForEach adds a dynamic fan-out transition.
func (g *Graph) AddTransitionForEach(from, to string, forEach string, as string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	if as == "" {
		as = "item"
	}
	g.transitions = append(g.transitions, Transition{From: from, To: to, ForEach: forEach, As: as})
}

// AddTransitionOnError adds a transition that is taken when the source task returns an error.
func (g *Graph) AddTransitionOnError(from, to string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, OnError: true})
}

// AddTransitionOnTimeout adds an error transition that is taken only when the source task's
// error carries HTTP status 408.
func (g *Graph) AddTransitionOnTimeout(from, to string) {
	from = g.autoRegister(from)
	to = g.autoRegister(to)
	g.transitions = append(g.transitions, Transition{From: from, To: to, OnError: true, StatusCode: http.StatusRequestTimeout})
}

// autoRegister resolves a transition endpoint string to a node name, registering a new
// node if needed. Resolution priority:
//  1. If a node with this name exists, return the name.
//  2. If exactly one node has this URL, return that node's name (URL is a unique alias).
//  3. Otherwise register a new node via AddTask(s, s) — name and URL are both s. This
//     is the backward-compatible path for existing graph builders that pass URLs directly
//     to AddTransition without a prior AddTask call.
//
// END passes through unchanged.
//
// To register a node with a name different from its URL (e.g. to reuse the same task
// at multiple positions in the graph), call AddTask(name, url) explicitly before any
// transitions reference that name.
func (g *Graph) autoRegister(s string) string {
	if s == END {
		return END
	}
	for _, n := range g.nodes {
		if n.Name == s {
			return n.Name
		}
	}
	matches := g.NamesForURL(s)
	if len(matches) == 1 {
		return matches[0]
	}
	g.AddTask(s, s)
	return s
}

// ErrorTransition returns the error transition from the given node name, if one exists.
func (g *Graph) ErrorTransition(name string) (Transition, bool) {
	for _, tr := range g.transitions {
		if tr.From == name && tr.OnError {
			return tr, true
		}
	}
	return Transition{}, false
}

// SetFanIn marks a node as a fan-in nexus. Opts the graph into the lineage validator.
func (g *Graph) SetFanIn(name string) {
	if g.fanInNodes == nil {
		g.fanInNodes = make(map[string]bool)
	}
	g.fanInNodes[name] = true
}

// IsFanIn reports whether the named node is a fan-in nexus.
func (g *Graph) IsFanIn(name string) bool {
	return g.fanInNodes[name]
}

// HasFanIn reports whether the graph declares any fan-in nexus.
func (g *Graph) HasFanIn() bool {
	return len(g.fanInNodes) > 0
}

// FanInFor returns the fan-in node that pops the frame pushed by a fan-out at the named
// source, or "" if the source is not a fan-out. Populated by Validate.
func (g *Graph) FanInFor(fanOutSource string) string {
	return g.fanOutToFanIn[fanOutSource]
}

// IsFanOutSource reports whether the named node has 2+ non-goto/non-error outgoing
// transitions, or any forEach outgoing transition.
func (g *Graph) IsFanOutSource(name string) bool {
	var normalCount int
	for _, tr := range g.transitions {
		if tr.From != name || tr.WithGoto || tr.OnError {
			continue
		}
		if tr.ForEach != "" {
			return true
		}
		normalCount++
		if normalCount >= 2 {
			return true
		}
	}
	return false
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

// SetTimeBudget sets the execution time budget for a specific node, by name.
func (g *Graph) SetTimeBudget(name string, budget time.Duration) {
	for i := range g.nodes {
		if g.nodes[i].Name == name {
			g.nodes[i].TimeBudget = budget
			return
		}
	}
}

// TimeBudget returns the execution time budget for a node by name, or 0 if not set.
func (g *Graph) TimeBudget(name string) time.Duration {
	for _, t := range g.nodes {
		if t.Name == name {
			return t.TimeBudget
		}
	}
	return 0
}

// Mermaid returns a fully-styled Mermaid flowchart representation of the graph,
// suitable for writing directly to a .mmd file. The output includes the classDef
// styles, a title node derived from the graph's URL, and per-node class annotations.
//
// Node shapes encode role: regular tasks are rectangles, subgraphs use the double
// rectangle, forEach targets use a stacked rectangle ("many of these"), and SetFanIn
// nodes use an inverted trapezoid ("many converging here"). Edges into a SetFanIn
// node carry a "fan-in" label.
func (g *Graph) Mermaid() string {
	var b strings.Builder
	b.WriteString("graph LR\n")
	b.WriteString("    classDef task fill:#32a7c1,color:#f4f2ef,stroke:#434343\n")
	b.WriteString("    classDef sub fill:#ed2e92,color:#f4f2ef,stroke:#434343\n")
	b.WriteString("    classDef term fill:#e5f4f3,color:#434343,stroke:#434343\n")
	b.WriteString("\n")

	// Title is the workflow's last URL path segment (kebab-case route).
	graphLabel := stripHostPort(g.name, "")
	fmt.Fprintf(&b, "    _title{{%q}}:::term --> _start\n", graphLabel)

	// Node IDs are stable t0..tN by registration order. Labels strip the graph's
	// own hostname:port prefix and protocol so URL-as-name nodes render as just
	// the route segment; cross-host references keep their hostname for clarity.
	ownHost := hostOf(g.name)
	ids := make(map[string]string, len(g.nodes))
	labels := make(map[string]string, len(g.nodes))
	for i, t := range g.nodes {
		ids[t.Name] = fmt.Sprintf("t%d", i)
		labels[t.Name] = stripHostPort(t.Name, ownHost)
	}

	// Identify nodes that need standalone shape declarations: forEach targets
	// (st-rect) and SetFanIn nodes (trap-t). Both use Mermaid's `@{...}` form,
	// which doesn't compose with `:::class`, so they get class lines at the end.
	foreachTargets := make(map[string]bool)
	standaloneOrder := []string{}
	for _, tr := range g.transitions {
		if tr.ForEach == "" || foreachTargets[tr.To] {
			continue
		}
		foreachTargets[tr.To] = true
		standaloneOrder = append(standaloneOrder, tr.To)
	}
	standaloneFanIn := make(map[string]bool)
	for _, t := range g.nodes {
		if g.fanInNodes[t.Name] && !foreachTargets[t.Name] {
			standaloneFanIn[t.Name] = true
			standaloneOrder = append(standaloneOrder, t.Name)
		}
	}
	for _, name := range standaloneOrder {
		shape := "st-rect"
		if standaloneFanIn[name] {
			shape = "trap-t"
		}
		fmt.Fprintf(&b, "    %s@{ shape: %s, label: %q }\n", ids[name], shape, labels[name])
	}

	defined := make(map[string]bool, len(g.nodes))
	for name := range foreachTargets {
		defined[name] = true
	}
	for name := range standaloneFanIn {
		defined[name] = true
	}
	nodeShape := func(name string) string {
		id := ids[name]
		if defined[name] {
			return id
		}
		defined[name] = true
		lbl := labels[name]
		classSuffix := ":::task"
		if g.IsSubgraph(name) {
			return fmt.Sprintf("%s[[%q]]:::sub", id, lbl)
		}
		return fmt.Sprintf("%s[%q]%s", id, lbl, classSuffix)
	}

	if _, ok := ids[g.entryPoint]; ok {
		fmt.Fprintf(&b, "    _start(( )):::term --> %s\n", nodeShape(g.entryPoint))
	}

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
			errLabel := "onError"
			if tr.StatusCode == http.StatusRequestTimeout {
				errLabel = "onTimeout"
			}
			if label != "" {
				label = errLabel + "; " + label
			} else {
				label = errLabel
			}
		}
		if label != "" {
			edgeLabels[e] = append(edgeLabels[e], label)
		}
	}

	for _, e := range edgeOrder {
		fromID := ids[e.from]
		label := strings.Join(edgeLabels[e], " | ")
		// "fan-in" annotation: every edge whose target is a SetFanIn node.
		if e.to != END && g.fanInNodes[e.to] {
			if label != "" {
				label += "; fan-in"
			} else {
				label = "fan-in"
			}
		}
		if e.to == END {
			if label != "" {
				fmt.Fprintf(&b, "    %s -->|%q| _end(( )):::term\n", fromID, label)
			} else {
				fmt.Fprintf(&b, "    %s --> _end(( )):::term\n", fromID)
			}
		} else {
			if label != "" {
				fmt.Fprintf(&b, "    %s -->|%q| %s\n", fromID, label, nodeShape(e.to))
			} else {
				fmt.Fprintf(&b, "    %s --> %s\n", fromID, nodeShape(e.to))
			}
		}
	}

	// Class annotations for shape-declared nodes (couldn't combine inline).
	for _, name := range standaloneOrder {
		class := "task"
		if g.IsSubgraph(name) {
			class = "sub"
		}
		fmt.Fprintf(&b, "    class %s %s\n", ids[name], class)
	}

	return b.String()
}

// hostOf returns the host[:port] of a URL-shaped string, or "" if it has no scheme.
func hostOf(s string) string {
	_, after, ok := strings.Cut(s, "://")
	if !ok {
		return ""
	}
	if i := strings.IndexByte(after, '/'); i >= 0 {
		return after[:i]
	}
	return after
}

// stripHostPort returns the path portion of a URL-shaped string when its host
// matches ownHost (or any host when ownHost is ""). Falls back to stripProto
// for URL-shaped values whose host doesn't match. Non-URL inputs pass through.
func stripHostPort(s, ownHost string) string {
	_, after, ok := strings.Cut(s, "://")
	if !ok {
		return s
	}
	host, path := after, ""
	if i := strings.IndexByte(after, '/'); i >= 0 {
		host = after[:i]
		path = after[i+1:]
	}
	if ownHost == "" || host == ownHost {
		return path
	}
	return after
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
		if t.URL == "" {
			return errors.New("node '%s' in graph '%s' has no URL", t.Name, g.name)
		}
	}
	if !nodeSet[g.entryPoint] || g.IsSubgraph(g.entryPoint) {
		return errors.New("entry point '%s' is not a registered task in graph '%s'", g.entryPoint, g.name)
	}
	for fanInName := range g.fanInNodes {
		if !nodeSet[fanInName] {
			return errors.New("SetFanIn references unknown node '%s' in graph '%s'", fanInName, g.name)
		}
		if fanInName == END {
			return errors.New("SetFanIn cannot mark END in graph '%s'", g.name)
		}
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
		if tr.StatusCode != 0 && !tr.OnError {
			return errors.New("transition from '%s' to '%s' in graph '%s' sets statusCode without onError", tr.From, tr.To, g.name)
		}
		if tr.OnError && tr.From == tr.To {
			return errors.New("transition from '%s' to itself in graph '%s' would loop unboundedly; use flow.Retry or flow.RetryOnTimeout in the task body for bounded retries with backoff", stripProto(tr.From), g.name)
		}
		if tr.When != "" {
			err := boolexp.Validate(tr.When)
			if err != nil {
				return errors.New("transition from '%s' to '%s' in graph '%s' has invalid 'when' expression: %v", stripProto(tr.From), stripProto(tr.To), g.name, err)
			}
		}
	}

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

	return g.validateLineage()
}

// validateLineage runs when SetFanIn is declared; see CLAUDE.md.
// Side effect: populates g.fanOutToFanIn.
func (g *Graph) validateLineage() error {
	g.fanOutToFanIn = make(map[string]string)

	isFanOutSource := make(map[string]bool, len(g.nodes))
	for _, t := range g.nodes {
		var normalCount int
		var hasForEach bool
		for _, tr := range g.transitions {
			if tr.From != t.Name || tr.WithGoto || tr.OnError {
				continue
			}
			normalCount++
			if tr.ForEach != "" {
				hasForEach = true
			}
		}
		if normalCount >= 2 || hasForEach {
			isFanOutSource[t.Name] = true
		}
	}

	stacks := make(map[string][]string, len(g.nodes))
	queue := []string{g.entryPoint}
	stacks[g.entryPoint] = nil

	stackEqual := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	stackCopy := func(s []string) []string {
		if len(s) == 0 {
			return nil
		}
		c := make([]string, len(s))
		copy(c, s)
		return c
	}

	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		fromStack := stacks[from]
		fromIsFanOut := isFanOutSource[from]

		for _, tr := range g.transitions {
			if tr.From != from {
				continue
			}
			var nextStack []string
			switch {
			case tr.WithGoto, tr.OnError:
				// Stay in the same scope.
				nextStack = fromStack
			case g.fanInNodes[tr.To]:
				if fromIsFanOut {
					// push+pop on the same edge cancel
					nextStack = fromStack
					g.fanOutToFanIn[from] = tr.To
				} else {
					if len(fromStack) == 0 {
						return errors.New(
							"transition from '%s' to fan-in node '%s' in graph '%s' has no fan-out frame to pop",
							stripProto(from), stripProto(tr.To), g.name,
						)
					}
					nextStack = stackCopy(fromStack[:len(fromStack)-1])
					g.fanOutToFanIn[fromStack[len(fromStack)-1]] = tr.To
				}
			case fromIsFanOut:
				nextStack = append(stackCopy(fromStack), from)
			default:
				nextStack = fromStack
			}

			if tr.To == END {
				if len(nextStack) != 0 {
					return errors.New(
						"transition from '%s' to END in graph '%s' has unpopped fan-out frames %v; every branch must pass through a fan-in node before reaching END",
						stripProto(from), g.name, nextStack,
					)
				}
				continue
			}

			if prior, seen := stacks[tr.To]; seen {
				if !stackEqual(prior, nextStack) {
					return errors.New(
						"node '%s' in graph '%s' is reachable with two different lineage stacks (%v and %v); register a separate alias node via AddTask to disambiguate",
						stripProto(tr.To), g.name, prior, nextStack,
					)
				}
				continue
			}
			stacks[tr.To] = nextStack
			queue = append(queue, tr.To)
		}
	}

	for source := range isFanOutSource {
		if _, ok := g.fanOutToFanIn[source]; !ok {
			return errors.New(
				"fan-out source '%s' in graph '%s' has no fan-in node downstream; mark the convergence node with SetFanIn",
				stripProto(source), g.name,
			)
		}
	}

	return nil
}

// stripProto removes the scheme prefix from a URL-like string for cleaner error messages.
func stripProto(s string) string {
	var x string
	if _, x, _ = strings.Cut(s, "://"); x == "" {
		x = s
	}
	return x
}

// MarshalJSON serializes the graph to JSON.
func (g *Graph) MarshalJSON() ([]byte, error) {
	type jsonTask struct {
		Name       string `json:"name"`
		URL        string `json:"url,omitzero"`
		TimeBudget string `json:"timeBudget,omitzero"`
		Subgraph   bool   `json:"subgraph,omitzero"`
		FanIn      bool   `json:"fanIn,omitzero"`
	}
	jsonTasks := make([]jsonTask, len(g.nodes))
	for i, t := range g.nodes {
		jt := jsonTask{Name: t.Name, URL: t.URL, Subgraph: t.Subgraph, FanIn: g.fanInNodes[t.Name]}
		if t.TimeBudget > 0 {
			jt.TimeBudget = t.TimeBudget.String()
		}
		jsonTasks[i] = jt
	}
	type jsonGraph struct {
		Name          string             `json:"name"`
		EntryPoint    string             `json:"entryPoint"`
		Tasks         []jsonTask         `json:"tasks"`
		Transitions   []Transition       `json:"transitions"`
		Reducers      map[string]Reducer `json:"reducers,omitzero"`
		Inputs        []string           `json:"inputs"`
		Outputs       []string           `json:"outputs"`
		FanOutToFanIn map[string]string  `json:"fanOutToFanIn,omitzero"`
	}
	jg := jsonGraph{
		Name:          g.name,
		EntryPoint:    g.entryPoint,
		Tasks:         jsonTasks,
		Transitions:   g.transitions,
		Reducers:      g.reducers,
		Inputs:        g.inputs,
		Outputs:       g.outputs,
		FanOutToFanIn: g.fanOutToFanIn,
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
		URL        string `json:"url,omitzero"`
		TimeBudget string `json:"timeBudget,omitzero"`
		Subgraph   bool   `json:"subgraph,omitzero"`
		FanIn      bool   `json:"fanIn,omitzero"`
	}
	type jsonGraph struct {
		Name          string             `json:"name"`
		EntryPoint    string             `json:"entryPoint"`
		Tasks         []jsonTask         `json:"tasks"`
		Transitions   []Transition       `json:"transitions"`
		Reducers      map[string]Reducer `json:"reducers,omitzero"`
		Inputs        []string           `json:"inputs"`
		Outputs       []string           `json:"outputs"`
		FanOutToFanIn map[string]string  `json:"fanOutToFanIn,omitzero"`
	}
	var jg jsonGraph
	err := json.Unmarshal(data, &jg)
	if err != nil {
		return err
	}
	g.name = jg.Name
	g.entryPoint = jg.EntryPoint
	g.nodes = make([]Node, len(jg.Tasks))
	g.fanInNodes = nil
	for i, jt := range jg.Tasks {
		g.nodes[i].Name = jt.Name
		g.nodes[i].URL = jt.URL
		if g.nodes[i].URL == "" {
			// Legacy JSON (no url field): name doubled as the URL.
			g.nodes[i].URL = jt.Name
		}
		g.nodes[i].Subgraph = jt.Subgraph
		if jt.TimeBudget != "" {
			g.nodes[i].TimeBudget, _ = time.ParseDuration(jt.TimeBudget)
		}
		if jt.FanIn {
			if g.fanInNodes == nil {
				g.fanInNodes = make(map[string]bool)
			}
			g.fanInNodes[jt.Name] = true
		}
	}
	g.transitions = jg.Transitions
	g.reducers = jg.Reducers
	g.inputs = jg.Inputs
	g.outputs = jg.Outputs
	g.fanOutToFanIn = jg.FanOutToFanIn
	return nil
}
