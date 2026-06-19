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
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "weird.fixture"

// Name is the decorative PascalCase name of the microservice.
const Name = "Weird"

// Version is the major version of the microservice's public API.
const Version = 1

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Weird is a gencreds fixture exercising every route shape.`

// OnSomething is fired when something happens.
var OnSomething = define.OutboundEvent{ // MARKER: OnSomething
	Host: Hostname, Method: "POST", Route: ":417/on-something",
	In: OnSomethingIn{}, Out: OnSomethingOut{},
}

// OnSomethingIn are the input arguments of the outbound event.
type OnSomethingIn struct {
	Detail string `json:"detail,omitzero"`
}

// OnSomethingOut are the output arguments of the outbound event.
type OnSomethingOut struct {
	OK bool `json:"ok,omitzero"`
}

// Plain is a baseline function on the safe trust segment.
var Plain = define.Function{ // MARKER: Plain
	Host: Hostname, Method: "GET", Route: ":443/plain",
	TimeBudget: 30 * time.Second,
	In:         PlainIn{}, Out: PlainOut{},
}

// PlainIn are the input arguments of Plain.
type PlainIn struct{}

// PlainOut are the output arguments of Plain.
type PlainOut struct {
	Result string `json:"result,omitzero"`
}

// PathArg accepts a path argument.
var PathArg = define.Function{ // MARKER: PathArg
	Host: Hostname, Method: "GET", Route: ":443/path-arg/{id}",
	In: PathArgIn{}, Out: PathArgOut{},
}

// PathArgIn are the input arguments of PathArg.
type PathArgIn struct {
	ID string `json:"id,omitzero"`
}

// PathArgOut are the output arguments of PathArg.
type PathArgOut struct{}

// GreedyArg accepts a greedy tail path argument.
var GreedyArg = define.Function{ // MARKER: GreedyArg
	Host: Hostname, Method: "GET", Route: ":443/greedy/{tail...}",
	In: GreedyArgIn{}, Out: GreedyArgOut{},
}

// GreedyArgIn are the input arguments of GreedyArg.
type GreedyArgIn struct {
	Tail string `json:"tail,omitzero"`
}

// GreedyArgOut are the output arguments of GreedyArg.
type GreedyArgOut struct{}

// PeriodInPath has a period inside its path segment.
var PeriodInPath = define.Function{ // MARKER: PeriodInPath
	Host: Hostname, Method: "GET", Route: ":888/openapi.json",
	In: PeriodInPathIn{}, Out: PeriodInPathOut{},
}

// PeriodInPathIn are the input arguments of PeriodInPath.
type PeriodInPathIn struct{}

// PeriodInPathOut are the output arguments of PeriodInPath.
type PeriodInPathOut struct{}

// AnyMethod accepts any HTTP method.
var AnyMethod = define.Function{ // MARKER: AnyMethod
	Host: Hostname, Method: "ANY", Route: ":443/any-method",
	In: AnyMethodIn{}, Out: AnyMethodOut{},
}

// AnyMethodIn are the input arguments of AnyMethod.
type AnyMethodIn struct{}

// AnyMethodOut are the output arguments of AnyMethod.
type AnyMethodOut struct{}

// InternalPort is on the :444 internal port.
var InternalPort = define.Function{ // MARKER: InternalPort
	Host: Hostname, Method: "POST", Route: ":444/internal",
	In: InternalPortIn{}, Out: InternalPortOut{},
}

// InternalPortIn are the input arguments of InternalPort.
type InternalPortIn struct{}

// InternalPortOut are the output arguments of InternalPort.
type InternalPortOut struct{}

// TrustRoot is the trust-root :666 endpoint.
var TrustRoot = define.Function{ // MARKER: TrustRoot
	Host: Hostname, Method: "POST", Route: ":666/dangerous",
	In: TrustRootIn{}, Out: TrustRootOut{},
}

// TrustRootIn are the input arguments of TrustRoot.
type TrustRootIn struct {
	Cmd string `json:"cmd,omitzero"`
}

// TrustRootOut are the output arguments of TrustRoot.
type TrustRootOut struct{}

// SlashHostRoot has route "//root".
var SlashHostRoot = define.Function{ // MARKER: SlashHostRoot
	Host: Hostname, Method: "ANY", Route: "//root",
	In: SlashHostRootIn{}, Out: SlashHostRootOut{},
}

// SlashHostRootIn are the input arguments of SlashHostRoot.
type SlashHostRootIn struct{}

// SlashHostRootOut are the output arguments of SlashHostRoot.
type SlashHostRootOut struct{}

// SlashHostPort has route "//alt.host:0/alt-path".
var SlashHostPort = define.Function{ // MARKER: SlashHostPort
	Host: Hostname, Method: "GET", Route: "//alt.host:0/alt-path",
	In: SlashHostPortIn{}, Out: SlashHostPortOut{},
}

// SlashHostPortIn are the input arguments of SlashHostPort.
type SlashHostPortIn struct{}

// SlashHostPortOut are the output arguments of SlashHostPort.
type SlashHostPortOut struct{}

// SlashHostPathArg has route "//alt.host/items/{id}".
var SlashHostPathArg = define.Function{ // MARKER: SlashHostPathArg
	Host: Hostname, Method: "GET", Route: "//alt.host/items/{id}",
	In: SlashHostPathArgIn{}, Out: SlashHostPathArgOut{},
}

// SlashHostPathArgIn are the input arguments of SlashHostPathArg.
type SlashHostPathArgIn struct {
	ID string `json:"id,omitzero"`
}

// SlashHostPathArgOut are the output arguments of SlashHostPathArg.
type SlashHostPathArgOut struct{}

// SpecialHostRoute has route "//my$.xml/lookup" - URL-special character in the hostname segment.
var SpecialHostRoute = define.Function{ // MARKER: SpecialHostRoute
	Host: Hostname, Method: "GET", Route: "//my$.xml/lookup",
	In: SpecialHostRouteIn{}, Out: SpecialHostRouteOut{},
}

// SpecialHostRouteIn are the input arguments of SpecialHostRoute.
type SpecialHostRouteIn struct{}

// SpecialHostRouteOut are the output arguments of SpecialHostRoute.
type SpecialHostRouteOut struct{}

// UpperCasePath has route ":443/UPPERCASE.xml" - uppercase path segment with a period.
var UpperCasePath = define.Function{ // MARKER: UpperCasePath
	Host: Hostname, Method: "GET", Route: ":443/UPPERCASE.xml",
	In: UpperCasePathIn{}, Out: UpperCasePathOut{},
}

// UpperCasePathIn are the input arguments of UpperCasePath.
type UpperCasePathIn struct{}

// UpperCasePathOut are the output arguments of UpperCasePath.
type UpperCasePathOut struct{}
