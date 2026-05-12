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

// Package schema holds the typed shapes that the cmd/* tools share.
// Manifest parsing lives here so cmd/genmanifest and cmd/gencreds read
// the same struct definitions.
package schema

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// Manifest is the subset of manifest.yaml that the cmd/* tools depend on.
// Fields not used by any tool are intentionally omitted; YAML unmarshal
// silently ignores unknown keys, so adding fields here is non-breaking.
type Manifest struct {
	General        General             `yaml:"general"`
	Webs           map[string]Route    `yaml:"webs"`
	Functions      map[string]Route    `yaml:"functions"`
	Tasks          map[string]Route    `yaml:"tasks"`
	Workflows      map[string]Route    `yaml:"workflows"`
	OutboundEvents map[string]Route    `yaml:"outboundEvents"`
	InboundEvents  map[string]EventSub `yaml:"inboundEvents"`
	Downstream     []Downstream        `yaml:"downstream"`
}

// General is the manifest's identity block.
type General struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Package  string `yaml:"package"`
}

// Route is a registered endpoint on the bus.
type Route struct {
	Method string `yaml:"method"`
	Route  string `yaml:"route"`
}

// EventSub captures the source of an inbound event subscription. The
// event name is the YAML map key (e.g. "OnOrderCreated"). The resolved
// hostname/route/method are intentionally omitted - they're derived
// from source by gencreds at deploy time.
type EventSub struct {
	Package string `yaml:"package"`
}

// Downstream is a service this one calls into via its *api Client. Only
// the hostname and package are recorded - the per-call endpoint set
// (route, method, hostname overrides) is derived from source by
// gencreds at deploy time, not stored in the manifest. This keeps
// callers' manifests stable across callee Def renames.
type Downstream struct {
	Hostname string `yaml:"hostname"`
	Package  string `yaml:"package,omitempty"`
}

// ReadManifest parses manifest.yaml from the given path.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.General.Hostname == "" {
		return nil, fmt.Errorf("manifest %s: general.hostname is empty", path)
	}
	return &m, nil
}

// ExposedRoutes returns all caller-reachable routes (webs + functions + tasks + workflows).
func (m *Manifest) ExposedRoutes() []Route {
	out := make([]Route, 0,
		len(m.Webs)+len(m.Functions)+len(m.Tasks)+len(m.Workflows))
	for _, r := range m.Webs {
		out = append(out, r)
	}
	for _, r := range m.Functions {
		out = append(out, r)
	}
	for _, r := range m.Tasks {
		out = append(out, r)
	}
	for _, r := range m.Workflows {
		out = append(out, r)
	}
	return out
}
