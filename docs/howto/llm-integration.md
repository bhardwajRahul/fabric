# LLM Integration

The [LLM core microservice](../structure/coreservices-llm.md) bridges LLM tool-calling protocols with Microbus endpoint invocations. Any Microbus endpoint is automatically a tool - the service fetches schemas from OpenAPI and translates them into the provider's native format. It supports Claude, OpenAI and Gemini backends via configuration.

## Configuration

Set the LLM provider, model and API key. The API key is a secret and should go in `config.local.yaml`:

```yaml
# config.yaml
llm.core:
  Provider: claude                                      # or: openai, gemini
  BaseURL: https://api.anthropic.com                    # or: https://api.openai.com, https://generativelanguage.googleapis.com
  Model: claude-haiku-4-5                               # or: gpt-4, gemini-pro

# config.local.yaml (git-ignored)
llm.core:
  APIKey: sk-ant-your-key-here
```

## Single Chat Request

The simplest usage is a single synchronous `Chat` call. Pass messages and optionally a list of Microbus endpoint URLs as tools:

```go
import "github.com/microbus-io/fabric/coreservices/llm/llmapi"

// Simple text conversation
messages := []llmapi.Message{
    {Role: "user", Content: "What is the capital of France?"},
}
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, nil, 0)
// messagesOut contains the full conversation including the assistant's reply
```

### With Tools

Pass Microbus endpoint URLs as tools. The LLM service fetches each endpoint's OpenAPI schema automatically and presents them to the LLM as callable tools:

```go
messages := []llmapi.Message{
    {Role: "user", Content: "What is 42 * 17?"},
}
tools := []llmapi.Tool{
    {URL: "https://calculator.example:443/arithmetic"},
}
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, tools, 5)
```

The `Chat` endpoint handles the tool-calling loop internally: if the LLM requests a tool call, the service invokes the Microbus endpoint over the bus, feeds the result back to the LLM, and repeats until the LLM produces a final text response or the round limit is reached.

The `maxToolRounds` parameter controls how many tool call round-trips are allowed. Pass `0` to use the configured default (10).

`Chat` returns `messagesOut`, the full conversation including the original messages and all new messages produced by the LLM. To continue the conversation, append a new user message to `messagesOut` and call `Chat` again.

## ChatLoop Workflow

For conversations that may involve many tool rounds or need durability, use the `ChatLoop` workflow. It performs the same Chat logic but as a series of durable workflow steps orchestrated by the [Foreman](../structure/coreservices-foreman.md).

### Single Turn

```go
import "github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
import "github.com/microbus-io/fabric/coreservices/llm/llmapi"

// Run a single turn synchronously
status, state, err := foremanapi.NewClient(svc).Run(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What's the weather in San Francisco?"},
    },
    "tools": []llmapi.Tool{
        {URL: "https://weather.svc:443/forecast"},
    },
})
// state["messages"] contains the full conversation
```

### Multi-Turn via Continue

The ChatLoop workflow uses `ReducerAppend` for the `messages` field, which means `Continue` appends new messages to the completed conversation rather than replacing it. This enables a natural multi-turn pattern:

```go
// First turn
flowID, _ := foremanapi.NewClient(svc).Create(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What's the weather in San Francisco?"},
    },
    "tools": tools,
})
foremanapi.NewClient(svc).Start(ctx, flowID)
status, state, _ := foremanapi.NewClient(svc).Await(ctx, flowID)
// Present state["messages"] to the user...

// Second turn - Continue finds the latest completed flow in the thread and appends the new message
// flowID doubles as the threadKey (any flowKey in the thread works)
newFlowID, _ := foremanapi.NewClient(svc).Continue(ctx, flowID, map[string]any{
    "messages": []llmapi.Message{
        {Role: "user", Content: "What about tomorrow?"},
    },
})
foremanapi.NewClient(svc).Start(ctx, newFlowID)
status, state, _ = foremanapi.NewClient(svc).Await(ctx, newFlowID)
```

Each `Continue` creates a new flow in the same thread, starting from the final state of the latest completed flow, with the new messages appended. The caller can pass any flowKey from the thread - `Continue` automatically finds the latest one. The workflow re-resolves tool schemas and runs a fresh LLM turn with the full conversation context.

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
llmMock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool, maxToolRounds int) (messagesOut []llmapi.Message, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, nil
})

app := application.New()
app.Add(svc, llmMock, tester)
app.RunInTest(t)
```

To test with the real LLM service but without calling a live LLM API, mock the [HTTP egress proxy](../structure/coreservices-httpegress.md) to return canned API responses:

```go
httpEgressMock := httpegress.NewMock()
httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
    req, _ := http.ReadRequest(bufio.NewReader(r.Body))
    if strings.Contains(req.URL.String(), "/v1/messages") {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn"}`))
    }
    return nil
})

app := application.New()
app.Add(svc, httpEgressMock, tester)
app.RunInTest(t)
```
