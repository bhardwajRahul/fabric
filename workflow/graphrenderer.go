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
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Default palette. Primary (teal) is the brand fill used on task nodes, edges,
// titles, cluster text, and note text, with off-white text drawn on top of it.
// Secondary (light teal) is the surface used on terminal markers, routing
// diamonds, the reduce circle, the title tile, and the edge label background,
// with dark grey text drawn on top of it.
const (
	defaultPrimaryFill   = "#32a7c1"
	defaultPrimaryText   = "#f4f2ef"
	defaultSecondaryFill = "#e5f4f3"
	defaultSecondaryText = "#434343"
)

// GraphRenderer renders a workflow Graph to a Mermaid flowchart. Configure via
// the With* builder methods, then call Render. The zero behavior reproduces the
// default brand palette in left-to-right orientation with the title label on.
type GraphRenderer struct {
	g               *Graph
	primaryFill     string
	primaryText     string
	secondaryFill   string
	secondaryText   string
	annotationColor string // empty means follow primaryFill
	direction       string
	titleLabel      bool
	linkParam       string
}

// NewGraphRenderer creates a renderer for the given graph with default styling.
func NewGraphRenderer(g *Graph) *GraphRenderer {
	return &GraphRenderer{
		g:             g,
		primaryFill:   defaultPrimaryFill,
		primaryText:   defaultPrimaryText,
		secondaryFill: defaultSecondaryFill,
		secondaryText: defaultSecondaryText,
		direction:     "LR",
		titleLabel:    true,
	}
}

// WithPrimaryColors overrides the primary brand pair. fill is the brand color
// applied to task node fill and stroke, edge lines, the title color, cluster
// text, and note text; text is the color drawn on top of fill (the label
// inside task nodes). The pair must be chosen together to stay legible.
func (r *GraphRenderer) WithPrimaryColors(fill, text string) *GraphRenderer {
	r.primaryFill = fill
	r.primaryText = text
	return r
}

// WithSecondaryColors overrides the secondary surface pair. fill is the
// surface color applied to terminal markers (start, end, title tile), routing
// diamonds, the reduce circle, and the edge label background; text is the
// color drawn on top of fill (terminal labels, edge labels, diamond text).
// The pair must be chosen together to stay legible.
func (r *GraphRenderer) WithSecondaryColors(fill, text string) *GraphRenderer {
	r.secondaryFill = fill
	r.secondaryText = text
	return r
}

// WithAnnotationColor overrides the color of annotation text rendered under
// nodes via Graph.Annotate. Annotations sit on the page background rather
// than on any palette fill, so this is a third surface independent of the
// primary and secondary pairs. Passing the empty string restores the default
// behavior: annotation color tracks the primary fill.
func (r *GraphRenderer) WithAnnotationColor(color string) *GraphRenderer {
	r.annotationColor = color
	return r
}

// WithTopDown renders the diagram top-to-bottom instead of the default
// left-to-right orientation.
func (r *GraphRenderer) WithTopDown() *GraphRenderer {
	r.direction = "TD"
	return r
}

// WithLeftRight renders the diagram left-to-right. This is the default orientation
// for the graph renderer; provided for symmetry with WithTopDown.
func (r *GraphRenderer) WithLeftRight() *GraphRenderer {
	r.direction = "LR"
	return r
}

// WithTitleLabel toggles the title tile that precedes the start marker. The
// title shows the graph's last URL path segment.
func (r *GraphRenderer) WithTitleLabel(show bool) *GraphRenderer {
	r.titleLabel = show
	return r
}

// WithLinks enables click directives on every task node. The emitted hyperlink
// is "?<paramName>=<taskName>", typically consumed by a host page that uses
// paramName to load a task inspector. Empty (the default) suppresses all click
// directives.
func (r *GraphRenderer) WithLinks(paramName string) *GraphRenderer {
	r.linkParam = paramName
	return r
}

