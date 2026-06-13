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

package foremanapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/microbus-io/fabric/workflow"
)

// Default palette colors.
const (
	defaultPrimaryFill   = "#32a7c1"
	defaultPrimaryText   = "#f4f2ef"
	defaultSecondaryFill = "#e5f4f3"
	defaultSecondaryText = "#434343"
	defaultErrorFill     = "#f15922"
	defaultErrorText     = "#f4f2ef"
	defaultAttentionFill = "#ed2e92"
	defaultAttentionText = "#f4f2ef"
)

// FlowRenderer renders the execution history of a flow as a Mermaid flowchart. Configure via
// the With* builder methods, then call Render. The zero behavior reproduces the default brand
// palette in top-down orientation with no title and no click links.
type FlowRenderer struct {
	steps         []FlowStep
	primaryFill   string
	primaryText   string
	secondaryFill string
	secondaryText string
	errorFill     string
	errorText     string
	attentionFill string
	attentionText string
	direction     string
	title         string
	linkParam     string
}

// NewFlowRenderer creates a renderer for a flow's execution history. steps is
// the output of foreman.History (or any compatible source); the renderer treats
// it as read-only.
func NewFlowRenderer(steps []FlowStep) *FlowRenderer {
	return &FlowRenderer{
		steps:         steps,
		primaryFill:   defaultPrimaryFill,
		primaryText:   defaultPrimaryText,
		secondaryFill: defaultSecondaryFill,
		secondaryText: defaultSecondaryText,
		errorFill:     defaultErrorFill,
		errorText:     defaultErrorText,
		attentionFill: defaultAttentionFill,
		attentionText: defaultAttentionText,
		direction:     "TD",
	}
}

// hasInflightStep reports whether any step in the tree is still pending, running, or
// interrupted. Render uses it to suppress the trailing _end marker on an in-motion flow.
func hasInflightStep(steps []FlowStep) bool {
	for _, s := range steps {
		switch s.Status {
		case workflow.StatusPending, workflow.StatusRunning, workflow.StatusInterrupted, "":
			return true
		}
		if s.Subgraph && hasInflightStep(s.SubHistory) {
			return true
		}
	}
	return false
}

// WithPrimaryColors overrides the primary pair (completed and running task nodes; running adds
// a dashed border). fill is the node color; text is the label color.
func (r *FlowRenderer) WithPrimaryColors(fill, text string) *FlowRenderer {
	r.primaryFill = fill
	r.primaryText = text
	return r
}

// WithSecondaryColors overrides the secondary pair (chrome markers, subgraph wrapper boxes, and
// pending task nodes). fill is the surface color; text is the label color.
func (r *FlowRenderer) WithSecondaryColors(fill, text string) *FlowRenderer {
	r.secondaryFill = fill
	r.secondaryText = text
	return r
}

// WithErrorColors overrides the error pair (failed and cancelled task nodes).
func (r *FlowRenderer) WithErrorColors(fill, text string) *FlowRenderer {
	r.errorFill = fill
	r.errorText = text
	return r
}

// WithAttentionColors overrides the attention pair (interrupted task nodes).
func (r *FlowRenderer) WithAttentionColors(fill, text string) *FlowRenderer {
	r.attentionFill = fill
	r.attentionText = text
	return r
}

// WithTopDown renders top-to-bottom. The default.
func (r *FlowRenderer) WithTopDown() *FlowRenderer {
	r.direction = "TD"
	return r
}

// WithLeftRight renders left-to-right.
func (r *FlowRenderer) WithLeftRight() *FlowRenderer {
	r.direction = "LR"
	return r
}

// WithTitle sets a title tile rendered above the start marker. Empty suppresses it.
func (r *FlowRenderer) WithTitle(text string) *FlowRenderer {
	r.title = text
	return r
}

// WithLinks enables click directives on every task node, emitting "?<paramName>=<stepKey>".
// Empty suppresses all click directives.
func (r *FlowRenderer) WithLinks(paramName string) *FlowRenderer {
	r.linkParam = paramName
	return r
}

