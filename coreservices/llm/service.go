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

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

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
func (svc *Service) turn(ctx context.Context, providerHost string, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
	content, toolCalls, usage, err = llmapi.NewClient(svc).ForHost(providerHost).Turn(ctx, model, messages, tools, options)
	if err != nil {
		return "", nil, llmapi.Usage{}, errors.Trace(err)
	}
	return content, toolCalls, usage, nil
}

/*
Turn on llm.core is a stub. The interface contract is defined here so providers can implement it,
but llm.core does not route or delegate. Callers should ForHost(<providerHostname>) directly to
hit a specific provider, or use Chat for the full conversation loop.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) { // MARKER: Turn
	return "", nil, llmapi.Usage{}, errors.New("not implemented on llm.core; call ForHost(<provider>).Turn or use Chat", http.StatusNotImplemented)
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
		content, toolCalls, turnUsage, err := svc.turn(ctx, provider, model, currentMessages, tools, turnOpts)
		if err != nil {
			return nil, usage, errors.Trace(err)
		}
		svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)
		usage.Add(turnUsage)

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
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
	content, toolCalls, turnUsage, err := svc.turn(ctx, provider, model, currentMessages, nil, turnOpts)
	if err != nil {
		return nil, usage, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)
	usage.Add(turnUsage)
	if content != "" {
		messagesOut = append(messagesOut, llmapi.Message{
			Role:    "assistant",
			Content: content,
		})
	}
	return messagesOut, usage, nil
}

/*
InitChat stores the caller-supplied tools and options in flow state for use by the chat loop.
*/
func (svc *Service) InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error) { // MARKER: InitChat
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
	content, toolCalls, turnUsage, err := svc.turn(ctx, provider, model, messages, tools, turnOpts)
	if err != nil {
		return "", nil, llmapi.Usage{}, errors.Trace(err)
	}
	svc.logCompletion(ctx, provider, content, toolCalls, turnUsage)

	return content, toolCalls, turnUsage, nil
}

/*
ProcessResponse inspects the LLM response, accumulates usage, and routes to the next step.
*/
func (svc *Service) ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) { // MARKER: ProcessResponse
	// Read internal state
	var pending []llmapi.ToolCall
	flow.Get("pendingToolCalls", &pending)

	// Get accumulated response messages and usage
	flow.Get("messagesOut", &messagesOut)
	flow.Get("usage", &usageOut)
	usageOut.Add(turnUsage)

	toolRoundsOut = toolRounds

	if len(pending) == 0 {
		// No tool calls - we're done
		toolsRequested = false
		if llmContent != "" {
			messagesOut = append(messagesOut, llmapi.Message{
				Role:    "assistant",
				Content: llmContent,
			})
		}
		return messagesOut, toolsRequested, toolRoundsOut, usageOut, nil
	}

	// Tool calls present - check round limit
	if toolRounds >= maxToolRounds {
		// Exhausted rounds, return what we have
		toolsRequested = false
		if llmContent != "" {
			messagesOut = append(messagesOut, llmapi.Message{
				Role:    "assistant",
				Content: llmContent,
			})
		}
		return messagesOut, toolsRequested, toolRoundsOut, usageOut, nil
	}

	// Tool calls present - record assistant message and set up forEach
	toolsRequested = true
	toolRoundsOut = toolRounds + 1
	if llmContent != "" {
		messagesOut = append(messagesOut, llmapi.Message{
			Role:    "assistant",
			Content: llmContent,
		})
		// Also add to conversation history for the LLM
		var messages []llmapi.Message
		flow.Get("messages", &messages)
		messages = append(messages, llmapi.Message{
			Role:    "assistant",
			Content: llmContent,
		})
		flow.Set("messages", messages)
	}
	flow.Set("messagesOut", messagesOut)
	flow.Set("usage", usageOut)

	return messagesOut, toolsRequested, toolRoundsOut, usageOut, nil
}

/*
ExecuteTool executes a single tool call identified by the currentTool forEach variable.
On re-entry after a dynamic subgraph completes, toolExecuted is true and the child's
output is in state.
*/
func (svc *Service) ExecuteTool(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error) { // MARKER: ExecuteTool
	if toolExecuted {
		// Re-entry after a workflow subgraph completed.
		// Identify child output by diffing current state keys against the pre-subgraph snapshot.
		var preKeys []string
		flow.Get("preSubgraphKeys", &preKeys)
		preKeySet := make(map[string]bool, len(preKeys))
		for _, k := range preKeys {
			preKeySet[k] = true
		}

		// Any state field not in the pre-subgraph snapshot is child output
		snap := flow.Snapshot()
		childOutput := make(map[string]any)
		for k, v := range snap {
			if !preKeySet[k] {
				childOutput[k] = v
			}
		}
		if len(childOutput) == 0 {
			childOutput["status"] = "completed"
		}
		resultJSON, _ := json.Marshal(childOutput)

		var messages []llmapi.Message
		flow.Get("messages", &messages)
		messages = append(messages, llmapi.Message{
			Role:    "tool",
			Content: string(resultJSON),
		})
		flow.Set("messages", messages)
		return true, nil
	}

	// First run - execute the tool
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
		return false, errors.New("tool not found: %s", currentTool.Name)
	}

	// Workflow tools are executed as dynamic subgraphs
	if def.Type == "workflow" {
		// Snapshot current state keys so we can identify child output on re-entry
		snap := flow.Snapshot()
		stateKeys := make([]string, 0, len(snap))
		for k := range snap {
			stateKeys = append(stateKeys, k)
		}
		flow.Set("preSubgraphKeys", stateKeys)

		inputState := make(map[string]any)
		if currentTool.Arguments != nil {
			json.Unmarshal(currentTool.Arguments, &inputState)
		}
		flow.Subgraph(def.URL, inputState)
		return true, nil
	}

	// Regular endpoint tools are executed via direct bus call
	result, err := svc.executeTool(ctx, currentTool, tools)
	if err != nil {
		return false, errors.Trace(err)
	}

	var messages []llmapi.Message
	flow.Get("messages", &messages)
	messages = append(messages, llmapi.Message{
		Role:    "tool",
		Content: string(result),
	})
	flow.Set("messages", messages)
	return true, nil
}

/*
ChatLoop defines the workflow graph for the LLM chat loop.
*/
func (svc *Service) ChatLoop(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: ChatLoop
	initChat := llmapi.InitChat.URL()
	callLLM := llmapi.CallLLM.URL()
	processResponse := llmapi.ProcessResponse.URL()
	executeTool := llmapi.ExecuteTool.URL()

	graph = workflow.NewGraph(llmapi.ChatLoop.URL())

	// InitChat → CallLLM → ProcessResponse
	graph.AddTransition(initChat, callLLM)
	graph.AddTransition(callLLM, processResponse)

	// ProcessResponse → END when no tools requested
	graph.AddTransitionWhen(processResponse, workflow.END, "!toolsRequested")

	// ProcessResponse → forEach(pendingToolCalls) → ExecuteTool → CallLLM when tools requested
	graph.AddTransitionForEach(processResponse, executeTool, "pendingToolCalls", "currentTool")
	graph.AddTransition(executeTool, callLLM)

	// Reducer for messages - fan-in from parallel ExecuteTool branches
	graph.SetReducer("messages", workflow.ReducerAppend)

	// Declare inputs and outputs
	graph.DeclareInputs("provider", "model", "messages", "tools", "options")
	graph.DeclareOutputs("messages", "usage")

	return graph, nil
}
