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
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "llm.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "LLM"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 12

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations.`

// MaxToolRounds is the maximum number of tool call round-trips per invocation.
var MaxToolRounds = define.Config{ // MARKER: MaxToolRounds
	Value:      int(0),
	Default:    "10",
	Validation: "int [1,50]",
}

// LLMTokens counts tokens consumed per LLM turn
var LLMTokens = define.Metric{ // MARKER: LLMTokens
	Kind: define.Counter, Value: int(0), Labels: []string{"provider", "model", "direction"},
	OTelName: "microbus_llm_tokens_total",
}

// Chat sends messages to an LLM with optional tools, looping through tool calls until the LLM returns a final answer.
//
// Input:
//   - provider: provider is the hostname of the LLM provider microservice to use
//   - model: model is the provider-specific model identifier
//   - messages: messages is the conversation history to send to the LLM
//   - toolURLs: toolURLs is the list of Microbus endpoint URLs exposed to the LLM
//   - options: options configures tool-call rounds, max tokens, temperature (nil = defaults)
//
// Output:
//   - messagesOut: messagesOut is the full conversation including new messages produced by the LLM
//   - usage: usage is the aggregate token consumption across all turns
var Chat = define.Function{ // MARKER: Chat
	Host: Hostname, Method: "POST", Route: ":444/chat",
	In: ChatIn{}, Out: ChatOut{},
}

// ChatIn are the input arguments of Chat.
type ChatIn struct { // MARKER: Chat
	Provider string       `json:"provider,omitzero"`
	Model    string       `json:"model,omitzero"`
	Messages []Message    `json:"messages,omitzero"`
	ToolURLs []string     `json:"toolURLs,omitzero"`
	Options  *ChatOptions `json:"options,omitzero"`
}

// ChatOut are the output arguments of Chat.
type ChatOut struct { // MARKER: Chat
	MessagesOut []Message `json:"messagesOut,omitzero"`
	Usage       Usage     `json:"usage,omitzero"`
}

// Turn executes a single LLM turn. On llm.core this returns 501 Not Implemented; the actual implementation lives in each provider microservice (claudellm, chatgptllm, geminillm).
//
// Input:
//   - model: model is the provider-specific model identifier
//   - messages: messages is the conversation history to send to the LLM
//   - tools: tools is the resolved tool definitions with schemas
//   - options: options configures max tokens and temperature (nil = provider defaults)
//
// Output:
//   - content: content is the LLM's text response, if any
//   - toolCalls: toolCalls is the list of tool calls requested by the LLM
//   - usage: usage is the token consumption for this single turn
var Turn = define.Function{ // MARKER: Turn
	Host: Hostname, Method: "POST", Route: ":444/turn",
	In: TurnIn{}, Out: TurnOut{},
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Model    string       `json:"model,omitzero"`
	Messages []Message    `json:"messages,omitzero"`
	Tools    []Tool       `json:"tools,omitzero"`
	Options  *TurnOptions `json:"options,omitzero"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	Content    string     `json:"content,omitzero"`
	ToolCalls  []ToolCall `json:"toolCalls,omitzero"`
	StopReason string     `json:"stopReason,omitzero"`
	Usage      Usage      `json:"usage,omitzero"`
}

// InitChat validates inputs, resolves tool schemas from OpenAPI, and stores them in flow state.
var InitChat = define.Task{ // MARKER: InitChat
	Host: Hostname, Method: "POST", Route: ":428/init-chat",
	In: InitChatIn{}, Out: InitChatOut{},
}

// InitChatIn are the input arguments of InitChat.
type InitChatIn struct { // MARKER: InitChat
	Messages []Message    `json:"messages,omitzero"`
	ToolURLs []string     `json:"toolURLs,omitzero"`
	Options  *ChatOptions `json:"options,omitzero"`
}

// InitChatOut are the output arguments of InitChat.
type InitChatOut struct { // MARKER: InitChat
	MaxToolRounds int `json:"maxToolRounds,omitzero"`
	ToolRounds    int `json:"toolRounds,omitzero"`
}

