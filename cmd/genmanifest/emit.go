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

package main

import (
	"strings"
)

// emit serializes m to the canonical manifest.yaml form. The leading comment
// block of the existing file (everything before the first non-comment
// non-blank line) is reused verbatim if present - this preserves operator-
// authored license headers and any preamble comments. Fresh manifests, and
// manifests whose existing file has no header comment block, get no header.
func emit(m *Manifest, existingBytes []byte) []byte {
	var sb strings.Builder
	if h := headerOf(existingBytes); h != "" {
		sb.WriteString(h)
		sb.WriteByte('\n')
	}
	emitGeneral(&sb, &m.General)
	emitConfigs(&sb, m.Configs)
	emitMetrics(&sb, m.Metrics)
	emitOutboundEvents(&sb, m.OutboundEvents)
	emitFunctions(&sb, m.Functions)
	emitWebs(&sb, m.Webs)
	emitInboundEvents(&sb, m.InboundEvents)
	emitTasks(&sb, m.Tasks)
	emitWorkflows(&sb, m.Workflows)
	emitTickers(&sb, m.Tickers)
	return []byte(sb.String())
}

// headerOf returns the leading comment block from existingBytes (everything up
// to but not including the first blank line that follows comments). Returns
// "" when the file is absent or has no leading comment block - the tool does
// not synthesize a license header.
func headerOf(existingBytes []byte) string {
	if len(existingBytes) == 0 {
		return ""
	}
	lines := strings.SplitAfter(string(existingBytes), "\n")
	var sb strings.Builder
	sawComment := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			sb.WriteString(line)
			sawComment = true
			continue
		}
		if trim == "" {
			if sawComment {
				// First blank after comments terminates the header. We do NOT
				// include the blank - the section emitters add their own.
				return sb.String()
			}
			sb.WriteString(line)
			continue
		}
		// Hit content: stop.
		break
	}
	if sawComment {
		return sb.String()
	}
	return ""
}

func emitGeneral(sb *strings.Builder, g *General) {
	sb.WriteString("general:\n")
	if g.Name != "" {
		writeKV(sb, "  ", "name", g.Name)
	}
	if g.Hostname != "" {
		writeKV(sb, "  ", "hostname", g.Hostname)
	}
	if g.Description != "" {
		writeKV(sb, "  ", "description", g.Description)
	}
	if g.Package != "" {
		writeKV(sb, "  ", "package", g.Package)
	}
	if g.FrameworkVersion != "" {
		writeKV(sb, "  ", "frameworkVersion", g.FrameworkVersion)
	}
	if g.ModifiedAt != "" {
		// modifiedAt is emitted with quotes (it's a timestamp).
		sb.WriteString("  modifiedAt: \"")
		sb.WriteString(g.ModifiedAt)
		sb.WriteString("\"\n")
	}
}

func emitConfigs(sb *strings.Builder, cfgs []Config) {
	if len(cfgs) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("configs:\n")
	for _, c := range cfgs {
		sb.WriteString("  ")
		sb.WriteString(c.Name)
		sb.WriteString(":\n")
		if c.Signature != "" {
			writeKV(sb, "    ", "signature", c.Signature)
		}
		if c.Description != "" {
			writeKV(sb, "    ", "description", c.Description)
		}
		if c.Validation != "" {
			writeKV(sb, "    ", "validation", c.Validation)
		}
		if c.Default != "" {
			writeKV(sb, "    ", "default", c.Default)
		}
		if c.Secret {
			sb.WriteString("    secret: true\n")
		}
		if c.Callback {
			sb.WriteString("    callback: true\n")
		}
	}
}

func emitMetrics(sb *strings.Builder, ms []Metric) {
	if len(ms) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("metrics:\n")
	for _, m := range ms {
		sb.WriteString("  ")
		sb.WriteString(m.Name)
		sb.WriteString(":\n")
		if m.Signature != "" {
			writeKV(sb, "    ", "signature", m.Signature)
		}
		if m.Description != "" {
			writeKV(sb, "    ", "description", m.Description)
		}
		if m.Kind != "" {
			writeKV(sb, "    ", "kind", m.Kind)
		}
		if len(m.Buckets) > 0 {
			sb.WriteString("    buckets: [")
			sb.WriteString(strings.Join(m.Buckets, ", "))
			sb.WriteString("]\n")
		}
		if m.OtelName != "" {
			writeKV(sb, "    ", "otelName", m.OtelName)
		}
		if m.Observable {
			sb.WriteString("    observable: true\n")
		}
	}
}

func emitOutboundEvents(sb *strings.Builder, evs []Endpoint) {
	if len(evs) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("outboundEvents:\n")
	for _, e := range evs {
		emitFunctionLike(sb, e)
	}
}

func emitFunctions(sb *strings.Builder, fns []Endpoint) {
	if len(fns) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("functions:\n")
	for _, f := range fns {
		emitFunctionLike(sb, f)
	}
}

func emitWebs(sb *strings.Builder, webs []Endpoint) {
	if len(webs) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("webs:\n")
	for _, w := range webs {
		sb.WriteString("  ")
		sb.WriteString(w.Name)
		sb.WriteString(":\n")
		if w.Description != "" {
			writeKV(sb, "    ", "description", w.Description)
		}
		if w.Method != "" {
			writeKV(sb, "    ", "method", w.Method)
		}
		if w.Route != "" {
			writeKV(sb, "    ", "route", w.Route)
		}
		if w.LoadBalancing != "" && w.LoadBalancing != "default" {
			writeKV(sb, "    ", "loadBalancing", w.LoadBalancing)
		}
		if w.RequiredClaims != "" {
			writeKV(sb, "    ", "requiredClaims", w.RequiredClaims)
		}
	}
}

