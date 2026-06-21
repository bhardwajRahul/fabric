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
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/chatgptllm"
	"github.com/microbus-io/fabric/coreservices/chatgptllm/chatgptllmapi"
	"github.com/microbus-io/fabric/coreservices/claudellm"
	"github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/coreservices/geminillm"
	"github.com/microbus-io/fabric/coreservices/geminillm/geminillmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *workflow.Flow
	_ testarossa.Asserter
	_ llmapi.Client
	_ json.Encoder
	_ httpegress.Service
	_ claudellm.Mock
	_ calculator.Service
)

// MARKER: Chat

func TestLLM_Chat(t *testing.T) { // MARKER: Chat
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
			result, usage, err := client.Chat(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, nil, nil)
			assert.Expect(
				result, expectedResult,
				usage.Turns, 1,
				err, nil,
			)
		})
	*/
}

func TestLLM_ChatLive(t *testing.T) {
	t.Skip("Manual test - set real API keys and comment this line to run")
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	claude := claudellm.NewService()
	claude.SetAPIKey("") // sk-...

	gemini := geminillm.NewService()
	gemini.SetAPIKey("") // AI...

	chatgpt := chatgptllm.NewService()
	chatgpt.SetAPIKey("") // sk-...

	egressSvc := httpegress.NewService()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, claude, gemini, chatgpt, egressSvc, calcSvc, tester)
	app.RunInTest(t)

	t.Run("text_only_claude", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is the capital of France? Answer in one word."}}
		result, _, err := client.Chat(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, nil, nil)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})

	t.Run("text_only_gemini", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is the capital of Japan? Answer in one word."}}
		result, _, err := client.Chat(ctx, geminillmapi.Hostname, geminillmapi.ModelGemini20Flash, messages, nil, nil)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})

	t.Run("text_only_chatgpt", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is the capital of Italy? Answer in one word."}}
		result, _, err := client.Chat(ctx, chatgptllmapi.Hostname, chatgptllmapi.ModelGPT4o, messages, nil, nil)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})

	t.Run("with_tools", func(t *testing.T) {
		assert := testarossa.For(t)
		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7? Use the calculator tool."}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, _, err := client.Chat(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, tools, nil)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})
}

func TestLLM_ChatWithMockedProvider(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	providerMock := claudellm.NewMock()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, providerMock, calcSvc, tester)
	app.RunInTest(t)

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		providerMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) {
			return "Hello from mocked provider!", nil, llmapi.StopReasonEndTurn, llmapi.Usage{InputTokens: 5, OutputTokens: 5, Model: model, Turns: 1}, nil
		})
		defer providerMock.MockTurn(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		result, usage, err := client.Chat(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, nil, nil)
		if assert.NoError(err) {
			assert.Expect(len(result), 1)
			assert.Expect(result[0].Role, "assistant")
			assert.Expect(result[0].Content, "Hello from mocked provider!")
			assert.Expect(usage.Turns, 1)
			assert.Expect(usage.OutputTokens, 5)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)
		callCount := 0
		providerMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) {
			callCount++
			if callCount == 1 {
				return "", []llmapi.ToolCall{{
					ID:        "call_1",
					Name:      "Arithmetic",
					Arguments: json.RawMessage(`{"x":3,"op":"+","y":5}`),
				}}, llmapi.StopReasonToolUse, llmapi.Usage{InputTokens: 10, OutputTokens: 5, Model: model, Turns: 1}, nil
			}
			return "3 + 5 = 8", nil, llmapi.StopReasonEndTurn, llmapi.Usage{InputTokens: 15, OutputTokens: 8, Model: model, Turns: 1}, nil
		})
		defer providerMock.MockTurn(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, usage, err := client.Chat(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, tools, nil)
		if assert.NoError(err) {
			assert.Expect(len(result) >= 2, true)
			last := result[len(result)-1]
			assert.Expect(last.Role, "assistant")
			assert.Expect(last.Content, "3 + 5 = 8")
			assert.Expect(usage.Turns, 2)
		}
	})
}

