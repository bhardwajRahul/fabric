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

package claudellmapi

import (
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "claude.llm.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Messages []llmapi.Message `json:"messages,omitzero"`
	Tools    []llmapi.Tool    `json:"tools,omitzero"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	Completion *llmapi.TurnCompletion `json:"completion,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Turn = Def{Method: "POST", Route: ":444/turn"} // MARKER: Turn
)
