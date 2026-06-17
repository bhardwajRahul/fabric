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

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ llmapi.Client
)

/*
Service implements the llm.core microservice.

The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// logPrompt logs the last message being sent to the LLM.
func (svc *Service) logPrompt(ctx context.Context, messages []llmapi.Message) {
	if len(messages) > 0 {
		prompt := messages[len(messages)-1].Content
		if len(prompt) > 1024 {
			prompt = prompt[:1024] + "..."
		}
		svc.LogInfo(ctx, "Asking LLM", "prompt", prompt)
	}
}

// logCompletion logs the LLM's response and emits the LLMTokens metric.
func (svc *Service) logCompletion(ctx context.Context, provider string, content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage) {
	reply := content
	if len(reply) > 1024 {
		reply = reply[:1024] + "..."
	}
	toolNames := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		toolNames[i] = tc.Name
	}
	svc.LogInfo(ctx, "LLM answered",
		"reply", reply,
		"toolCalls", toolNames,
		"inputTokens", usage.InputTokens,
		"outputTokens", usage.OutputTokens,
	)

	model := usage.Model
	if usage.InputTokens > 0 {
		svc.IncrementLLMTokens(ctx, usage.InputTokens, provider, model, "input")
	}
	if usage.OutputTokens > 0 {
		svc.IncrementLLMTokens(ctx, usage.OutputTokens, provider, model, "output")
	}
	if usage.CacheReadTokens > 0 {
		svc.IncrementLLMTokens(ctx, usage.CacheReadTokens, provider, model, "cacheRead")
	}
	if usage.CacheWriteTokens > 0 {
		svc.IncrementLLMTokens(ctx, usage.CacheWriteTokens, provider, model, "cacheWrite")
	}
}

// turn calls the provider's Turn endpoint over the bus.
func (svc *Service) turn(ctx context.Context, providerHost string, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) {
	content, toolCalls, stopReason, usage, err = llmapi.NewClient(svc).ForHost(providerHost).Turn(ctx, model, messages, tools, options)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	return content, toolCalls, stopReason, usage, nil
}

// stopReasonError returns nil when the turn's stop reason is a normal completion
// (end_turn / stop_sequence / refusal / tool_use), and an error otherwise. Truncation
// (max_tokens), provider extensions (pause_turn), and unknown values fall through to
// the error path so silent partial responses cannot flow downstream.
func stopReasonError(stopReason, provider, model string) error {
	switch stopReason {
	case llmapi.StopReasonEndTurn, llmapi.StopReasonStopSequence, llmapi.StopReasonRefusal, llmapi.StopReasonToolUse:
		return nil
	case llmapi.StopReasonMaxTokens:
		return errors.New("LLM response truncated at max_tokens",
			"provider", provider, "model", model, http.StatusInsufficientStorage)
	case llmapi.StopReasonPauseTurn:
		return errors.New("LLM returned pause_turn (provider extension not handled)",
			"provider", provider, "model", model, http.StatusBadGateway)
	default:
		return errors.New("LLM returned unknown stop reason",
			"provider", provider, "model", model, "stopReason", stopReason, http.StatusBadGateway)
	}
}

/*
Turn on llm.core is a stub. The interface contract is defined here so providers can implement it,
but llm.core does not route or delegate. Callers should ForHost(<providerHostname>) directly to
hit a specific provider, or use Chat for the full conversation loop.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	return "", nil, "", llmapi.Usage{}, errors.New("not implemented on llm.core; call ForHost(<provider>).Turn or use Chat", http.StatusNotImplemented)
}

/*
Chat sends messages to an LLM with optional tools and returns the response messages.

The provider hostname picks which provider microservice handles the request (e.g.
"claude.llm.core"). The model identifier is provider-specific.

Each entry in toolURLs is the canonical URL of a Microbus endpoint to expose to the LLM.
Chat groups the URLs by host:port, fetches the OpenAPI document of each host, and resolves
each URL to a callable tool. Only FeatureFunction, FeatureWeb, and FeatureWorkflow endpoints
are exposed.

Input:
  - provider: provider is the hostname of the LLM provider microservice to use
  - model: model is the provider-specific model identifier
  - messages: messages is the conversation history to send to the LLM
  - toolURLs: toolURLs is the list of Microbus endpoint URLs exposed to the LLM
  - options: options configures tool-call rounds, max tokens, temperature (nil = defaults)

Output:
  - messagesOut: messagesOut is the full conversation including new messages produced by the LLM
  - usage: usage is the aggregate token consumption across all turns
*/
func (svc *Service) Chat(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) { // MARKER: Chat
	if provider == "" {
		return nil, llmapi.Usage{}, errors.New("provider is required", http.StatusBadRequest)
	}
	if model == "" {
		return nil, llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}

	tools, err := svc.fetchTools(ctx, toolURLs)
	if err != nil {
		return nil, llmapi.Usage{}, errors.Trace(err)
	}

	maxRounds := svc.MaxToolRounds()
	var turnOpts *llmapi.TurnOptions
	if options != nil {
		if options.MaxToolRounds > 0 {
			maxRounds = options.MaxToolRounds
		}
		if options.MaxTokens > 0 || options.Temperature != 0 {
			turnOpts = &llmapi.TurnOptions{
				MaxTokens:   options.MaxTokens,
				Temperature: options.Temperature,
			}
		}
	}

	// Conversation with the LLM
	currentMessages := make([]llmapi.Message, len(messages))
	copy(currentMessages, messages)

	for range maxRounds {
		// Call the LLM
		svc.logPrompt(ctx, currentMessages)
		content, toolCalls, stopReason, turnUsage, err := svc.turn(ctx, provider, model, currentMessages, tools, turnOpts)
		if err != nil {
			return nil, usage, errors.Trace(err)
		}
		svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)
		usage.Add(turnUsage)

		// Surface non-completion stop reasons so a truncated or paused turn never flows
		// downstream as if it were end_turn.
		err = stopReasonError(stopReason, provider, model)
		if err != nil {
			return nil, usage, errors.Trace(err)
		}

		// end_turn / stop_sequence / refusal: the model is done. Emit the assistant content
		// and exit. tool_use without any tool calls is a provider bug; treat as done too.
		if stopReason != llmapi.StopReasonToolUse || len(toolCalls) == 0 {
			if content != "" {
				messagesOut = append(messagesOut, llmapi.Message{
					Role:    "assistant",
					Content: content,
				})
			}
			return messagesOut, usage, nil
		}

		// Record the assistant's response with tool calls metadata
		toolCallsJSON, _ := json.Marshal(toolCalls)
		assistantMsg := llmapi.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: string(toolCallsJSON),
		}
		messagesOut = append(messagesOut, assistantMsg)
		currentMessages = append(currentMessages, assistantMsg)

		// Execute each tool call
		for _, tc := range toolCalls {
			result, err := svc.executeTool(ctx, tc, tools)
			if err != nil {
				return nil, usage, errors.Trace(err)
			}

			toolResultMsg := llmapi.Message{
				Role:       "tool",
				Content:    string(result),
				ToolCallID: tc.ID,
			}
			messagesOut = append(messagesOut, toolResultMsg)
			currentMessages = append(currentMessages, toolResultMsg)
		}
	}

	// Exhausted tool rounds - make one final call without tools to get a text response
	svc.logPrompt(ctx, currentMessages)
	content, toolCalls, stopReason, turnUsage, err := svc.turn(ctx, provider, model, currentMessages, nil, turnOpts)
	if err != nil {
		return nil, usage, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)
	usage.Add(turnUsage)
	err = stopReasonError(stopReason, provider, model)
	if err != nil {
		return nil, usage, errors.Trace(err)
	}
	if content != "" {
		messagesOut = append(messagesOut, llmapi.Message{
			Role:    "assistant",
			Content: content,
		})
	}
	return messagesOut, usage, nil
}