func TestLLM_ChatLoop(t *testing.T) { // MARKER: ChatLoop
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	providerMock := claudellm.NewMock()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := llmapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(svc, providerMock, calcSvc, foreman.NewService(), tester)
	app.RunInTest(t)

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		providerMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) {
			return "Hello from the workflow!", nil, llmapi.StopReasonEndTurn, llmapi.Usage{InputTokens: 5, OutputTokens: 5, Model: model, Turns: 1}, nil
		})
		defer providerMock.MockTurn(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		out, usage, status, err := exec.ChatLoop(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, nil, nil)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			usage.Turns, 1,
			usage.OutputTokens, 5,
		)
		// Output messages = input history + assistant reply (the messages field is Append-reduced).
		if assert.Expect(len(out) >= 1, true) {
			last := out[len(out)-1]
			assert.Expect(last.Role, "assistant")
			assert.Expect(last.Content, "Hello from the workflow!")
		}
	})

	t.Run("tool_calling_loop", func(t *testing.T) {
		assert := testarossa.For(t)

		callCount := 0
		providerMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) {
			callCount++
			if callCount == 1 {
				// First turn: ask for a tool call.
				return "", []llmapi.ToolCall{{
					ID:        "call_1",
					Name:      "Arithmetic",
					Arguments: json.RawMessage(`{"x":3,"op":"+","y":5}`),
				}}, llmapi.StopReasonToolUse, llmapi.Usage{InputTokens: 10, OutputTokens: 5, Model: model, Turns: 1}, nil
			}
			// Second turn (after fan-in at nextLLM): finalize.
			return "3 + 5 = 8", nil, llmapi.StopReasonEndTurn, llmapi.Usage{InputTokens: 15, OutputTokens: 8, Model: model, Turns: 1}, nil
		})
		defer providerMock.MockTurn(nil)

		// ChatLoop's InitChat resolves URLs to tool schemas via the host's OpenAPI document,
		// so we pass the calculator's canonical URL and let InitChat fetch the schema.
		toolURLs := []string{calculatorapi.Arithmetic.URL()}
		messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
		out, usage, status, err := exec.ChatLoop(ctx, claudellmapi.Hostname, claudellmapi.ModelHaiku45, messages, toolURLs, nil)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			usage.Turns, 2,
		)
		// After two turns (with a fan-in round in between), the final assistant reply lands.
		if assert.Expect(len(out) >= 1, true) {
			last := out[len(out)-1]
			assert.Expect(last.Role, "assistant")
			assert.Expect(last.Content, "3 + 5 = 8")
		}
	})
}

