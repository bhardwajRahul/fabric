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

package kitchenapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "kitchen.fixture"

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
	// MyFunc - kitchen's own function endpoint.
	MyFunc = Def{Method: "POST", Route: ":443/my-func"}
	// SelfPing - used by kitchen for peer-to-peer self-calls.
	SelfPing = Def{Method: "POST", Route: ":444/self-ping"}
	// AltHostFn - registers on a non-own host via the //host:port/path form.
	// The connector subscribes on host="alt.kitchen" rather than the service's
	// own hostname; the ACL must reflect that.
	AltHostFn = Def{Method: "GET", Route: "//alt.kitchen:0/alt-fn"}
	// OnMyEvent - kitchen's outbound event.
	OnMyEvent = Def{Method: "POST", Route: ":417/on-my-event"}
)

// MyFuncIn are the input arguments of MyFunc.
type MyFuncIn struct {
	Input string `json:"input,omitzero"`
}

// MyFuncOut are the output arguments of MyFunc.
type MyFuncOut struct {
	Output string `json:"output,omitzero"`
}

// SelfPingIn are the input arguments of SelfPing.
type SelfPingIn struct{}

// SelfPingOut are the output arguments of SelfPing.
type SelfPingOut struct{}

// AltHostFnIn are the input arguments of AltHostFn.
type AltHostFnIn struct{}

// AltHostFnOut are the output arguments of AltHostFn.
type AltHostFnOut struct{}

// OnMyEventIn are the input arguments of OnMyEvent.
type OnMyEventIn struct {
	Note string `json:"note,omitzero"`
}

// OnMyEventOut are the output arguments of OnMyEvent.
type OnMyEventOut struct {
	OK bool `json:"ok,omitzero"`
}