/*
InitChat resolves caller-supplied tool URLs into LLM tool schemas via each host's OpenAPI document
and stores them, along with chat options, in flow state for use by the rest of the chat loop.

Input:
  - messages: messages is the initial conversation history sent to the LLM
  - toolURLs: toolURLs is the list of Microbus endpoint URLs exposed to the LLM
  - options: options configures tool-call rounds, max tokens, temperature (nil = defaults)

Output:
  - maxToolRounds: maxToolRounds is the resolved per-conversation tool-round ceiling
  - toolRounds: toolRounds is the starting round counter (always zero)
*/
func (svc *Service) InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error) { // MARKER: InitChat
	tools, err := svc.fetchTools(ctx, toolURLs)
	if err != nil {
		return 0, 0, errors.Trace(err)
	}
	if len(tools) > 0 {
		flow.Set("toolSchemas", tools)
	}
	maxToolRounds = svc.MaxToolRounds()
	if options != nil && options.MaxToolRounds > 0 {
		maxToolRounds = options.MaxToolRounds
	}
	if options != nil && (options.MaxTokens > 0 || options.Temperature != 0) {
		flow.Set("turnOptions", &llmapi.TurnOptions{
			MaxTokens:   options.MaxTokens,
			Temperature: options.Temperature,
		})
	}
	toolRounds = 0
	return maxToolRounds, toolRounds, nil
}