// TestLLM_ErrorProbe sends a ~1MB system prompt (well over 200k tokens) to each provider once and
// catalogs the shape of the resulting error. It is a manual investigation aid for the 429 / rate-limit
// classification work: paste real provider API keys below and enable with LLM_PROBE=1. The providers also
// log the raw status, headers, and body of every non-OK response, so run with -v to see both the
// normalized TracedError (statusCode / message / properties) and the raw upstream response.
//
// Do not commit real keys.
func TestLLM_ErrorProbe(t *testing.T) {
	if os.Getenv("LLM_PROBE") == "" {
		t.Skip("Live error-shape probe; paste provider API keys and set LLM_PROBE=1 to run")
	}
	ctx := t.Context()

	const (
		claudeKey  = "" // sk-ant-...
		geminiKey  = "" // AQ...
		chatgptKey = "" // sk-proj-...
	)

	svc := NewService()
	claude := claudellm.NewService()
	claude.SetAPIKey(claudeKey)
	gemini := geminillm.NewService()
	gemini.SetAPIKey(geminiKey)
	chatgpt := chatgptllm.NewService()
	chatgpt.SetAPIKey(chatgptKey)
	egressSvc := httpegress.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, claude, gemini, chatgpt, egressSvc, tester)
	app.RunInTest(t)

	// System prompt sized per provider at ~4 chars/token. unit is 20 chars; 50k units = ~1MB = ~250k tokens
	// (over the 200k context of claude/gpt-4o). Gemini's window is ~1M tokens, so it needs ~6MB (~1.5M tokens).
	prompt := func(units int) string { return strings.Repeat("word word word word ", units) }

	providers := []struct {
		name, host, model, key string
		promptUnits            int
	}{
		{"claude", claudellmapi.Hostname, claudellmapi.ModelHaiku45, claudeKey, 50_000},
		{"gemini", geminillmapi.Hostname, "gemini-2.5-flash", geminiKey, 300_000},
		{"chatgpt", chatgptllmapi.Hostname, chatgptllmapi.ModelGPT4o, chatgptKey, 50_000},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			if p.key == "" {
				t.Skipf("no API key pasted for %s", p.name)
			}
			messages := []llmapi.Message{
				{Role: "system", Content: prompt(p.promptUnits)},
				{Role: "user", Content: "Summarize the above in one word."},
			}
			_, usage, err := client.Chat(ctx, p.host, p.model, messages, nil, nil)
			if err == nil {
				t.Logf("[%s] unexpected success; usage=%+v", p.name, usage)
				return
			}
			te := errors.Convert(err)
			t.Logf("\n===== %s =====\nstatusCode: %d\nmessage:    %s\nproperties: %#v",
				p.name, te.StatusCode, te.Error(), te.Properties)
		})
	}
}

// TestLLM_ErrorProbeGeminiBurst sends several large Gemini requests back-to-back to trip a genuine (transient)
// RESOURCE_EXHAUSTED: the burst overflows the per-minute window even though no single request exceeds the quota, so
// the geminillm parser must keep it retryable (a retryAfter is attached) rather than treat it as the single-oversized
// poison case. Each request is sized so its conservative estimate (~2 chars/token) still fits the quota; only the
// accumulated burst overflows. Enable with LLM_PROBE=1 and a pasted Gemini key. Run with -v.
func TestLLM_ErrorProbeGeminiBurst(t *testing.T) {
	if os.Getenv("LLM_PROBE") == "" {
		t.Skip("Live error-shape probe; paste a Gemini API key and set LLM_PROBE=1 to run")
	}
	ctx := t.Context()

	const geminiKey = "" // AQ...
	if geminiKey == "" {
		t.Skip("no Gemini API key pasted")
	}

	svc := NewService()
	gemini := geminillm.NewService()
	gemini.SetAPIKey(geminiKey)
	egressSvc := httpegress.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, gemini, egressSvc, tester)
	app.RunInTest(t)

	// At ~4 tokens per 20-char unit, 90k units is ~360k tokens (~1.8M chars). The conservative ~2 chars/token estimate
	// is ~900k < the 1M/min quota, so each request alone is classified transient (not poison); three back-to-back in
	// the same minute total ~1.08M > 1M, so the overflowing one trips a retryable RESOURCE_EXHAUSTED.
	burstPrompt := strings.Repeat("word word word word ", 90_000)

	for i := 1; i <= 3; i++ {
		messages := []llmapi.Message{
			{Role: "system", Content: burstPrompt},
			{Role: "user", Content: "Summarize the above in one word."},
		}
		_, usage, err := client.Chat(ctx, geminillmapi.Hostname, "gemini-2.5-flash", messages, nil, nil)
		if err == nil {
			t.Logf("\n===== request %d: SUCCESS =====\nusage=%+v", i, usage)
			continue
		}
		te := errors.Convert(err)
		t.Logf("\n===== request %d: ERROR =====\nstatusCode: %d\nmessage:    %s\nproperties: %#v",
			i, te.StatusCode, te.Error(), te.Properties)
	}
}
