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

/*
Package define is the vocabulary for a microservice's api package definition.go, where every feature the
microservice exposes is declared as a define.* var: Function, Web, Task, Workflow, OutboundEvent,
InboundEvent, Config, Metric, and Ticker. cmd/genservice generates the rest of the microservice from
these declarations.

Write only statically resolvable values: literals, the In/Out/Value type carriers (e.g. In: FooIn{},
Value: int(0)), and references to other define.* vars (e.g. an InboundEvent's Source). Design rationale
is in CLAUDE.md.
*/
package define

import (
	"strings"
	"time"
)

// LoadBalancing values for an endpoint's queue behavior. The empty string (the default) uses a
// queue named after the hostname, load-balancing requests among all peers.
const (
	Default = "default" // explicit form of the hostname-named queue: load-balanced among all peers
	None    = "none"    // no queue: multicast to all peers
)

// Metric kinds.
const (
	Counter   = "counter"
	Gauge     = "gauge"
	Histogram = "histogram"
)

// Function is a unicast or multicast request-response RPC endpoint.
type Function struct {
	Host           string        // the api package's Hostname const
	Method         string        // GET, POST, ..., or ANY
	Route          string        // e.g. ":443/my-func" or "//host/path"
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
	In             any           // the FooIn{} struct, as a type carrier
	Out            any           // the FooOut{} struct, as a type carrier
}

// URL is the full URL of the endpoint, joined with its Host.
func (f Function) URL() string { return joinHostAndPath(f.Host, f.Route) }

// Web is a raw http.ResponseWriter / *http.Request handler endpoint. It carries no In/Out.
type Web struct {
	Host           string        // the api package's Hostname const
	Method         string        // GET, POST, ..., or ANY
	Route          string        // e.g. ":443/my-func" or "//host/path"
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
}

// URL is the full URL of the endpoint, joined with its Host.
func (w Web) URL() string { return joinHostAndPath(w.Host, w.Route) }

// Task is a workflow task endpoint, invoked with a *workflow.Flow carrier.
type Task struct {
	Host           string        // the api package's Hostname const
	Method         string        // GET, POST, ..., or ANY
	Route          string        // e.g. ":443/my-func" or "//host/path"
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
	In             any           // the FooIn{} struct, as a type carrier
	Out            any           // the FooOut{} struct, as a type carrier
}

// URL is the full URL of the endpoint, joined with its Host.
func (t Task) URL() string { return joinHostAndPath(t.Host, t.Route) }

// Workflow is a workflow graph endpoint.
type Workflow struct {
	Host           string        // the api package's Hostname const
	Method         string        // GET, POST, ..., or ANY
	Route          string        // e.g. ":443/my-func" or "//host/path"
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
	In             any           // the FooIn{} struct, as a type carrier
	Out            any           // the FooOut{} struct, as a type carrier
}

// URL is the full URL of the endpoint, joined with its Host.
func (g Workflow) URL() string { return joinHostAndPath(g.Host, g.Route) }

// OutboundEvent is an event this microservice fires for subscribers to consume.
type OutboundEvent struct {
	Host           string        // the api package's Hostname const
	Method         string        // GET, POST, ..., or ANY
	Route          string        // e.g. ":417/on-my-event"
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
	In             any           // the FooIn{} struct, as a type carrier
	Out            any           // the FooOut{} struct, as a type carrier
}

// URL is the full URL of the endpoint, joined with its Host.
func (e OutboundEvent) URL() string { return joinHostAndPath(e.Host, e.Route) }

// InboundEvent subscribes this microservice to an event fired by another. Source references the
// source microservice's OutboundEvent var, so a removed or renamed event is a compile error here.
// The Service method that handles the event is named after the InboundEvent var.
type InboundEvent struct {
	Source         OutboundEvent // the source microservice's OutboundEvent var
	RequiredClaims string        // boolean expression over JWT claims; empty means open
	TimeBudget     time.Duration // per-endpoint max duration; zero means the framework default
	LoadBalancing  string        // "" (default), define.None, or a custom queue name
}

// Config is a runtime configuration property, sourced as a string then converted to Value's Go type.
type Config struct {
	Value      any    // getter return type carrier: string(""), int(0), ..., time.Duration(0), or MyStruct{}
	Default    string // raw default string, as it would appear in YAML; empty means no default
	Validation string // cfg validation applied to the raw string, e.g. "int [1,]", "url", "json"
	Secret     bool   // never logged
	Callback   bool   // OnChanged<Name> fires when the value changes
}

// Metric is a counter, gauge, or histogram.
type Metric struct {
	Kind       string    // define.Counter | define.Gauge | define.Histogram
	Value      any       // recorder value type carrier: a numeric conversion, e.g. int(0) or float64(0)
	Labels     []string  // label parameter names (string-typed)
	Buckets    []float64 // histogram bucket boundaries
	OTelName   string    // the OpenTelemetry metric name
	Observable bool      // measured just-in-time via the OnObserve<Name> callback
}

// Ticker runs a recurring operation on a schedule.
type Ticker struct {
	Interval time.Duration // duration between iterations
}

// joinHostAndPath mirrors httpx.JoinHostAndPath.
func joinHostAndPath(host string, route string) string {
	if route == "" {
		// (empty)
		return "https://" + host
	}
	if strings.HasPrefix(route, ":") {
		// :1080/path
		return "https://" + host + route
	}
	if strings.HasPrefix(route, "//") {
		// //host.name/path/with/slash
		return "https:" + route
	}
	if strings.HasPrefix(route, "/") {
		// /path/with/slash
		return "https://" + host + route
	}
	if !strings.Contains(route, "://") {
		// path/with/no/slash
		return "https://" + host + "/" + route
	}
	return route
}