// Render returns the Mermaid flowchart representation of the flow's execution
// history.
func (r *FlowRenderer) Render() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "flowchart %s\n", r.direction)

	// pending and term fall back to secondaryFill for stroke when secondaryText is empty (CSS
	// mode), to keep Mermaid from painting its default purple border.
	chromeStroke := r.secondaryText
	if chromeStroke == "" {
		chromeStroke = r.secondaryFill
	}
	classDefLine(&b, workflow.StatusCompleted, r.primaryFill, r.primaryText, r.primaryFill, "")
	classDefLine(&b, workflow.StatusRunning, r.primaryFill, r.primaryText, r.primaryFill, "stroke-dasharray:4 2")
	classDefLine(&b, workflow.StatusPending, r.secondaryFill, r.secondaryText, chromeStroke, "")
	classDefLine(&b, workflow.StatusFailed, r.errorFill, r.errorText, r.errorFill, "")
	classDefLine(&b, workflow.StatusCancelled, r.errorFill, r.errorText, r.errorFill, "")
	classDefLine(&b, workflow.StatusInterrupted, r.attentionFill, r.attentionText, r.attentionFill, "")
	classDefLine(&b, "term", r.secondaryFill, r.secondaryText, chromeStroke, "")

	fmt.Fprintf(&b, "    linkStyle default stroke:%s\n", r.primaryFill)
	b.WriteString("\n")

	if r.title != "" {
		fmt.Fprintf(&b, "    _title{{%q}}:::term -.-> _start\n", r.title)
	}
	showEnd := !hasInflightStep(r.steps)

	b.WriteString("    _start((\" \")):::term\n")
	if showEnd {
		b.WriteString("    _end((\" \")):::term\n")
	}

	heads, tails := r.renderSteps(&b, "", r.steps)
	for _, h := range heads {
		fmt.Fprintf(&b, "    _start --> %s\n", h)
	}
	if showEnd {
		for _, t := range tails {
			fmt.Fprintf(&b, "    %s --> _end\n", t)
		}
	}

	return b.String(), nil
}

