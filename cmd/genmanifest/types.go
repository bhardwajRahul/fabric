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

// Manifest is the in-memory representation of a microservice's manifest.yaml.
// The shape is a superset of cmd/internal/schema's Manifest - that package keeps
// a narrow consumer-side shape; this one carries every field the tool emits.
type Manifest struct {
	General        General
	Configs        []Config
	Metrics        []Metric
	OutboundEvents []Endpoint
	Functions      []Endpoint
	Webs           []Endpoint
	InboundEvents  []InboundEvent
	Tasks          []Endpoint
	Workflows      []Endpoint
	Tickers        []Ticker
}

// General is the manifest's identity block.
type General struct {
	Name             string
	Hostname         string
	Description      string
	Package          string
	FrameworkVersion string
	ModifiedAt       string
}

// Endpoint is a reachable surface - function, web handler, task, workflow, or
// outbound event. Not every field applies to every kind, but the set is small
// enough that one shape is simpler than four.
type Endpoint struct {
	Name           string
	Signature      string // empty for webs (which describe themselves with method+route only)
	Description    string
	Method         string
	Route          string
	LoadBalancing  string // "" → omit (default), "none" → NoQueue, custom queue name otherwise
	RequiredClaims string
	TimeBudget     string // declared sub.TimeBudget as a compact duration (e.g. "50ms"); "" → omit
}

// InboundEvent is a hook into another service's outbound event. The
// resolved route/method/hostname are intentionally absent - gencreds
// derives them from the source *api at deploy time.
type InboundEvent struct {
	Name           string
	Signature      string
	Description    string
	LoadBalancing  string
	RequiredClaims string
	Package        string // package path of the event source
}

// Config is a runtime configuration property.
type Config struct {
	Name        string
	Signature   string
	Description string
	Validation  string
	Default     string
	Secret      bool
	Callback    bool
}

// Metric describes a counter, gauge, or histogram.
type Metric struct {
	Name        string // local name (e.g. FlowsStarted)
	Signature   string
	Description string
	Kind        string // counter | gauge | histogram
	Buckets     []string
	OtelName    string
	Observable  bool
}

// Ticker is a recurring operation.
type Ticker struct {
	Name        string
	Signature   string
	Description string
	Interval    string
}
