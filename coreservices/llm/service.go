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
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/frame"
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

// logPrompt logs the tail of the conversation being sent to the LLM, at Debug level. The tail is the
// new input for this turn: a user/system message on the first turn, or the tool_result items on a
// tool-calling round - so it is labeled by what it actually is rather than always "prompt". Content can
// carry sensitive data and every turn hits this path, so it stays out of the default (INFO) stream;
// enable with MICROBUS_LOG_DEBUG=1 when diagnosing. Logged in full (no truncation).
func (svc *Service) logPrompt(ctx context.Context, items []llmapi.Item) {
	for i := len(items) - 1; i >= 0; i-- {
		switch items[i].Type() {
		case llmapi.ItemMessage:
			svc.LogDebug(ctx, "Calling LLM", "lastMessage", items[i].Message.Content)
			return
		case llmapi.ItemToolResult:
			svc.LogDebug(ctx, "Calling LLM", "lastToolResult", items[i].ToolResult.Output)
			return
		}
	}
}

// logCompletion emits the LLMTokens metric and logs the LLM's response turn at Debug level. The reply
// can carry sensitive content and every turn hits this path, so the content log is Debug-only; the
// prod-observable signal is the LLMTokens metric below, not a per-call INFO line.
func (svc *Service) logCompletion(ctx context.Context, provider string, turnItems []llmapi.Item, usage llmapi.Usage) {
	toolCalls := llmapi.PendingToolCalls(turnItems)
	toolNames := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		toolNames[i] = tc.Name
	}
	svc.LogDebug(ctx, "LLM answered",
		"reply", llmapi.LastAssistantMessage(turnItems),
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
func (svc *Service) turn(ctx context.Context, providerHost string, model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (turnItems []llmapi.Item, stopReason string, usage llmapi.Usage, err error) {
	turnItems, stopReason, usage, err = llmapi.NewClient(svc).ForHost(providerHost).Turn(ctx, model, items, tools, options)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	return turnItems, stopReason, usage, nil
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
			http.StatusInsufficientStorage,
			"provider", provider,
			"model", model,
		)
	case llmapi.StopReasonPauseTurn:
		return errors.New("LLM returned pause_turn (provider extension not handled)",
			http.StatusBadGateway,
			"provider", provider,
			"model", model,
		)
	default:
		return errors.New("LLM returned unknown stop reason",
			http.StatusBadGateway,
			"provider", provider,
			"model", model,
			"stopReason", stopReason,
		)
	}
}