/*
CallLLM sends the current messages and tools to the LLM provider.
*/
func (svc *Service) CallLLM(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message) (llmContent string, pendingToolCalls any, turnUsage llmapi.Usage, err error) { // MARKER: CallLLM
	// Read tool schemas
	var tools []llmapi.Tool
	flow.Get("toolSchemas", &tools)

	// Read turn options
	var turnOpts *llmapi.TurnOptions
	flow.Get("turnOptions", &turnOpts)

	// Call the LLM
	svc.logPrompt(ctx, messages)
	content, toolCalls, stopReason, turnUsage, err := svc.turn(ctx, provider, model, messages, tools, turnOpts)
	if err != nil {
		return "", nil, llmapi.Usage{}, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)

	// Truncation, pause_turn, or unknown stop reasons fail the step so the workflow's
	// OnError route (if any) handles it instead of feeding a partial response into the
	// next graph node.
	err = stopReasonError(stopReason, provider, model)
	if err != nil {
		return "", nil, llmapi.Usage{}, errors.Trace(err)
	}

	return content, toolCalls, turnUsage, nil
}

/*
ProcessResponse inspects the LLM response, accumulates usage, and routes to the next step.
When the conversation is complete (no tool calls or round limit exhausted), it calls
flow.Goto(workflow.END) to exit the chat loop. Otherwise the forEach transition fans out
one ExecuteTool per pending tool call.
*/
func (svc *Service) ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) { // MARKER: ProcessResponse
	// Read internal state
	var pending []llmapi.ToolCall
	flow.Get("pendingToolCalls", &pending)
	{
		snap := flow.Snapshot()
		keys := make([]string, 0, len(snap))
		for k := range snap {
			keys = append(keys, k)
		}
		var teVal any
		flow.Get("toolExecuted", &teVal)
		svc.LogDebug(ctx, "ProcessResponse entry",
			"toolRounds", toolRounds,
			"pendingCount", len(pending),
			"stateKeys", keys,
			"toolExecutedInState", teVal,
		)
	}

	flow.Get("usage", &usageOut)
	usageOut.Add(turnUsage)

	toolRoundsOut = toolRounds

	// CRITICAL: messagesOut is declared in the return signature with JSON tag "messages" (see
	// ProcessResponseOut). The auto-generated marshaler runs flow.SetChanges(out, snap) after
	// this function returns, and a non-nil messagesOut there would write the FULL accumulated
	// list to the "messages" state key -- overwriting the delta-only flow.Set("messages", ...)
	// calls below and producing Append-reducer duplication on the next turn. We return nil for
	// messagesOut and let the running "messages" state key (driven by ReducerAppend) carry the
	// full transcript; the workflow's ChatLoopOut.MessagesOut (json:"messages") reads from that
	// same state key when the flow terminates.
	done := len(pending) == 0 || toolRounds >= maxToolRounds
	if done {
		toolsRequested = false
		if llmContent != "" {
			flow.Set("messages", []llmapi.Message{{
				Role:    "assistant",
				Content: llmContent,
			}})
		}
		// Strip in-loop scratch so the flow's final_state contains only the
		// declared ChatLoopOut surface (`messages`, `usage`). This matters when
		// ChatLoop runs as a subgraph or feeds Continue: state crosses the
		// boundary unfiltered, and these fields would otherwise pollute the
		// parent's state. toolRounds and toolsRequested are in ProcessResponseOut
		// and re-written by the auto-marshaler regardless, so they survive — the
		// long-term fix is a scratch-naming convention the framework strips at
		// subgraph boundaries.
		flow.Delete(
			"toolSchemas",
			"turnOptions",
			"pendingToolCalls",
			"maxToolRounds",
			"llmContent",
			"turnUsage",
		)
		flow.Goto(workflow.END)
		return toolsRequested, toolRoundsOut, usageOut, nil
	}

	// Tool calls present - record assistant message and set up forEach.
	// The assistant message MUST carry the tool calls so that, on the next turn, claudellm can
	// emit a tool_use block whose id matches the tool_use_id on the upcoming tool_result.
	// Without this, Anthropic rejects the next call with
	//   "unexpected tool_use_id found in tool_result blocks: ...".
	//
	// `messages` has reducer ReducerAppend on the parent graph. The spawn-step's changes are
	// merged through that reducer when the forEach cohort is created, so writing the full
	// accumulated list here would concatenate the entire history onto itself. We write only
	// the delta -- the single assistant message that this turn produced -- and let the reducer
	// append it onto the prior snapshot.
	toolsRequested = true
	toolRoundsOut = toolRounds + 1
	toolCallsJSON, _ := json.Marshal(pending)
	assistantMsg := llmapi.Message{
		Role:      "assistant",
		Content:   llmContent,
		ToolCalls: string(toolCallsJSON),
	}
	flow.Set("messages", []llmapi.Message{assistantMsg})

	return toolsRequested, toolRoundsOut, usageOut, nil
}

