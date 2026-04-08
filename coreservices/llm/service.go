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

// logCompletion logs the LLM's response content and tool call names.
func (svc *Service) logCompletion(ctx context.Context, resp *llmapi.TurnCompletion) {
	reply := resp.Content
	if len(reply) > 1024 {
		reply = reply[:1024] + "..."
	}
	toolNames := make([]string, len(resp.ToolCalls))
	for i, tc := range resp.ToolCalls {
		toolNames[i] = tc.Name
	}
	svc.LogInfo(ctx, "LLM answered", "reply", reply, "toolCalls", toolNames)
}

// turn calls the provider's Turn endpoint over the bus.
func (svc *Service) turn(ctx context.Context, messages []llmapi.Message, toolDefs []llmapi.ToolDef) (*llmapi.TurnCompletion, error) {
	completion, err := llmapi.NewClient(svc).ForHost(svc.ProviderHostname()).Turn(ctx, messages, toolDefs)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return completion, nil
}

/*
Turn executes a single LLM turn. This endpoint delegates to the configured provider microservice.
*/
func (svc *Service) Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) { // MARKER: Turn
	return svc.turn(ctx, messages, tools)
}

/*
Chat sends messages to an LLM with optional tools and returns the response messages.

Input:
  - messages: messages is the conversation history to send to the LLM
  - tools: tools is a list of Microbus endpoint URLs to expose as LLM tools

Output:
  - messagesOut: messagesOut is the full conversation including new messages produced by the LLM
*/
func (svc *Service) Chat(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error) { // MARKER: Chat
	// Resolve tool schemas from OpenAPI
	var toolDefs []llmapi.ToolDef
	if len(tools) > 0 {
		toolDefs, err = svc.resolveTools(ctx, tools)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	maxRounds := svc.MaxToolRounds()

	// Conversation with the LLM
	currentMessages := make([]llmapi.Message, len(messages))
	copy(currentMessages, messages)

	for round := range maxRounds {
		_ = round
		// Call the LLM
		svc.logPrompt(ctx, currentMessages)
		resp, err := svc.turn(ctx, currentMessages, toolDefs)
		if err != nil {
			return nil, errors.Trace(err)
		}
		svc.logCompletion(ctx, resp)

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				messagesOut = append(messagesOut, llmapi.Message{
					Role:    "assistant",
					Content: resp.Content,
				})
			}
			return messagesOut, nil
		}

		// Record the assistant's response with tool calls metadata
		toolCallsJSON, _ := json.Marshal(resp.ToolCalls)
		assistantMsg := llmapi.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: string(toolCallsJSON),
		}
		messagesOut = append(messagesOut, assistantMsg)
		currentMessages = append(currentMessages, assistantMsg)

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result, err := svc.executeTool(ctx, tc, toolDefs)
			if err != nil {
				return nil, errors.Trace(err)
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
	resp, err := svc.turn(ctx, currentMessages, nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	svc.logCompletion(ctx, resp)
	if resp.Content != "" {
		messagesOut = append(messagesOut, llmapi.Message{
			Role:    "assistant",
			Content: resp.Content,
		})
	}
	return messagesOut, nil
}

/*
InitChat validates inputs, resolves tool schemas from OpenAPI, and stores them in flow state.
*/
func (svc *Service) InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (maxToolRounds int, toolRounds int, err error) { // MARKER: InitChat
	// Resolve tool schemas
	if len(tools) > 0 {
		toolDefs, err := svc.resolveTools(ctx, tools)
		if err != nil {
			return 0, 0, errors.Trace(err)
		}
		flow.Set("toolSchemas", toolDefs)
	}

	maxToolRounds = svc.MaxToolRounds()
	toolRounds = 0
	return maxToolRounds, toolRounds, nil
}

/*
CallLLM sends the current messages and tools to the LLM provider.
*/
func (svc *Service) CallLLM(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message) (llmContent string, pendingToolCalls any, err error) { // MARKER: CallLLM
	// Read tool schemas
	var toolDefs []llmapi.ToolDef
	flow.Get("toolSchemas", &toolDefs)

	// Call the LLM
	svc.logPrompt(ctx, messages)
	resp, err := svc.turn(ctx, messages, toolDefs)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	svc.logCompletion(ctx, resp)

	return resp.Content, resp.ToolCalls, nil
}

/*
ProcessResponse inspects the LLM response and routes to the next step.
*/
func (svc *Service) ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, err error) { // MARKER: ProcessResponse
	// Read internal state
	var pending []llmapi.ToolCall
	flow.Get("pendingToolCalls", &pending)

	// Get accumulated response messages
	flow.Get("messagesOut", &messagesOut)

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
		return messagesOut, toolsRequested, toolRoundsOut, nil
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
		return messagesOut, toolsRequested, toolRoundsOut, nil
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

	return messagesOut, toolsRequested, toolRoundsOut, nil
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
	var toolDefs []llmapi.ToolDef
	flow.Get("toolSchemas", &toolDefs)

	// Find the tool definition
	var def llmapi.ToolDef
	for _, t := range toolDefs {
		if t.Name == currentTool.Name {
			def = t
			break
		}
	}
	if def.URL == "" {
		return false, errors.New("tool not found: %s", currentTool.Name)
	}

	// Workflow tools are executed as dynamic subgraphs
	if def.FeatureType == "workflow" {
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
	result, err := svc.executeTool(ctx, currentTool, toolDefs)
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
	graph.DeclareInputs("messages", "tools")
	graph.DeclareOutputs("messages")

	return graph, nil
}
