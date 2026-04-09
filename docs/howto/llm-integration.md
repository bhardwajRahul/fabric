# LLM Integration

The [LLM core microservice](../structure/coreservices-llm.md) bridges LLM tool-calling protocols with Microbus endpoint invocations. Callers supply a list of `*openapi.Endpoint` values as tools, and the service drives the tool-calling loop - invoking each selected endpoint over the bus and feeding results back to the LLM until it produces a final response. It supports Claude, ChatGPT and Gemini backends via the `ProviderHostname` configuration.

## Configuration

Provider selection lives on `llm.core`. The provider's `BaseURL`, `APIKey`, and `Model` live on the provider microservice (`claudellm`, `chatgptllm`, or `geminillm`). The `APIKey` is a secret and belongs in `config.local.yaml`:

```yaml
# config.yaml
llm.core:
  ProviderHostname: claude.llm.core                    # or: chatgpt.llm.core, gemini.llm.core

claude.llm.core:
  Model: claude-haiku-4-5

# config.local.yaml (git-ignored)
claude.llm.core:
  APIKey: sk-ant-your-key-here
```

To switch providers, change `ProviderHostname` and supply the new provider's credentials under its hostname. The calling code does not change.

## Single Chat Request

The simplest usage is a synchronous `Chat` call. Pass messages and, optionally, tools:

```go
import "github.com/microbus-io/fabric/coreservices/llm/llmapi"

// Simple text conversation, no tools
messages := []llmapi.Message{
    {Role: "user", Content: "What is the capital of France?"},
}
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, nil)
// messagesOut contains the full conversation including the assistant's reply
```

### With Tools

Build tools from `*openapi.Endpoint` values exported by any downstream service's `*api` package. `llmapi.ToolsOf` reflects each endpoint's `InputArgs` into a JSON Schema and packages URL, method, and type for dispatch:

```go
import (
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
    "github.com/mycompany/myproject/calculator/calculatorapi"
    "github.com/mycompany/myproject/weather/weatherapi"
)

messages := []llmapi.Message{
    {Role: "user", Content: "What is 42 * 17, and what's the weather in Paris?"},
}
tools := llmapi.ToolsOf(
    calculatorapi.Arithmetic,
    weatherapi.Forecast,
)
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, tools)
```

The `Chat` endpoint runs the tool-calling loop internally: if the LLM requests a tool call, the service invokes the Microbus endpoint over the bus, feeds the result back to the LLM, and repeats until the LLM produces a final text response or `MaxToolRounds` is reached (configured on `llm.core`, default `10`).

Only `FeatureFunction`, `FeatureWeb`, and `FeatureWorkflow` endpoints are exposed - tasks and outbound events are silently skipped. When two endpoints share the same `Name`, the first keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order, so endpoints from multiple services can be concatenated without collision.

`Chat` returns `messagesOut`, the full conversation including the original messages and all new messages produced by the LLM. To continue the conversation, append a new user message to `messagesOut` and call `Chat` again.

## ChatLoop Workflow

For conversations that may involve many tool rounds or need durability, use the `ChatLoop` workflow. It performs the same logic as `Chat` but as a series of durable workflow steps orchestrated by the [Foreman](../structure/coreservices-foreman.md).

### Single Turn

```go
import (
    "github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// Run a single turn synchronously
status, state, err := foremanapi.NewClient(svc).Run(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What's the weather in San Francisco?"},
    },
    "tools": llmapi.ToolsOf(weatherapi.Forecast),
})
// state["messages"] contains the full conversation
```

### Multi-Turn via Continue

The `ChatLoop` workflow uses `ReducerAppend` for the `messages` field, which means `Continue` appends new messages to the completed conversation rather than replacing it:

```go
// First turn
tools := llmapi.ToolsOf(weatherapi.Forecast)
flowID, _ := foremanapi.NewClient(svc).Create(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What's the weather in San Francisco?"},
    },
    "tools": tools,
})
foremanapi.NewClient(svc).Start(ctx, flowID)
status, state, _ := foremanapi.NewClient(svc).Await(ctx, flowID)
// Present state["messages"] to the user...

// Second turn - Continue finds the latest completed flow in the thread and appends the new message.
// flowID doubles as the threadKey (any flowKey in the thread works)
newFlowID, _ := foremanapi.NewClient(svc).Continue(ctx, flowID, map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What about tomorrow?"},
    },
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

## Testing with Mocks

Mock the LLM service in tests to avoid needing a real API key:

```go
llmMock := llm.NewMock()
llmMock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, nil
})

app := application.New()
app.Add(svc, llmMock, tester)
app.RunInTest(t)
```

To test against the real `llm.core` without calling a live LLM API, mock the provider service (`claudellm`, `chatgptllm`, or `geminillm`) at the `Turn` boundary:

```go
claudeMock := claudellm.NewMock()
claudeMock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) {
    return &llmapi.TurnCompletion{Content: "Hello from mock!"}, nil
})

app := application.New()
app.Add(svc, llm.NewService(), claudeMock, tester)
app.RunInTest(t)
```

Mocking at the provider boundary exercises the full tool-calling loop, schema resolution, and bus dispatch in `llm.core` while keeping the test offline.
