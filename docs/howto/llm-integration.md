# LLM Integration

The [LLM core microservice](../structure/coreservices-llm.md) bridges LLM tool-calling protocols with Microbus endpoint invocations. Callers supply a list of canonical endpoint URLs as tools, a provider hostname, and a model identifier. The service drives the tool-calling loop - invoking each selected endpoint over the bus and feeding results back to the LLM until it produces a final response. It supports Claude, ChatGPT and Gemini providers out of the box, plus any custom microservice that implements the `Turn` contract.

## Configuration

Provider-specific settings (`BaseURL` and `APIKey`) live on the provider microservice (`claudellm`, `chatgptllm`, `geminillm`). The `APIKey` is a secret and belongs in `config.local.yaml`:

```yaml
# config.local.yaml (git-ignored)
claude.llm.core:
  APIKey: sk-ant-your-key-here
chatgpt.llm.core:
  APIKey: sk-your-openai-key
gemini.llm.core:
  APIKey: your-gemini-key
```

Provider and model are chosen per call - there is no global "active provider" or "active model" config. This forces both choices to be visible at every call site, which matters because models can differ in cost by 100x or more.

`llm.core` itself has a single optional config:

```yaml
# config.yaml
llm.core:
  MaxToolRounds: 10           # max tool call round-trips per Chat invocation
```

## Single Chat Request

The simplest usage is a synchronous `Chat` call. Pass provider, model, messages, and (optionally) tools and options:

```go
import (
    "github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// Simple text conversation, no tools
messages := []llmapi.Message{
    {Role: "user", Content: "What is the capital of France?"},
}
messagesOut, usage, err := llmapi.NewClient(svc).Chat(
    ctx,
    claudellmapi.Hostname,        // "claude.llm.core"
    claudellmapi.ModelHaiku45,    // "claude-haiku-4-5"
    messages,
    nil,                          // no tools
    nil,                          // no options
)
// messagesOut contains the full conversation including the assistant's reply.
// usage carries aggregated token counts across all turns.
```

Each provider's `*api` package exports typed model constants (e.g. `claudellmapi.ModelHaiku45`, `claudellmapi.ModelSonnet46`, `claudellmapi.ModelOpus47`, `chatgptllmapi.ModelGPT4o`, `geminillmapi.ModelGemini25Flash`), so a typo is a compile error rather than a runtime failure.

### With Tools

Tools are passed as a `[]string` of canonical Microbus endpoint URLs. Each `Def` value in a downstream service's `*api` package has a `URL()` helper that returns its canonical form:

```go
import (
    "github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
    "github.com/mycompany/myproject/calculator/calculatorapi"
    "github.com/mycompany/myproject/weather/weatherapi"
)

messages := []llmapi.Message{
    {Role: "user", Content: "What is 42 * 17, and what's the weather in Paris?"},
}
toolURLs := []string{
    calculatorapi.Arithmetic.URL(),
    weatherapi.Forecast.URL(),
}
messagesOut, usage, err := llmapi.NewClient(svc).Chat(
    ctx,
    claudellmapi.Hostname,
    claudellmapi.ModelHaiku45,
    messages,
    toolURLs,
    nil,
)
```

The `Chat` endpoint runs the tool-calling loop internally: it fetches each host's `:888/openapi.json` document, reflects the matching operation's request-body schema into a JSON Schema, and exposes the tool to the LLM. If the LLM requests a tool call, the service invokes the Microbus endpoint over the bus, feeds the result back to the LLM, and repeats until the LLM produces a final text response or `MaxToolRounds` is reached (configured on `llm.core`, default `10`; can be overridden per call via `ChatOptions.MaxToolRounds`).

Only `FeatureFunction`, `FeatureWeb`, and `FeatureWorkflow` endpoints are exposed - tasks and outbound events are silently skipped. When two endpoints share the same operation name, the first keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order, so URLs from multiple services can be concatenated without collision.

`Chat` returns `messagesOut` (the full conversation including original messages and all new messages from the LLM) and `usage` (aggregated token counts). To continue the conversation, append a new user message to `messagesOut` and call `Chat` again.

### ChatOptions

```go
opts := &llmapi.ChatOptions{
    MaxToolRounds: 5,        // overrides the MaxToolRounds config for this call
    MaxTokens:     1024,     // caps response length per turn
    Temperature:   0.2,      // sampling randomness
}
messagesOut, usage, err := llmapi.NewClient(svc).Chat(ctx, provider, model, messages, toolURLs, opts)
```

### Switching Providers

To swap providers, change the `provider` and `model` arguments. The calling code does not need any other change:

```go
// Same call, different provider
messagesOut, usage, err := llmapi.NewClient(svc).Chat(
    ctx,
    chatgptllmapi.Hostname,       // "chatgpt.llm.core"
    chatgptllmapi.ModelGPT4o,
    messages,
    toolURLs,
    nil,
)
```