/*
ExecuteTool executes a single tool call identified by the currentTool forEach variable. Workflow tools run as
dynamic subgraphs via flow.Subgraph, which parks the step and returns the child's result on re-entry; regular
tools run via a direct bus call.
*/
func (svc *Service) ExecuteTool(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: ExecuteTool
	var currentTool llmapi.ToolCall
	flow.Get("currentTool", &currentTool)
	var tools []llmapi.Tool
	flow.Get("toolSchemas", &tools)

	// Find the tool definition
	var def llmapi.Tool
	for _, t := range tools {
		if t.Name == currentTool.Name {
			def = t
			break
		}
	}
	if def.URL == "" {
		return errors.New("tool not found: %s", currentTool.Name)
	}

	// Workflow tools are executed as dynamic subgraphs. flow.Subgraph parks the step on the first
	// call (yield) and returns the child's final_state on re-entry.
	if def.Type == "workflow" {
		inputState := make(map[string]any)
		if currentTool.Arguments != nil {
			json.Unmarshal(currentTool.Arguments, &inputState)
		}
		var out map[string]any
		yield, err := flow.Subgraph(def.URL, inputState, &out)
		if err != nil {
			return errors.Trace(err)
		}
		if yield {
			return nil // parked, child workflow running
		}
		// Re-entry: the child's final_state is the tool result. `messages` has reducer ReducerAppend,
		// so contribute only the new tool_result delta.
		childOutput := out
		if len(childOutput) == 0 {
			childOutput = map[string]any{"status": "completed"}
		}
		resultJSON, _ := json.Marshal(childOutput)
		flow.Set("messages", []llmapi.Message{{
			Role:       "tool",
			ToolCallID: currentTool.ID,
			Content:    string(resultJSON),
		}})
		return nil
	}

	// Regular endpoint tools are executed via direct bus call
	result, err := svc.executeTool(ctx, currentTool, tools)
	if err != nil {
		return errors.Trace(err)
	}

	// `messages` has reducer ReducerAppend. Branch contributes only the new tool_result delta.
	flow.Set("messages", []llmapi.Message{{
		Role:       "tool",
		ToolCallID: currentTool.ID,
		Content:    string(result),
	}})
	return nil
}

/*
ChatLoop defines the workflow graph for the LLM chat loop.
*/
func (svc *Service) ChatLoop(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: ChatLoop
	graph = workflow.NewGraph("ChatLoop")
	graph.SetEndpoint("InitChat", llmapi.InitChat.URL())
	// FirstLLM and NextLLM are two graph positions sharing one task URL. FirstLLM is the
	// initial sequential call after InitChat; NextLLM is the fan-in nexus for the per-round
	// tool cohort. Both dispatch to the same CallLLM task. Splitting them lets the lineage
	// validator pop the cohort frame at NextLLM without conflicting with the initial entry,
	// which has no frame to pop.
	graph.SetEndpoint("FirstLLM", llmapi.CallLLM.URL())
	graph.SetEndpoint("NextLLM", llmapi.CallLLM.URL())
	graph.SetEndpoint("ProcessResponse", llmapi.ProcessResponse.URL())
	graph.SetEndpoint("ExecuteTool", llmapi.ExecuteTool.URL())
	graph.SetFanIn("NextLLM")
	// messages is the conversation history shared with the LLM each turn. forEach branches each
	// contribute their tool result message; Append reducer concatenates them at the fan-in so the
	// next CallLLM sees the full history.
	graph.SetReducer("messages", workflow.ReducerAppend)
	graph.AddTransition("InitChat", "FirstLLM")
	graph.AddTransition("FirstLLM", "ProcessResponse")
	// When ProcessResponse decides the conversation is done (no tools requested or round
	// limit exceeded), it calls flow.Goto(workflow.END) to exit the loop.
	graph.AddTransitionGoto("ProcessResponse", workflow.END)
	// Otherwise the forEach fans out one ExecuteTool per pending tool call; all branches
	// converge at NextLLM via the fan-in.
	graph.AddTransitionForEach("ProcessResponse", "ExecuteTool", "pendingToolCalls", "currentTool")
	graph.AddTransition("ExecuteTool", "NextLLM")
	graph.AddTransition("NextLLM", "ProcessResponse")
	return graph, nil
}