// Render returns a fully-styled Mermaid flowchart representation of the graph,
// suitable for writing directly to a .mmd file. The output includes the
// classDef styles, a title node derived from the graph's URL, and per-node
// class annotations.
//
// Each forEach fan-out scope is rendered as a Mermaid subgraph block (dashed
// outline, faint fill) titled "for each"; nodes that share the same lineage
// frame sit inside that block, nested scopes nest accordingly. Subgraph nodes
// render as their own dashed block titled "subgraph". Static When fan-outs do
// not get a scope block; their branches stay as plain arrows.
func (r *GraphRenderer) Render() (string, error) {
	var b strings.Builder
	// All color values flow through classDef and style directives. Deliberately
	// no `%%{init: themeVariables}%%` block: Mermaid's theme parser rejects
	// non-literal color values (CSS var(...), currentColor), so a caller passing
	// CSS custom properties would break parsing. Callers wanting global theme
	// behavior should achieve it by passing CSS custom properties to the With*
	// setters - the cascade resolves them at render time.
	fmt.Fprintf(&b, "graph %s\n", r.direction)
	fmt.Fprintf(&b, "    classDef task fill:%s,color:%s,stroke:%s\n", r.primaryFill, r.primaryText, r.primaryFill)
	fmt.Fprintf(&b, "    classDef term fill:%s,color:%s,stroke:%s\n", r.secondaryFill, r.secondaryText, r.primaryFill)
	annoColor := r.annotationColor
	if annoColor == "" {
		annoColor = r.primaryFill
	}
	fmt.Fprintf(&b, "    classDef note fill:none,stroke:none,color:%s,font-size:0.8em\n", annoColor)
	// linkStyle default keeps edges in the primary palette without relying on
	// themeVariables.lineColor.
	fmt.Fprintf(&b, "    linkStyle default stroke:%s\n", r.primaryFill)
	b.WriteString("\n")

	if r.titleLabel {
		graphLabel := stripHostPort(r.g.name, "")
		fmt.Fprintf(&b, "    _title{{%q}}:::term -.-> _start\n", graphLabel)
	}

	heads, endEdges := r.renderBody(&b, "    ", "")

	if len(heads) > 0 {
		for _, h := range heads {
			fmt.Fprintf(&b, "    _start(( )):::term --> %s\n", h)
		}
	} else {
		b.WriteString("    _start(( )):::term\n")
	}
	for _, ee := range endEdges {
		if ee.label != "" {
			fmt.Fprintf(&b, "    %s -->|%q| _end(( )):::term\n", ee.from, ee.label)
		} else {
			fmt.Fprintf(&b, "    %s --> _end(( )):::term\n", ee.from)
		}
	}

	return b.String(), nil
}

// endEdge records an edge that terminated at END in the rendered body, so the
// caller can wire it to the _end marker.
type endEdge struct {
	from  string
	label string
}