func emitInboundEvents(sb *strings.Builder, evs []InboundEvent) {
	if len(evs) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("inboundEvents:\n")
	for _, e := range evs {
		sb.WriteString("  ")
		sb.WriteString(e.Name)
		sb.WriteString(":\n")
		if e.Signature != "" {
			writeKV(sb, "    ", "signature", e.Signature)
		}
		if e.Description != "" {
			writeKV(sb, "    ", "description", e.Description)
		}
		if e.LoadBalancing != "" && e.LoadBalancing != "default" {
			writeKV(sb, "    ", "loadBalancing", e.LoadBalancing)
		}
		if e.RequiredClaims != "" {
			writeKV(sb, "    ", "requiredClaims", e.RequiredClaims)
		}
		if e.Package != "" {
			writeKV(sb, "    ", "package", e.Package)
		}
	}
}

func emitTasks(sb *strings.Builder, tasks []Endpoint) {
	if len(tasks) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("tasks:\n")
	for _, t := range tasks {
		sb.WriteString("  ")
		sb.WriteString(t.Name)
		sb.WriteString(":\n")
		if t.Signature != "" {
			writeKV(sb, "    ", "signature", t.Signature)
		}
		if t.Description != "" {
			writeKV(sb, "    ", "description", t.Description)
		}
		if t.Route != "" {
			writeKV(sb, "    ", "route", t.Route)
		}
		if t.RequiredClaims != "" {
			writeKV(sb, "    ", "requiredClaims", t.RequiredClaims)
		}
	}
}

func emitWorkflows(sb *strings.Builder, wfs []Endpoint) {
	if len(wfs) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("workflows:\n")
	for _, w := range wfs {
		sb.WriteString("  ")
		sb.WriteString(w.Name)
		sb.WriteString(":\n")
		if w.Signature != "" {
			writeKV(sb, "    ", "signature", w.Signature)
		}
		if w.Description != "" {
			writeKV(sb, "    ", "description", w.Description)
		}
		if w.Route != "" {
			writeKV(sb, "    ", "route", w.Route)
		}
	}
}

func emitTickers(sb *strings.Builder, tks []Ticker) {
	if len(tks) == 0 {
		return
	}
	sb.WriteByte('\n')
	sb.WriteString("tickers:\n")
	for _, t := range tks {
		sb.WriteString("  ")
		sb.WriteString(t.Name)
		sb.WriteString(":\n")
		if t.Signature != "" {
			writeKV(sb, "    ", "signature", t.Signature)
		}
		if t.Description != "" {
			writeKV(sb, "    ", "description", t.Description)
		}
		if t.Interval != "" {
			writeKV(sb, "    ", "interval", t.Interval)
		}
	}
}

// emitFunctionLike emits a function/outboundEvent-style entry: signature,
// description, method, route, loadBalancing, requiredClaims.
func emitFunctionLike(sb *strings.Builder, e Endpoint) {
	sb.WriteString("  ")
	sb.WriteString(e.Name)
	sb.WriteString(":\n")
	if e.Signature != "" {
		writeKV(sb, "    ", "signature", e.Signature)
	}
	if e.Description != "" {
		writeKV(sb, "    ", "description", e.Description)
	}
	if e.Method != "" {
		writeKV(sb, "    ", "method", e.Method)
	}
	if e.Route != "" {
		writeKV(sb, "    ", "route", e.Route)
	}
	if e.LoadBalancing != "" && e.LoadBalancing != "default" {
		writeKV(sb, "    ", "loadBalancing", e.LoadBalancing)
	}
	if e.RequiredClaims != "" {
		writeKV(sb, "    ", "requiredClaims", e.RequiredClaims)
	}
}

// writeKV writes a `key: value` pair, applying quoting only when YAML requires
// it. Multi-line values are emitted as block scalars.
func writeKV(sb *strings.Builder, indent, key, value string) {
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(": ")
	if strings.Contains(value, "\n") {
		// Use literal block scalar to preserve formatting.
		sb.WriteString("|-\n")
		for _, line := range strings.Split(value, "\n") {
			sb.WriteString(indent)
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
		return
	}
	if needsQuoting(value) {
		sb.WriteByte('"')
		sb.WriteString(escapeYAMLString(value))
		sb.WriteByte('"')
	} else {
		sb.WriteString(value)
	}
	sb.WriteByte('\n')
}

// needsQuoting returns true if a YAML scalar cannot be emitted bare. We're
// conservative - quote when a value starts/ends with whitespace, contains `: `,
// `#`, or starts with a YAML reserved character. Most manifest values are
// plain identifiers/sentences and don't need quoting.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	if s != strings.TrimSpace(s) {
		return true
	}
	switch s[0] {
	case '!', '&', '*', '?', '|', '>', '\'', '"', '%', '@', '`', '[', '{':
		return true
	}
	// Reserved scalar values.
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off":
		return true
	}
	if strings.Contains(s, ": ") {
		return true
	}
	if strings.HasSuffix(s, ":") {
		return true
	}
	if strings.Contains(s, " #") {
		return true
	}
	return false
}

// escapeYAMLString applies the minimal escaping needed inside a double-quoted
// YAML scalar.
func escapeYAMLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
