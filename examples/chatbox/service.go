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

package chatbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
	"github.com/microbus-io/fabric/examples/chatbox/chatboxapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ chatboxapi.Client
)

/*
Service implements the chatbox.example microservice.

Chatbox is a demo LLM provider that pattern-matches user messages to demonstrate the tool-calling flow.
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

// mathPattern matches questions like "what is 6 * 7", "how much is 10 + 3", "calculate 100 / 4"
var mathPattern = regexp.MustCompile(`(?i)(?:what is|how much is|calculate|compute|what's|whats)\s+(\d+)\s*([\+\-\*\/x]|plus|minus|times|multiplied by|divided by|over)\s*(\d+)`)

// opMap normalizes English operator names to symbols.
var opMap = map[string]string{
	"+": "+", "-": "-", "*": "*", "/": "/", "x": "*",
	"plus": "+", "minus": "-", "times": "*", "multiplied by": "*", "divided by": "/", "over": "/",
}

/*
Turn executes a single LLM turn using the chatbox demo provider.
It pattern-matches math questions and generates tool calls to the calculator.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) { // MARKER: Turn
	usage = llmapi.Usage{Model: model, Turns: 1}

	if len(messages) == 0 {
		return "I'm the Chatbox demo. Ask me a math question!", nil, usage, nil
	}

	lastMsg := messages[len(messages)-1]

	// If the last message is a tool result, format it as a response
	if lastMsg.Role == "tool" {
		var result map[string]any
		json.Unmarshal([]byte(lastMsg.Content), &result)
		if r, ok := result["result"]; ok {
			return fmt.Sprintf("The answer is %v.", r), nil, usage, nil
		}
		return fmt.Sprintf("The result is: %s", lastMsg.Content), nil, usage, nil
	}

	// Try to match a math question
	if lastMsg.Role == "user" {
		matches := mathPattern.FindStringSubmatch(lastMsg.Content)
		if matches != nil {
			x, _ := strconv.Atoi(matches[1])
			opStr := strings.TrimSpace(strings.ToLower(matches[2]))
			y, _ := strconv.Atoi(matches[3])
			op, ok := opMap[opStr]
			if !ok {
				op = opStr
			}

			// Check if we have a calculator tool available
			var calcTool *llmapi.Tool
			for i := range tools {
				if strings.Contains(strings.ToLower(tools[i].Name), "arithmetic") ||
					strings.Contains(strings.ToLower(tools[i].Name), "calculator") {
					calcTool = &tools[i]
					break
				}
			}

			if calcTool != nil {
				args, _ := json.Marshal(map[string]any{"x": x, "op": op, "y": y})
				return fmt.Sprintf("I'll use the calculator to compute %d %s %d.", x, op, y),
					[]llmapi.ToolCall{{
						ID:        "chatbox_1",
						Name:      calcTool.Name,
						Arguments: args,
					}}, usage, nil
			}

			// No calculator tool - do the math ourselves
			var answer int
			switch op {
			case "+":
				answer = x + y
			case "-":
				answer = x - y
			case "*":
				answer = x * y
			case "/":
				if y != 0 {
					answer = x / y
				} else {
					return "Cannot divide by zero.", nil, usage, nil
				}
			}
			return fmt.Sprintf("%d %s %d = %d", x, op, y, answer), nil, usage, nil
		}

		// No pattern matched
		return "I don't understand. I'm the Chatbox demo and I can only answer math questions like \"What is 6 times 7?\"", nil, usage, nil
	}

	return "I don't understand that message.", nil, usage, nil
}

/*
Demo serves the interactive demo page for the chatbox.
*/
func (svc *Service) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	if r.Method == "GET" {
		err = svc.WriteResTemplate(w, "demo.html", nil)
		return errors.Trace(err)
	}

	// POST: process the chat message via the LLM service
	if err = r.ParseForm(); err != nil {
		return errors.Trace(err)
	}
	userMessage := r.FormValue("message")
	if userMessage == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing message"))
		return nil
	}
	provider := r.FormValue("provider")
	if provider == "" {
		provider = Hostname
	}
	model := r.FormValue("model")
	if model == "" {
		model = "chatbox-default"
	}

	// Call the LLM service's Chat endpoint with the calculator as a tool.
	messages := []llmapi.Message{{Role: "user", Content: userMessage}}
	tools := []string{calculatorapi.Arithmetic.URL()}
	result, _, err := llmapi.NewClient(svc).Chat(r.Context(), provider, model, messages, tools, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return nil
	}

	// Return the conversation as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
	return nil
}