/*
Turn executes a single LLM turn. On llm.core it is a stub returning 501 Not Implemented; the actual
implementation lives in each provider microservice (claudellm, chatgptllm, geminillm). Call
ForHost(<providerHostname>).Turn to reach a specific provider directly, or use Chat for the full
conversation loop.
*/
func (svc *Service) Turn(ctx context.Context, model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (itemsOut []llmapi.Item, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	return nil, "", llmapi.Usage{}, errors.New("stub, not implemented on llm.core", http.StatusNotImplemented)
}

/*
Chat sends a conversation to an LLM with optional tools, looping through tool calls until the LLM
returns a final answer. The provider hostname selects the provider microservice (e.g. claude.llm.core)
and the model is provider-specific. Each toolURL is the canonical URL of a Microbus Function, Web, or
Workflow endpoint exposed to the LLM; Chat fetches each host's OpenAPI document and resolves the URL to
a callable tool. On error it still returns the items accumulated before the failure, so a caller running
its own retry can resume from them (e.g. wait llmapi.RetryAfter(err) and re-call with the returned items)
instead of restarting the conversation.
*/
func (svc *Service) Chat(ctx context.Context, provider string, model string, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (itemsOut []llmapi.Item, usage llmapi.Usage, err error) { // MARKER: Chat
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
	currentItems := make([]llmapi.Item, len(items))
	copy(currentItems, items)

	// itemsOut is the full conversation returned to the caller: the input items followed by every
	// item the LLM produces. This matches ChatOut's documented contract and the resume pattern (a
	// caller re-invoking with the returned items resumes the whole conversation, not just the tail).
	itemsOut = append(itemsOut, items...)

	for range maxRounds {
		// Call the LLM
		svc.logPrompt(ctx, currentItems)
		turnItems, stopReason, turnUsage, err := svc.turn(ctx, provider, model, currentItems, tools, turnOpts)
		if err != nil {
			return itemsOut, usage, errors.Trace(err)
		}
		svc.logCompletion(ctx, provider, turnItems, turnUsage)
		usage.Add(turnUsage)

		// Surface non-completion stop reasons so a truncated or paused turn never flows
		// downstream as if it were end_turn.
		err = stopReasonError(stopReason, provider, model)
		if err != nil {
			return itemsOut, usage, errors.Trace(err)
		}

		// end_turn / stop_sequence / refusal: the model is done. Emit the assistant turn
		// and exit. tool_use without any tool calls is a provider bug; treat as done too.
		toolCalls := llmapi.PendingToolCalls(turnItems)
		if stopReason != llmapi.StopReasonToolUse || len(toolCalls) == 0 {
			itemsOut = append(itemsOut, turnItems...)
			return itemsOut, usage, nil
		}

		// Record the assistant turn (reasoning, text, and tool_call items) verbatim.
		itemsOut = append(itemsOut, turnItems...)
		currentItems = append(currentItems, turnItems...)

		// Execute each tool call and append its result as a tool_result item.
		for _, tc := range toolCalls {
			result, err := svc.executeTool(ctx, tc, tools)
			if err != nil {
				return itemsOut, usage, errors.Trace(err)
			}
			resultItem := llmapi.NewToolResult(tc.ID, string(result))
			itemsOut = llmapi.AppendItems(itemsOut, resultItem)
			currentItems = llmapi.AppendItems(currentItems, resultItem)
		}
	}

	// Exhausted tool rounds - make one final call without tools to get a text response
	svc.logPrompt(ctx, currentItems)
	turnItems, stopReason, turnUsage, err := svc.turn(ctx, provider, model, currentItems, nil, turnOpts)
	if err != nil {
		return itemsOut, usage, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, turnItems, turnUsage)
	usage.Add(turnUsage)
	err = stopReasonError(stopReason, provider, model)
	if err != nil {
		return itemsOut, usage, errors.Trace(err)
	}
	itemsOut = append(itemsOut, turnItems...)
	return itemsOut, usage, nil
}

/*
InitChat resolves caller-supplied tool URLs into LLM tool schemas via each host's OpenAPI document
and stores them, along with chat options, in flow state for use by the rest of the chat loop.
*/
func (svc *Service) InitChat(ctx context.Context, flow *workflow.Flow, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (err error) { // MARKER: InitChat
	// InitChat is a pure setup step: it seeds the ambient flow state the rest of the loop reads
	// (toolSchemas, turnOptions, maxToolRounds) and the tool-round counter (toolRounds), and declares
	// no output arguments.
	tools, err := svc.fetchTools(ctx, toolURLs)
	if err != nil {
		return errors.Trace(err)
	}
	if len(tools) > 0 {
		flow.Set("toolSchemas", tools)
	}
	maxToolRounds := svc.MaxToolRounds()
	if options != nil && options.MaxToolRounds > 0 {
		maxToolRounds = options.MaxToolRounds
	}
	flow.Set("maxToolRounds", maxToolRounds)
	if options != nil && (options.MaxTokens > 0 || options.Temperature != 0) {
		flow.Set("turnOptions", &llmapi.TurnOptions{
			MaxTokens:   options.MaxTokens,
			Temperature: options.Temperature,
		})
	}
	flow.Set("toolRounds", 0)
	return nil
}

/*
CallLLM is the sole owner of the `items` conversation state key. It folds the prior round's tool
results into the conversation, calls the provider, and writes the full conversation back to `items`.
*/
func (svc *Service) CallLLM(ctx context.Context, flow *workflow.Flow, provider string, model string, items []llmapi.Item, toolResults []llmapi.ToolResult) (itemsOut []llmapi.Item, pendingToolCalls []llmapi.ToolCall, turnUsage llmapi.Usage, err error) { // MARKER: CallLLM
	// Fold the prior round's tool results (appended into toolResults by the fan-in reducer) into a local
	// conversation copy. On the first call (FirstLLM) there are none.
	convo := items
	for _, tr := range toolResults {
		convo = llmapi.AppendItems(convo, tr)
	}

	// Read tool schemas
	var tools []llmapi.Tool
	flow.Get("toolSchemas", &tools)

	// Read turn options
	var turnOpts *llmapi.TurnOptions
	flow.Get("turnOptions", &turnOpts)

	// finalCall is set by ProcessResponse once the tool-round limit is reached. On the final call we
	// offer no tools, forcing the model to produce a text answer instead of another (unexecutable)
	// round of tool calls. Mirrors the live Chat loop's post-limit "one final call without tools".
	var finalCall bool
	flow.Get("finalCall", &finalCall)
	if finalCall {
		tools = nil
	}

	// Call the LLM
	svc.logPrompt(ctx, convo)
	turnItems, stopReason, turnUsage, err := svc.turn(ctx, provider, model, convo, tools, turnOpts)
	if err != nil {
		// A rate-limited error carries a retryAfter (a duration string); its presence is the retry signal, not the
		// status code. Re-dispatch this step after exactly that wait. The wait goes in Retry's initialDelay with
		// multiplier 1.0 and no per-interval cap, so every attempt waits exactly retryAfter with no exponential
		// growth on top (rate limits are a known-reset condition). Any error without a retryAfter is permanent and
		// fails the step. The horizon is the task's own time budget (read from the inbound frame): keep retrying
		// only as long as this CallLLM step is worth running, so a misclassified-permanent or poison 429 cannot
		// loop forever. A caller needing longer-than-budget patience owns the retry itself (see Chat, which returns
		// its accumulated items on error so the caller can resume rather than restart).
		if wait, retryable := llmapi.RetryAfter(err); retryable {
			if wait <= 0 {
				wait = time.Minute // provider marked it retryable but sent a malformed or non-positive retryAfter
			}
			if budget := frame.Of(ctx).TimeBudget(); budget > 0 && flow.Retry(wait, 1.0, 0, budget) {
				// Clean rewind: return the pristine input items so SetChanges records no diff on `items`,
				// and leave toolResults untouched so the re-dispatched call re-folds them. Returning nil
				// here would clear `items` (a nil field marshals to null and is persisted with the retry),
				// losing the conversation.
				return items, nil, llmapi.Usage{}, nil
			}
		}
		return nil, nil, llmapi.Usage{}, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, turnItems, turnUsage)

	// Truncation, pause_turn, or unknown stop reasons fail the step so the workflow's
	// OnError route (if any) handles it instead of feeding a partial response into the
	// next graph node.
	err = stopReasonError(stopReason, provider, model)
	if err != nil {
		return nil, nil, llmapi.Usage{}, errors.Trace(err)
	}

	// Commit: the fold is now permanent, so clear the reduced toolResults key (else the next round's
	// fan-in would re-append this round's results and double-fold), and write the full conversation -
	// input + folded results + this turn - back to `items` as a plain replace.
	if len(toolResults) > 0 {
		flow.Delete("toolResults")
	}
	itemsOut = append(convo, turnItems...)
	return itemsOut, llmapi.PendingToolCalls(turnItems), turnUsage, nil
}

/*
ProcessResponse inspects the LLM response, accumulates usage, and routes to the next step.
When the conversation is complete (no tool calls, or the round limit has already produced a final
tool-less answer), it calls flow.Goto(workflow.END) to exit the chat loop. Otherwise the forEach
transition fans out one ExecuteTool per pending tool call.
*/
func (svc *Service) ProcessResponse(ctx context.Context, flow *workflow.Flow, pendingToolCalls []llmapi.ToolCall, turnUsage llmapi.Usage, toolRounds int) (toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) { // MARKER: ProcessResponse
	// maxToolRounds is ambient config seeded once by InitChat.
	var maxToolRounds int
	flow.Get("maxToolRounds", &maxToolRounds)

	flow.Get("usage", &usageOut)
	usageOut.Add(turnUsage)

	toolRoundsOut = toolRounds

	// ProcessResponse does not touch the `items` state key - CallLLM owns it (plain replace, the full
	// conversation each turn). This step only accumulates usage and routes: fan out to execute the
	// pending tool calls, or end the loop. The ExecuteTool branches write the `toolResults` state key
	// (Append-reduced at the fan-in), which the next CallLLM folds into `items`.

	// Done when the model returned no tool calls (natural completion), or the round limit has already
	// been reached. The latter is only hit if the final tool-less call still came back with tool calls -
	// a failsafe against an unbounded loop, not the normal exit.
	done := len(pendingToolCalls) == 0 || toolRounds >= maxToolRounds
	if done {
		toolsRequested = false
		// Strip in-loop scratch so the flow's final_state contains only the
		// declared ChatLoopOut surface (`items`, `usage`). This matters when
		// ChatLoop runs as a subgraph or feeds Continue: state crosses the
		// boundary unfiltered, and these fields would otherwise pollute the
		// parent's state. toolRounds and toolsRequested are in ProcessResponseOut
		// and re-written by the auto-marshaler regardless, so they survive. The
		// long-term fix is a scratch-naming convention the framework strips at
		// subgraph boundaries.
		flow.Delete(
			"toolSchemas",
			"turnOptions",
			"pendingToolCalls",
			"maxToolRounds",
			"turnUsage",
			"toolResults",
			"finalCall",
		)
		flow.Goto(workflow.END)
		return toolsRequested, toolRoundsOut, usageOut, nil
	}

	// Tool calls present and under the limit: fan out to execute them.
	toolsRequested = true
	toolRoundsOut = toolRounds + 1
	// When this is the last permitted tool round, arm finalCall so the following CallLLM omits tools
	// and the model must return a text answer, rather than the loop ending on dangling, unexecuted
	// tool_use items. Mirrors the live Chat loop's post-limit "one final call without tools".
	if toolRoundsOut >= maxToolRounds {
		flow.Set("finalCall", true)
	}

	return toolsRequested, toolRoundsOut, usageOut, nil
}

/*
ExecuteTool executes a single tool call identified by the currentTool forEach variable. Workflow tools run as
dynamic subgraphs via flow.Subgraph, which parks the step and returns the child's result on re-entry; regular
tools run via a direct bus call.
*/
func (svc *Service) ExecuteTool(ctx context.Context, flow *workflow.Flow, currentTool llmapi.ToolCall) (toolResults []llmapi.ToolResult, err error) { // MARKER: ExecuteTool
	// toolSchemas is ambient flow state set once by InitChat, not a per-branch argument.
	var toolSchemas []llmapi.Tool
	flow.Get("toolSchemas", &toolSchemas)

	// Find the tool definition
	var def llmapi.Tool
	for _, t := range toolSchemas {
		if t.Name == currentTool.Name {
			def = t
			break
		}
	}
	if def.URL == "" {
		return nil, errors.New("tool not found: %s", currentTool.Name)
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
			svc.IncrementToolCalls(ctx, 1, def.URL, def.Type, "error")
			return nil, errors.Trace(err)
		}
		if yield {
			return nil, nil // parked, child workflow running; counted on re-entry
		}
		svc.IncrementToolCalls(ctx, 1, def.URL, def.Type, "ok")
		// Re-entry: the child's final_state is the tool result. Return a single-element delta that the
		// fan-in appends into the `toolResults` state key (Append-reduced).
		childOutput := out
		if len(childOutput) == 0 {
			childOutput = map[string]any{"status": "completed"}
		}
		resultJSON, _ := json.Marshal(childOutput)
		return []llmapi.ToolResult{{CallID: currentTool.ID, Output: string(resultJSON)}}, nil
	}

	// Regular endpoint tools are executed via direct bus call
	result, err := svc.executeTool(ctx, currentTool, toolSchemas)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Single-element delta; the fan-in appends it into the `toolResults` state key (Append-reduced).
	return []llmapi.ToolResult{{CallID: currentTool.ID, Output: string(result)}}, nil
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
	// toolResults is the only Append-reduced key: each forEach branch (ExecuteTool) contributes one
	// tool_result, and the reducer concatenates them at the NextLLM fan-in. The next CallLLM folds the
	// assembled toolResults into the conversation and clears the key. `items` itself is NOT reduced -
	// CallLLM owns it and writes the full conversation each turn (plain replace).
	graph.SetReducer("toolResults", workflow.ReducerAppend)
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
