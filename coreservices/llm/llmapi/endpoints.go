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

package llmapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "llm.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// ChatIn are the input arguments of Chat.
type ChatIn struct { // MARKER: Chat
	Messages []Message `json:"messages,omitzero"`
	Tools    []string  `json:"tools,omitzero"`
}

// ChatOut are the output arguments of Chat.
type ChatOut struct { // MARKER: Chat
	MessagesOut []Message `json:"messagesOut,omitzero"`
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Messages []Message `json:"messages,omitzero"`
	Tools    []Tool    `json:"tools,omitzero"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	Completion *TurnCompletion `json:"completion,omitzero"`
}

// InitChatIn are the input arguments of InitChat.
type InitChatIn struct { // MARKER: InitChat
	Messages []Message `json:"messages,omitzero"`
	Tools    []Tool    `json:"tools,omitzero"`
}

// InitChatOut are the output arguments of InitChat.
type InitChatOut struct { // MARKER: InitChat
	MaxToolRounds int `json:"maxToolRounds,omitzero"`
	ToolRounds    int `json:"toolRounds,omitzero"`
}

// CallLLMIn are the input arguments of CallLLM.
type CallLLMIn struct { // MARKER: CallLLM
	Messages []Message `json:"messages,omitzero"`
}

// CallLLMOut are the output arguments of CallLLM.
type CallLLMOut struct { // MARKER: CallLLM
	LLMContent       string `json:"llmContent,omitzero"`
	PendingToolCalls any    `json:"pendingToolCalls,omitzero"`
}

// ProcessResponseIn are the input arguments of ProcessResponse.
type ProcessResponseIn struct { // MARKER: ProcessResponse
	LLMContent    string `json:"llmContent,omitzero"`
	ToolRounds    int    `json:"toolRounds,omitzero"`
	MaxToolRounds int    `json:"maxToolRounds,omitzero"`
}

// ProcessResponseOut are the output arguments of ProcessResponse.
type ProcessResponseOut struct { // MARKER: ProcessResponse
	MessagesOut    []Message `json:"messages,omitzero"`
	ToolsRequested bool      `json:"toolsRequested,omitzero"`
	ToolRoundsOut  int       `json:"toolRounds,omitzero"`
}

// ExecuteToolIn are the input arguments of ExecuteTool.
type ExecuteToolIn struct { // MARKER: ExecuteTool
	ToolExecuted bool `json:"toolExecuted,omitzero"`
}

// ExecuteToolOut are the output arguments of ExecuteTool.
type ExecuteToolOut struct { // MARKER: ExecuteTool
	ToolExecutedOut bool `json:"toolExecuted,omitzero"`
}

// ChatLoopIn are the input arguments of ChatLoop.
type ChatLoopIn struct { // MARKER: ChatLoop
	Messages []Message `json:"messages,omitzero"`
	Tools    []Tool    `json:"tools,omitzero"`
}

// ChatLoopOut are the output arguments of ChatLoop.
type ChatLoopOut struct { // MARKER: ChatLoop
	MessagesOut []Message `json:"messages,omitzero"`
}

var (
	Chat            = Def{Method: "POST", Route: ":444/chat"}             // MARKER: Chat
	Turn            = Def{Method: "POST", Route: ":444/turn"}             // MARKER: Turn
	InitChat        = Def{Method: "POST", Route: ":428/init-chat"}        // MARKER: InitChat
	CallLLM         = Def{Method: "POST", Route: ":428/call-llm"}         // MARKER: CallLLM
	ProcessResponse = Def{Method: "POST", Route: ":428/process-response"} // MARKER: ProcessResponse
	ExecuteTool     = Def{Method: "POST", Route: ":428/execute-tool"}     // MARKER: ExecuteTool
	ChatLoop        = Def{Method: "GET", Route: ":428/chat-loop"}         // MARKER: ChatLoop
)
