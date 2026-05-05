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

package weirdapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "weird.fixture"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// Plain - port + path on the default safe trust.
	Plain = Def{Method: "GET", Route: ":443/plain"}
	// PathArg - single path argument.
	PathArg = Def{Method: "GET", Route: ":443/path-arg/{id}"}
	// GreedyArg - greedy tail path argument.
	GreedyArg = Def{Method: "GET", Route: ":443/greedy/{tail...}"}
	// PeriodInPath - period inside a segment, exercises encoding rule (. → _).
	PeriodInPath = Def{Method: "GET", Route: ":888/openapi.json"}
	// AnyMethod - method=ANY exercises the * encoding for the method slot.
	AnyMethod = Def{Method: "ANY", Route: ":443/any-method"}
	// InternalPort - :444 (internal-only port).
	InternalPort = Def{Method: "POST", Route: ":444/internal"}
	// TrustRoot - :666 trust-root endpoint exercises the danger trust segment.
	TrustRoot = Def{Method: "POST", Route: ":666/dangerous"}
	// SlashHostRoot - //root form, registers on the special "root" hostname.
	SlashHostRoot = Def{Method: "ANY", Route: "//root"}
	// SlashHostPort - //host:port/path form, registers on alt.host with port 0.
	SlashHostPort = Def{Method: "GET", Route: "//alt.host:0/alt-path"}
	// SlashHostPathArg - //host/path with path argument.
	SlashHostPathArg = Def{Method: "GET", Route: "//alt.host/items/{id}"}
	// SpecialHostRoute - absolute route on a hostname containing a URL-special character.
	// Exercises the route hostname's percent-encoding for non-dot specials (e.g. '$' → %24).
	SpecialHostRoute = Def{Method: "GET", Route: "//my$.xml/lookup"}
	// UpperCasePath - case-sensitive path segment with a period inside, exercises the
	// path encoder's per-byte %xx rule applied to '.' while uppercase letters pass through.
	UpperCasePath = Def{Method: "GET", Route: ":443/UPPERCASE.xml"}
	// OnSomething - outbound event on the conventional :417 port.
	OnSomething = Def{Method: "POST", Route: ":417/on-something"}
)

// PlainIn are the input arguments of Plain.
type PlainIn struct{}

// PlainOut are the output arguments of Plain.
type PlainOut struct {
	Result string `json:"result,omitzero"`
}

// PathArgIn are the input arguments of PathArg.
type PathArgIn struct {
	ID string `json:"id,omitzero"`
}

// PathArgOut are the output arguments of PathArg.
type PathArgOut struct{}

// GreedyArgIn are the input arguments of GreedyArg.
type GreedyArgIn struct {
	Tail string `json:"tail,omitzero"`
}

// GreedyArgOut are the output arguments of GreedyArg.
type GreedyArgOut struct{}

// PeriodInPathIn are the input arguments of PeriodInPath.
type PeriodInPathIn struct{}

// PeriodInPathOut are the output arguments of PeriodInPath.
type PeriodInPathOut struct{}

// AnyMethodIn are the input arguments of AnyMethod.
type AnyMethodIn struct{}

// AnyMethodOut are the output arguments of AnyMethod.
type AnyMethodOut struct{}

// InternalPortIn are the input arguments of InternalPort.
type InternalPortIn struct{}

// InternalPortOut are the output arguments of InternalPort.
type InternalPortOut struct{}

// TrustRootIn are the input arguments of TrustRoot.
type TrustRootIn struct {
	Cmd string `json:"cmd,omitzero"`
}

// TrustRootOut are the output arguments of TrustRoot.
type TrustRootOut struct{}

// SlashHostRootIn are the input arguments of SlashHostRoot.
type SlashHostRootIn struct{}

// SlashHostRootOut are the output arguments of SlashHostRoot.
type SlashHostRootOut struct{}

// SlashHostPortIn are the input arguments of SlashHostPort.
type SlashHostPortIn struct{}

// SlashHostPortOut are the output arguments of SlashHostPort.
type SlashHostPortOut struct{}

// SlashHostPathArgIn are the input arguments of SlashHostPathArg.
type SlashHostPathArgIn struct {
	ID string `json:"id,omitzero"`
}

// SlashHostPathArgOut are the output arguments of SlashHostPathArg.
type SlashHostPathArgOut struct{}

// SpecialHostRouteIn are the input arguments of SpecialHostRoute.
type SpecialHostRouteIn struct{}

// SpecialHostRouteOut are the output arguments of SpecialHostRoute.
type SpecialHostRouteOut struct{}

// UpperCasePathIn are the input arguments of UpperCasePath.
type UpperCasePathIn struct{}

// UpperCasePathOut are the output arguments of UpperCasePath.
type UpperCasePathOut struct{}

// OnSomethingIn are the input arguments of the outbound event.
type OnSomethingIn struct {
	Detail string `json:"detail,omitzero"`
}

// OnSomethingOut are the output arguments of the outbound event.
type OnSomethingOut struct {
	OK bool `json:"ok,omitzero"`
}