// CallLLM sends the current messages and tools to the LLM provider.
var CallLLM = define.Task{ // MARKER: CallLLM
	Host: Hostname, Method: "POST", Route: ":428/call-llm",
	In: CallLLMIn{}, Out: CallLLMOut{},
}

// CallLLMIn are the input arguments of CallLLM.
type CallLLMIn struct { // MARKER: CallLLM
	Provider string    `json:"provider,omitzero"`
	Model    string    `json:"model,omitzero"`
	Messages []Message `json:"messages,omitzero"`
}

// CallLLMOut are the output arguments of CallLLM.
type CallLLMOut struct { // MARKER: CallLLM
	LLMContent       string `json:"llmContent,omitzero"`
	PendingToolCalls any    `json:"pendingToolCalls,omitzero"`
	TurnUsage        Usage  `json:"turnUsage,omitzero"`
}

// ProcessResponse inspects the LLM response, accumulates usage, and routes to the next step.
var ProcessResponse = define.Task{ // MARKER: ProcessResponse
	Host: Hostname, Method: "POST", Route: ":428/process-response",
	In: ProcessResponseIn{}, Out: ProcessResponseOut{},
}

// ProcessResponseIn are the input arguments of ProcessResponse.
type ProcessResponseIn struct { // MARKER: ProcessResponse
	LLMContent    string `json:"llmContent,omitzero"`
	TurnUsage     Usage  `json:"turnUsage,omitzero"`
	ToolRounds    int    `json:"toolRounds,omitzero"`
	MaxToolRounds int    `json:"maxToolRounds,omitzero"`
}

// ProcessResponseOut are the output arguments of ProcessResponse.
// The running conversation lives entirely in the `messages` state key (Append-reduced across the
// per-tool-call cohort), which ChatLoop's terminal output reads at flow completion. There is no
// per-task "messagesOut" accumulator to return: ProcessResponse and ExecuteTool contribute deltas
// to `messages` via flow.Set, the reducer assembles them, done.
type ProcessResponseOut struct { // MARKER: ProcessResponse
	ToolsRequested bool  `json:"toolsRequested,omitzero"`
	ToolRoundsOut  int   `json:"toolRounds,omitzero"`
	UsageOut       Usage `json:"usage,omitzero"`
}

// ExecuteTool executes a single tool call, identified by the currentTool forEach variable.
var ExecuteTool = define.Task{ // MARKER: ExecuteTool
	Host: Hostname, Method: "POST", Route: ":428/execute-tool",
	In: ExecuteToolIn{}, Out: ExecuteToolOut{},
}

// ExecuteToolIn are the input arguments of ExecuteTool.
type ExecuteToolIn struct { // MARKER: ExecuteTool
}

// ExecuteToolOut are the output arguments of ExecuteTool.
type ExecuteToolOut struct { // MARKER: ExecuteTool
}

// ChatLoop defines the workflow graph for multi-turn LLM conversations with tool calling.
var ChatLoop = define.Workflow{ // MARKER: ChatLoop
	Host: Hostname, Method: "GET", Route: ":428/chat-loop",
	In: ChatLoopIn{}, Out: ChatLoopOut{},
}

// ChatLoopIn are the input arguments of ChatLoop.
type ChatLoopIn struct { // MARKER: ChatLoop
	Provider string       `json:"provider,omitzero"`
	Model    string       `json:"model,omitzero"`
	Messages []Message    `json:"messages,omitzero"`
	ToolURLs []string     `json:"toolURLs,omitzero"`
	Options  *ChatOptions `json:"options,omitzero"`
}

// ChatLoopOut are the output arguments of ChatLoop.
type ChatLoopOut struct { // MARKER: ChatLoop
	MessagesOut []Message `json:"messages,omitzero"`
	Usage       Usage     `json:"usage,omitzero"`
}