// renderSteps writes Mermaid nodes and edges for a list of steps, returning the head node IDs
// (no incoming edge) and tail node IDs (no outgoing edge) so the caller can wire them to
// surrounding markers. Recurses through subgraph SubHistory via the same prefix scheme.
func (r *FlowRenderer) renderSteps(buf *strings.Builder, prefix string, steps []FlowStep) (heads []string, tails []string) {
	if len(steps) == 0 {
		return nil, nil
	}

	type renderNode struct {
		entries []string
		exits   []string
	}
	byID := make(map[int]*renderNode, len(steps))
	stepByID := make(map[int]FlowStep, len(steps))
	order := make([]int, 0, len(steps))

	type subBlock struct {
		blockID       string
		label         string
		body          string
		innerHeads    []string
		callEdgeLabel string
	}
	subBlocks := map[int]subBlock{}

	for i := range steps {
		s := steps[i]
		if s.StepID == 0 {
			continue
		}
		nodeID := fmt.Sprintf("%ss%d", prefix, s.StepID)
		byID[s.StepID] = &renderNode{entries: []string{nodeID}, exits: []string{nodeID}}
		if s.Subgraph && len(s.SubHistory) > 0 {
			subPrefix := fmt.Sprintf("%ss%d_sub", prefix, s.StepID)
			var body strings.Builder
			subHeads, subTails := r.renderSteps(&body, subPrefix, s.SubHistory)
			blockID := fmt.Sprintf("%ss%d_sg", prefix, s.StepID)
			label := stripProto(s.SubWorkflowName)
			if label == "" {
				label = "subgraph"
			}
			// Call-edge label is the entry step's queue wait (StartedAt - CreatedAt).
			var callEdgeLabel string
			for _, sub := range s.SubHistory {
				if sub.PredecessorID == 0 && sub.HasStarted() && !sub.StartedAt.IsZero() && !sub.CreatedAt.IsZero() {
					if d := sub.StartedAt.Sub(sub.CreatedAt); d > 0 {
						callEdgeLabel = formatDuration(d)
					}
					break
				}
			}
			subBlocks[s.StepID] = subBlock{
				blockID:       blockID,
				label:         label,
				body:          body.String(),
				innerHeads:    subHeads,
				callEdgeLabel: callEdgeLabel,
			}
			// Outgoing DAG edges from a subgraph caller route through the inner tails.
			if len(subTails) > 0 {
				byID[s.StepID].exits = subTails
			}
		}
		stepByID[s.StepID] = s
		order = append(order, s.StepID)
	}

	emitClick := func(nodeID, stepKey string) {
		if r.linkParam == "" || stepKey == "" {
			return
		}
		fmt.Fprintf(buf, "    click %s \"?%s=%s\"\n", nodeID, r.linkParam, stepKey)
	}

	emitStep := func(stepID int) {
		s := stepByID[stepID]
		nodeID := fmt.Sprintf("%ss%d", prefix, stepID)
		label := stripProto(s.TaskName)
		blk, isSubgraphCaller := subBlocks[stepID]
		// Duration line: task body time for a regular step, NET caller cost for a subgraph
		// caller. Skipped on non-terminal steps or when subgraph wall time isn't finalized.
		if isSubgraphCaller {
			if isTerminalStepStatus(s.Status) && s.HasStarted() && !s.UpdatedAt.IsZero() && !s.StartedAt.IsZero() {
				if subDur, ok := subgraphWallTime(s.SubHistory); ok {
					net := s.UpdatedAt.Sub(s.StartedAt) - subDur
					if net < 0 {
						net = 0
					}
					label += "\n" + formatDuration(net)
				}
			}
		} else if s.HasStarted() && isTerminalStepStatus(s.Status) && !s.UpdatedAt.IsZero() && !s.StartedAt.IsZero() {
			label += "\n" + formatDuration(s.UpdatedAt.Sub(s.StartedAt))
		}
		statusClass := s.Status
		if statusClass == "" {
			statusClass = workflow.StatusPending
		}
		fmt.Fprintf(buf, "    %s[\"%s\"]:::%s\n", nodeID, escapeMermaidLabel(label), statusClass)
		emitClick(nodeID, s.StepKey)
		if isSubgraphCaller {
			fmt.Fprintf(buf, "    subgraph %s [%q]\n", blk.blockID, blk.label)
			buf.WriteString("        direction TB\n")
			buf.WriteString(blk.body)
			buf.WriteString("    end\n")
			fmt.Fprintf(buf, "    style %s %s\n", blk.blockID, r.clusterStyle())
		}
	}

	// Cohort grouping: any predecessor with 2+ children gets an invisible Mermaid layout
	// container so siblings cluster near their parent.
	childrenOf := map[int][]int{}
	for _, id := range order {
		s := stepByID[id]
		if s.PredecessorID == 0 {
			continue
		}
		childrenOf[s.PredecessorID] = append(childrenOf[s.PredecessorID], id)
	}
	cohortOf := map[int]int{}
	for parentID, kids := range childrenOf {
		if len(kids) >= 2 {
			for _, k := range kids {
				cohortOf[k] = parentID
			}
		}
	}

	emittedSteps := map[int]bool{}
	for _, id := range order {
		if emittedSteps[id] {
			continue
		}
		pid, ok := cohortOf[id]
		if !ok {
			emitStep(id)
			emittedSteps[id] = true
			continue
		}
		blockID := fmt.Sprintf("%sfo_s%d", prefix, pid)
		fmt.Fprintf(buf, "    subgraph %s [\" \"]\n", blockID)
		buf.WriteString("        direction TB\n")
		for _, child := range childrenOf[pid] {
			emitStep(child)
			emittedSteps[child] = true
		}
		buf.WriteString("    end\n")
		fmt.Fprintf(buf, "    style %s fill:none,stroke:none\n", blockID)
	}

	emitted := map[string]bool{}
	hasIncoming := map[int]bool{}
	hasOutgoing := map[int]bool{}
	// emitEdge writes one Mermaid edge with optional inline label, deduped by (src, dst).
	emitEdge := func(src, dst, label string) {
		if src == dst {
			return
		}
		key := src + "\x00" + dst
		if emitted[key] {
			return
		}
		emitted[key] = true
		if label == "" {
			fmt.Fprintf(buf, "    %s --> %s\n", src, dst)
			return
		}
		fmt.Fprintf(buf, "    %s -- %q --> %s\n", src, label, dst)
	}
	// edgeLabel returns to.StartedAt - from.UpdatedAt as a transition-gap label, or "" when
	// either endpoint's timestamps are not available.
	edgeLabel := func(from, to FlowStep) string {
		if from.UpdatedAt.IsZero() || !to.HasStarted() || to.StartedAt.IsZero() {
			return ""
		}
		d := to.StartedAt.Sub(from.UpdatedAt)
		if d < 0 {
			d = 0
		}
		return formatDuration(d)
	}
	addEdge := func(fromID, toID int) {
		from := byID[fromID]
		to := byID[toID]
		if from == nil || to == nil || fromID == toID {
			return
		}
		label := edgeLabel(stepByID[fromID], stepByID[toID])
		for _, src := range from.exits {
			for _, dst := range to.entries {
				emitEdge(src, dst, label)
			}
		}
		hasOutgoing[fromID] = true
		hasIncoming[toID] = true
	}
	for _, id := range order {
		s := stepByID[id]
		if s.PredecessorID != 0 {
			addEdge(s.PredecessorID, id)
		}
		if s.SuccessorID != 0 {
			addEdge(id, s.SuccessorID)
		}
	}

	// Call edges: caller --> innerHead, labeled with the entry's queue wait. Don't mark
	// hasOutgoing on the caller - inner tails own that semantic via byID[caller].exits.
	for _, id := range order {
		blk, ok := subBlocks[id]
		if !ok {
			continue
		}
		callerNodeID := fmt.Sprintf("%ss%d", prefix, id)
		for _, h := range blk.innerHeads {
			emitEdge(callerNodeID, h, blk.callEdgeLabel)
		}
	}

	for _, id := range order {
		if !hasIncoming[id] {
			heads = append(heads, byID[id].entries...)
		}
		if !hasOutgoing[id] {
			tails = append(tails, byID[id].exits...)
		}
	}
	return heads, tails
}