## ChatLoop Workflow

For conversations that may involve many tool rounds or need durability, use the `ChatLoop` workflow. It performs the same logic as `Chat` but as a series of durable workflow steps orchestrated by the [Foreman](../structure/coreservices-foreman.md). Inputs match `Chat`; outputs are `messages` and `usage`.

### Single Turn

```go
import (
    "github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

status, state, err := foremanapi.NewClient(svc).Run(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "provider": claudellmapi.Hostname,
    "model":    claudellmapi.ModelHaiku45,
    "messages": []llmapi.Message{
        {Role: "user", Content: "What's the weather in San Francisco?"},
    },
    "tools":    []string{weatherapi.Forecast.URL()},
})
// state["messages"] contains the full conversation
// state["usage"] contains the aggregated llmapi.Usage
```

### Multi-Turn via Continue

The `ChatLoop` workflow uses `ReducerAppend` for the `messages` field, which means `Continue` appends new messages to the completed conversation rather than replacing it:

```go
flowID, _ := foremanapi.NewClient(svc).Create(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "provider": claudellmapi.Hostname,
    "model":    claudellmapi.ModelHaiku45,
    "messages": []llmapi.Message{{Role: "user", Content: "What's the weather in San Francisco?"}},
    "tools":    []string{weatherapi.Forecast.URL()},
})
foremanapi.NewClient(svc).Start(ctx, flowID)
status, state, _ := foremanapi.NewClient(svc).Await(ctx, flowID)
// Present state["messages"] to the user...

// Second turn - Continue finds the latest completed flow in the thread and appends the new message.
// flowID doubles as the threadKey (any flowKey in the thread works).
newFlowID, _ := foremanapi.NewClient(svc).Continue(ctx, flowID, map[string]any{
    "messages": []llmapi.Message{{Role: "user", Content: "What about tomorrow?"}},
})
foremanapi.NewClient(svc).Start(ctx, newFlowID)
status, state, _ = foremanapi.NewClient(svc).Await(ctx, newFlowID)
```

Each `Continue` creates a new flow in the same thread, starting from the final state of the latest completed flow with the new messages appended. The caller can pass any flowKey from the thread - `Continue` automatically finds the latest one.

### When to Use Chat vs ChatLoop

| | `Chat` | `ChatLoop` |
|---|---|---|
| **Simplicity** | One function call | Requires Foreman setup |
| **Durability** | None - timeout loses all work | Full state persisted after each step |
| **Time budget** | Must complete within one request timeout | Each step fits within a normal timeout |
| **Multi-turn** | Caller manages conversation manually | `Continue` chains turns with state preserved |
| **Debugging** | Standard error handling | `History` shows step-by-step execution trace |

Use `Chat` for simple, quick interactions. Use `ChatLoop` when the conversation may involve many tool rounds, when you need durability against failures, or when you want the Foreman's debugging and continuation capabilities.

## Token Usage and Metrics

Every `Chat` call returns an `llmapi.Usage` aggregating token consumption across all turns:

```go
type Usage struct {
    InputTokens      int    // prompt tokens charged
    OutputTokens     int    // completion tokens generated
    CacheReadTokens  int    // tokens served from the provider's prompt cache
    CacheWriteTokens int    // tokens written to the provider's prompt cache
    Model            string // provider's model identifier that produced this completion
    Turns            int    // number of LLM turns aggregated
}
```

The `claudellm` provider sets two `cache_control` breakpoints on requests so Anthropic's prompt cache can be reused across turns. Cached input is reflected in `CacheReadTokens` / `CacheWriteTokens`.

Per-turn token consumption is also emitted as the `microbus_llm_tokens_total` counter metric, labeled by `provider`, `model`, and `direction` (`input`, `output`, `cacheRead`, `cacheWrite`). The bundled LLM Grafana dashboard charts tokens by direction/provider/model and the cache hit ratio.

## Testing with Mocks

Mock the LLM service in tests to avoid needing a real API key:

```go
llmMock := llm.NewMock()
llmMock.MockChat(func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, llmapi.Usage{Turns: 1}, nil
})

app := application.New()
app.Add(svc, llmMock, tester)
app.RunInTest(t)
```

To test against the real `llm.core` without calling a live LLM API, mock the provider service (`claudellm`, `chatgptllm`, or `geminillm`) at the `Turn` boundary:

```go
claudeMock := claudellm.NewMock()
claudeMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
    return "Hello from mock!", nil, llmapi.Usage{Model: model, Turns: 1}, nil
})

app := application.New()
app.Add(svc, llm.NewService(), claudeMock, tester)
app.RunInTest(t)
```

Mocking at the provider boundary exercises the full tool-calling loop, schema resolution, and bus dispatch in `llm.core` while keeping the test offline.