// renderBody emits the graph's nodes and transitions into b. It does NOT emit the title,
// _start/_end markers, classDefs, or theme variables - those belong to the top-level Render.
// The prefix is prepended to every generated node ID.
//
// Returns:
//   - heads: rendered node IDs that incoming edges from outside should target.
//   - endEdges: edges that terminated at END inside the body; each carries the
//     source's rendered exit ID and the edge label.
func (r *GraphRenderer) renderBody(b *strings.Builder, indent string, prefix string) (heads []string, endEdges []endEdge) {
	g := r.g
	// Node IDs are stable t0..tN by registration order. Labels strip the graph's
	// own hostname:port prefix and protocol so URL-as-name nodes render as just
	// the route segment; cross-host references keep their hostname for clarity.
	ownHost := hostOf(g.name)
	ids := make(map[string]string, len(g.nodes))
	labels := make(map[string]string, len(g.nodes))
	for i, t := range g.nodes {
		ids[t.Name] = fmt.Sprintf("%st%d", prefix, i)
		labels[t.Name] = stripHostPort(t.Name, ownHost)
	}

	entries := make(map[string][]string, len(g.nodes))
	exits := make(map[string][]string, len(g.nodes))
	for _, t := range g.nodes {
		// SetFanIn nodes get a "reduce" circle in front of them: all incoming
		// arrows converge on the circle, which then has a single edge to the
		// task body. The circle is the visual cue for the implicit reducer
		// merge that happens before the fan-in task runs.
		if g.fanInNodes[t.Name] {
			entries[t.Name] = []string{ids[t.Name] + "_reduce"}
		} else {
			entries[t.Name] = []string{ids[t.Name]}
		}
		exits[t.Name] = []string{ids[t.Name]}
	}

	// Switch and When success-path transitions route through a labeled diamond
	// node between the source and the targets, so the rendered graph reads as
	// a routing decision rather than a fan of competing arrows. Goto, OnError,
	// OnTimeout, plain unconditional, and forEach edges stay attached directly
	// to the source node since they each preempt the Switch/When evaluation.
	type diamondArm struct {
		to    string
		label string
	}
	switchArms := map[string][]diamondArm{}
	whenArms := map[string][]diamondArm{}
	for _, tr := range g.transitions {
		if tr.WithGoto || tr.OnError {
			continue
		}
		if tr.Switch {
			label := tr.When
			if label == "true" {
				label = "default"
			}
			switchArms[tr.From] = append(switchArms[tr.From], diamondArm{to: tr.To, label: label})
			continue
		}
		if tr.When != "" && tr.ForEach == "" {
			whenArms[tr.From] = append(whenArms[tr.From], diamondArm{to: tr.To, label: tr.When})
		}
	}

	emitNodeBody := func(name string, indent string) {
		fmt.Fprintf(b, "%s%s[%q]:::task\n", indent, ids[name], labels[name])
		if r.linkParam != "" {
			fmt.Fprintf(b, "%sclick %s \"?%s=%s\"\n", indent, ids[name], r.linkParam, url.QueryEscape(name))
		}
	}
	emitOneNode := func(name string, indent string) {
		// Annotated nodes are wrapped in an invisible direction-TB subgraph so
		// the note renders directly below the node in an outer LR flow. The
		// subgraph border and fill are styled transparent and the note has no
		// chrome - it reads as floating teal text under the node.
		if note := g.annotations[name]; note != "" {
			annoID := ids[name] + "_anno"
			noteID := ids[name] + "_note"
			fmt.Fprintf(b, "%ssubgraph %s [\" \"]\n", indent, annoID)
			fmt.Fprintf(b, "%s    direction TB\n", indent)
			emitNodeBody(name, indent+"    ")
			fmt.Fprintf(b, "%s    %s[%q]:::note\n", indent, noteID, note)
			fmt.Fprintf(b, "%send\n", indent)
			fmt.Fprintf(b, "%sstyle %s fill:none,stroke:none\n", indent, annoID)
		} else {
			emitNodeBody(name, indent)
		}
		if g.fanInNodes[name] {
			fmt.Fprintf(b, "%s%s_reduce((%q)):::term\n", indent, ids[name], "reduce")
			fmt.Fprintf(b, "%s%s_reduce --> %s\n", indent, ids[name], ids[name])
		}
		if len(switchArms[name]) > 0 {
			fmt.Fprintf(b, "%s%s_switch{%q}:::term\n", indent, ids[name], "switch")
		}
		if len(whenArms[name]) > 0 {
			fmt.Fprintf(b, "%s%s_when{%q}:::term\n", indent, ids[name], "when")
		}
	}

	// Every node renders flat in registration order. forEach no longer wraps its branch in a box;
	// the "for each" edge label marks the replicating transition, the same way onError/goto are labeled.
	for _, t := range g.nodes {
		emitOneNode(t.Name, indent)
	}

	type edge struct {
		from string
		to   string
	}
	edgeOrder := []edge{}
	edgeLabels := map[edge][]string{}
	for _, tr := range g.transitions {
		// Switch and bare-When transitions are emitted via their source's
		// diamond below; skip them here so they don't double up as direct edges.
		if tr.Switch && !tr.WithGoto && !tr.OnError {
			continue
		}
		if tr.When != "" && tr.ForEach == "" && !tr.WithGoto && !tr.OnError && !tr.Switch {
			continue
		}
		e := edge{tr.From, tr.To}
		if _, ok := edgeLabels[e]; !ok {
			edgeOrder = append(edgeOrder, e)
			edgeLabels[e] = nil
		}
		var label string
		switch {
		case tr.ForEach != "":
			label = "for each"
		case tr.Switch:
			label = "case " + tr.When
		case tr.When != "":
			label = "when " + tr.When
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
		label := strings.Join(edgeLabels[e], " | ")
		srcExits := exits[e.from]
		if e.to == END {
			// Don't emit "_end" edges in the body. Forward to the caller via
			// endEdges, using each of the source's exit IDs (a subgraph source
			// has multiple).
			for _, src := range srcExits {
				endEdges = append(endEdges, endEdge{from: src, label: label})
			}
			continue
		}
		dstEntries := entries[e.to]
		for _, src := range srcExits {
			for _, dst := range dstEntries {
				if label != "" {
					fmt.Fprintf(b, "%s%s -->|%q| %s\n", indent, src, label, dst)
				} else {
					fmt.Fprintf(b, "%s%s --> %s\n", indent, src, dst)
				}
			}
		}
	}

	// Diamond routing: source -> diamond, then diamond -> each target with the
	// per-arm label. Iterate in registration order of the source nodes so the
	// emitted Mermaid is deterministic.
	emitDiamond := func(from string, suffix string, arms []diamondArm) {
		if len(arms) == 0 {
			return
		}
		diamondID := ids[from] + suffix
		for _, src := range exits[from] {
			fmt.Fprintf(b, "%s%s --> %s\n", indent, src, diamondID)
		}
		for _, arm := range arms {
			if arm.to == END {
				endEdges = append(endEdges, endEdge{from: diamondID, label: arm.label})
				continue
			}
			for _, dst := range entries[arm.to] {
				fmt.Fprintf(b, "%s%s -->|%q| %s\n", indent, diamondID, arm.label, dst)
			}
		}
	}
	for _, t := range g.nodes {
		emitDiamond(t.Name, "_switch", switchArms[t.Name])
		emitDiamond(t.Name, "_when", whenArms[t.Name])
	}

	if _, ok := ids[g.entryPoint]; ok {
		heads = entries[g.entryPoint]
	}
	return heads, endEdges
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