// clusterStyle returns the comma-separated style declarations for subgraph wrapper boxes:
// faint primary tint, no border. Omits color when primaryText is empty (CSS-driven).
func (r *FlowRenderer) clusterStyle() string {
	parts := make([]string, 0, 4)
	if r.primaryFill != "" {
		parts = append(parts, "fill:"+r.primaryFill)
	}
	parts = append(parts, "fill-opacity:0.03")
	parts = append(parts, "stroke:none")
	if r.primaryText != "" && r.primaryFill != "" {
		parts = append(parts, "color:"+r.primaryFill)
	}
	return strings.Join(parts, ",")
}

// classDefLine emits one classDef directive, omitting empty fill/color/stroke and appending the
// verbatim extra declaration (e.g. "stroke-dasharray:4 2") when non-empty.
func classDefLine(b *strings.Builder, name, fill, text, stroke, extra string) {
	parts := make([]string, 0, 4)
	if fill != "" {
		parts = append(parts, "fill:"+fill)
	}
	if text != "" {
		parts = append(parts, "color:"+text)
	}
	if stroke != "" {
		parts = append(parts, "stroke:"+stroke)
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	fmt.Fprintf(b, "    classDef %s %s\n", name, strings.Join(parts, ","))
}

// escapeMermaidLabel neutralizes the characters that would break a quoted Mermaid node label.
// Newlines are left intact so multi-line labels render.
func escapeMermaidLabel(s string) string {
	return strings.NewReplacer(
		"\"", "#quot;",
		"[", "#91;",
		"]", "#93;",
	).Replace(s)
}

// isTerminalStepStatus reports whether a step's UpdatedAt is final and will not change.
func isTerminalStepStatus(status string) bool {
	switch status {
	case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled:
		return true
	}
	return false
}

// subgraphWallTime walks SubHistory recursively and returns max(UpdatedAt) - min(CreatedAt).
// Returns ok=false when any step in the tree is still in-flight.
func subgraphWallTime(history []FlowStep) (time.Duration, bool) {
	if len(history) == 0 {
		return 0, false
	}
	var minCreated, maxUpdated time.Time
	var walk func(steps []FlowStep) bool
	walk = func(steps []FlowStep) bool {
		for _, s := range steps {
			if !isTerminalStepStatus(s.Status) {
				return false
			}
			if minCreated.IsZero() || s.CreatedAt.Before(minCreated) {
				minCreated = s.CreatedAt
			}
			if s.UpdatedAt.After(maxUpdated) {
				maxUpdated = s.UpdatedAt
			}
			if s.Subgraph && len(s.SubHistory) > 0 {
				if !walk(s.SubHistory) {
					return false
				}
			}
		}
		return true
	}
	if !walk(history) {
		return 0, false
	}
	return maxUpdated.Sub(minCreated), true
}

// stripProto removes a "scheme://" prefix from a URL-like string.
func stripProto(u string) string {
	left, right, cut := strings.Cut(u, "://")
	if !cut {
		return left
	}
	return right
}

// formatDuration formats a duration as a human-readable label like "211ms", "2.34s", or "1h25m".
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.3gs", d.Seconds())
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		days := int(d.Hours() / 24)
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, h)
	}
}
