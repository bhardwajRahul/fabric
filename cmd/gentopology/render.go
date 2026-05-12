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
	"sort"
	"strings"
)

// renderMermaid produces the topology .mmd content from the scanned
// services.
func renderMermaid(svcs []scanned) []byte {
	var sb strings.Builder
	sb.WriteString("graph LR\n")
	sb.WriteString("    classDef core fill:#ed2e92,color:#f4f2ef,stroke-width:0px\n")
	sb.WriteString("    classDef svc fill:#32a7c1,color:#f4f2ef,stroke-width:0px\n")
	sb.WriteString("    classDef danger fill:#f15922,color:#f4f2ef,stroke-width:0px\n")
	sb.WriteString("    classDef ext fill:#e5f4f3,color:#434343,stroke:#434343\n")
	sb.WriteByte('\n')

	// Services that expose at least one :666 endpoint. Edges pointing
	// into one of these get a "danger" label, and the node itself uses
	// the danger class instead of svc/core.
	danger := map[string]bool{}
	for _, s := range svcs {
		if s.Danger {
			danger[s.Hostname] = true
		}
	}

	// Order services by reverse-hostname so that nodes with the same
	// suffix group together (e.g. all .core services adjacent).
	ordered := append([]scanned(nil), svcs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return reverseHost(ordered[i].Hostname) < reverseHost(ordered[j].Hostname)
	})

	// Track which nodes we've emitted with a label, so we can use the
	// bare hostname (no label) on subsequent edge lines for the same
	// source.
	labeled := map[string]bool{}

	emitNode := func(host, name string) string {
		if labeled[host] {
			return host
		}
		labeled[host] = true
		return host + "[" + name + "<br>" + host + "]"
	}

	hostName := map[string]string{}
	for _, s := range svcs {
		hostName[s.Hostname] = s.Name
	}

	for _, s := range ordered {
		// Service-to-service edges: dependencies first, then events,
		// then SQL/cloud.
		emittedAny := false

		for _, dep := range s.Deps {
			depName := hostName[dep]
			if depName == "" {
				depName = dep
			}
			arrow := " ---> "
			if danger[dep] {
				arrow = " --->|danger| "
			}
			sb.WriteString("    ")
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteString(arrow)
			sb.WriteString(emitNode(dep, depName))
			sb.WriteByte('\n')
			emittedAny = true
		}
		for _, ev := range s.Events {
			evName := hostName[ev]
			if evName == "" {
				evName = ev
			}
			// The source service publishes; we draw the edge FROM the
			// source TO this subscriber so the diagram reads "events
			// flow this way."
			arrow := " -..-> "
			if danger[s.Hostname] {
				arrow = " -..->|danger| "
			}
			sb.WriteString("    ")
			sb.WriteString(emitNode(ev, evName))
			sb.WriteString(arrow)
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteByte('\n')
			emittedAny = true
		}
		if s.DB != "" {
			sb.WriteString("    ")
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteString(" --- ")
			sb.WriteString(s.Hostname)
			sb.WriteString(".db[(")
			sb.WriteString(s.DB)
			sb.WriteString(")]\n")
			emittedAny = true
		}
		if s.Py != "" {
			sb.WriteString("    ")
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteString(" --- ")
			sb.WriteString(s.Hostname)
			sb.WriteString(".py[[")
			sb.WriteString(s.Py)
			sb.WriteString("]]\n")
			emittedAny = true
		}
		if s.Cloud != "" {
			sb.WriteString("    ")
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteString(" --- ")
			sb.WriteString(s.Hostname)
			sb.WriteString(".cloud@{shape: cloud, label: \"")
			sb.WriteString(s.Cloud)
			sb.WriteString("\"}\n")
			emittedAny = true
		}
		// Standalone services: emit a bare node so they appear in the
		// graph even with no edges.
		if !emittedAny && !labeled[s.Hostname] {
			sb.WriteString("    ")
			sb.WriteString(emitNode(s.Hostname, s.Name))
			sb.WriteByte('\n')
		}
	}

	sb.WriteByte('\n')

	// Class assignments. Group by class for readable output. Danger
	// services override the svc/core classification so the orange fill
	// is what the operator sees regardless of hostname suffix.
	var coreNodes, svcNodes, dangerNodes, extNodes []string
	for _, s := range ordered {
		switch {
		case s.Danger:
			dangerNodes = append(dangerNodes, s.Hostname)
		case strings.HasSuffix(s.Hostname, ".core"):
			coreNodes = append(coreNodes, s.Hostname)
		default:
			svcNodes = append(svcNodes, s.Hostname)
		}
		if s.DB != "" {
			extNodes = append(extNodes, s.Hostname+".db")
		}
		if s.Py != "" {
			extNodes = append(extNodes, s.Hostname+".py")
		}
		if s.Cloud != "" {
			extNodes = append(extNodes, s.Hostname+".cloud")
		}
	}
	if len(svcNodes) > 0 {
		sb.WriteString("    class ")
		sb.WriteString(strings.Join(svcNodes, ","))
		sb.WriteString(" svc\n")
	}
	if len(coreNodes) > 0 {
		sb.WriteString("    class ")
		sb.WriteString(strings.Join(coreNodes, ","))
		sb.WriteString(" core\n")
	}
	if len(dangerNodes) > 0 {
		sb.WriteString("    class ")
		sb.WriteString(strings.Join(dangerNodes, ","))
		sb.WriteString(" danger\n")
	}
	if len(extNodes) > 0 {
		sb.WriteString("    class ")
		sb.WriteString(strings.Join(extNodes, ","))
		sb.WriteString(" ext\n")
	}

	return []byte(sb.String())
}

// reverseHost reverses a dot-separated hostname so equal-suffix names
// sort adjacent (e.g. "hello.example" → "example.hello").
func reverseHost(h string) string {
	parts := strings.Split(h, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}
