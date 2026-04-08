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

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/coreservices/llm/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ llmapi.Client
	_ *workflow.Flow
)

const (
	Hostname = llmapi.Hostname
	Version  = 3
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Chat(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error)                                                                               // MARKER: Chat
	Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error)                                                                       // MARKER: Turn
	InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (maxToolRounds int, toolRounds int, err error)                                                 // MARKER: InitChat
	CallLLM(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message) (llmContent string, pendingToolCalls any, err error)                                                                 // MARKER: CallLLM
	ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, err error) // MARKER: ProcessResponse
	ExecuteTool(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error)                                                                                        // MARKER: ExecuteTool
	ChatLoop(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                  // MARKER: ChatLoop
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(llmapi.Chat.Method, llmapi.Chat.Route, svc.doChat) // MARKER: Chat
	svc.Subscribe(llmapi.Turn.Method, llmapi.Turn.Route, svc.doTurn) // MARKER: Turn

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: ProviderHostname
		"ProviderHostname",
		cfg.Description(`ProviderHostname is the hostname of the LLM provider microservice that implements the Turn endpoint.`),
		cfg.DefaultValue("claude.llm.core"),
	)
	svc.DefineConfig( // MARKER: MaxToolRounds
		"MaxToolRounds",
		cfg.Description(`MaxToolRounds is the maximum number of tool call round-trips per invocation.`),
		cfg.DefaultValue("10"),
		cfg.Validation("int [1,50]"),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here
	svc.Subscribe(llmapi.InitChat.Method, llmapi.InitChat.Route, svc.doInitChat)                      // MARKER: InitChat
	svc.Subscribe(llmapi.CallLLM.Method, llmapi.CallLLM.Route, svc.doCallLLM)                         // MARKER: CallLLM
	svc.Subscribe(llmapi.ProcessResponse.Method, llmapi.ProcessResponse.Route, svc.doProcessResponse) // MARKER: ProcessResponse
	svc.Subscribe(llmapi.ExecuteTool.Method, llmapi.ExecuteTool.Route, svc.doExecuteTool)             // MARKER: ExecuteTool

	// HINT: Add graph endpoints here
	svc.Subscribe(llmapi.ChatLoop.Method, llmapi.ChatLoop.Route, svc.doChatLoop) // MARKER: ChatLoop

	_ = marshalFunction
	return svc
}

// doChat handles marshaling for Chat.
func (svc *Intermediate) doChat(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Chat
	var in llmapi.ChatIn
	var out llmapi.ChatOut
	err = marshalFunction(w, r, llmapi.Chat.Route, &in, &out, func(_ any, _ any) error {
		out.MessagesOut, err = svc.Chat(r.Context(), in.Messages, in.Tools)
		return err // No trace
	})
	return err // No trace
}

// doTurn handles marshaling for Turn.
func (svc *Intermediate) doTurn(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Turn
	var in llmapi.TurnIn
	var out llmapi.TurnOut
	err = marshalFunction(w, r, llmapi.Turn.Route, &in, &out, func(_ any, _ any) error {
		out.Completion, err = svc.Turn(r.Context(), in.Messages, in.Tools)
		return err // No trace
	})
	return err // No trace
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
		{ // MARKER: Chat
			Type:    "function",
			Name:    "Chat",
			Method:  llmapi.Chat.Method,
			Route:   llmapi.Chat.Route,
			Summary: "Chat(messages []Message, tools []Tool) (messagesOut []Message)",
			Description: `Chat sends messages to an LLM with optional tools and returns the full conversation including new messages.

Input:
  - messages: messages is the conversation history to send to the LLM
  - tools: tools is a list of Microbus endpoint URLs to expose as LLM tools

Output:
  - messagesOut: messagesOut is the full conversation including new messages produced by the LLM`,
			InputArgs:  llmapi.ChatIn{},
			OutputArgs: llmapi.ChatOut{},
		},
		{ // MARKER: Turn
			Type:    "function",
			Name:    "Turn",
			Method:  llmapi.Turn.Method,
			Route:   llmapi.Turn.Route,
			Summary: "Turn(messages []Message, tools []ToolDef) (completion *TurnCompletion)",
			Description: `Turn executes a single LLM turn: sends messages and tool definitions to the LLM provider and returns the completion.

Input:
  - messages: messages is the conversation history to send to the LLM
  - tools: tools is the resolved tool definitions with schemas

Output:
  - completion: completion is the LLM's response including text and tool calls`,
			InputArgs:  llmapi.TurnIn{},
			OutputArgs: llmapi.TurnOut{},
		},
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
	// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	return nil
}

// doInitChat handles marshaling for InitChat.
func (svc *Intermediate) doInitChat(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: InitChat
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in llmapi.InitChatIn
	flow.ParseState(&in)
	var out llmapi.InitChatOut
	out.MaxToolRounds, out.ToolRounds, err = svc.InitChat(r.Context(), &flow, in.Messages, in.Tools)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	return errors.Trace(err)
}

// doCallLLM handles marshaling for CallLLM.
func (svc *Intermediate) doCallLLM(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CallLLM
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in llmapi.CallLLMIn
	flow.ParseState(&in)
	var out llmapi.CallLLMOut
	out.LLMContent, out.PendingToolCalls, err = svc.CallLLM(r.Context(), &flow, in.Messages)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	return errors.Trace(err)
}

// doProcessResponse handles marshaling for ProcessResponse.
func (svc *Intermediate) doProcessResponse(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ProcessResponse
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in llmapi.ProcessResponseIn
	flow.ParseState(&in)
	var out llmapi.ProcessResponseOut
	out.MessagesOut, out.ToolsRequested, out.ToolRoundsOut, err = svc.ProcessResponse(r.Context(), &flow, in.LLMContent, in.ToolRounds, in.MaxToolRounds)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	return errors.Trace(err)
}

// doExecuteTool handles marshaling for ExecuteTool.
func (svc *Intermediate) doExecuteTool(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ExecuteTool
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in llmapi.ExecuteToolIn
	flow.ParseState(&in)
	var out llmapi.ExecuteToolOut
	out.ToolExecutedOut, err = svc.ExecuteTool(r.Context(), &flow, in.ToolExecuted)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	return errors.Trace(err)
}

// doChatLoop handles marshaling for the ChatLoop workflow graph.
func (svc *Intermediate) doChatLoop(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ChatLoop
	graph, err := svc.ChatLoop(r.Context())
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(graph)
	return errors.Trace(err)
}

/*
ProviderHostname is the hostname of the LLM provider microservice that implements the Turn endpoint.
*/
func (svc *Intermediate) ProviderHostname() (value string) { // MARKER: ProviderHostname
	return svc.Config("ProviderHostname")
}

/*
SetProviderHostname sets the value of the configuration property.
*/
func (svc *Intermediate) SetProviderHostname(value string) (err error) { // MARKER: ProviderHostname
	return svc.SetConfig("ProviderHostname", value)
}

/*
MaxToolRounds is the maximum number of tool call round-trips per invocation.
*/
func (svc *Intermediate) MaxToolRounds() (value int) { // MARKER: MaxToolRounds
	_val := svc.Config("MaxToolRounds")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetMaxToolRounds sets the value of the configuration property.
*/
func (svc *Intermediate) SetMaxToolRounds(value int) (err error) { // MARKER: MaxToolRounds
	return svc.SetConfig("MaxToolRounds", strconv.Itoa(value))
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
