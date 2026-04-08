# Package `coreservices/llm`

The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations. It allows callers to send a prompt along with a list of Microbus endpoint URLs as "tools". The service handles schema discovery from OpenAPI, LLM communication, and tool execution over the bus.

Key capabilities:

- **Provider-agnostic** - supports Claude, OpenAI, and Gemini backends via configuration
- **Automatic tool schema discovery** - fetches endpoint schemas from OpenAPI at call time to build tool definitions in the provider's native format
- **Tool execution over the bus** - invokes Microbus endpoints directly when the LLM requests a tool call, with security context propagated naturally
- **Multi-turn workflow** - the built-in `ChatLoop` workflow orchestrates conversations that exceed a single request's time budget, with support for human-in-the-loop via `flow.Interrupt()`

## Chat

The `Chat` functional endpoint sends messages to an LLM with optional tools and returns the updated conversation. It handles the tool-calling loop internally, up to a configurable number of rounds:

```go
messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
tools := []llmapi.Tool{{URL: "https://calculator.example/arithmetic"}}
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, tools, 0)
```

The output `messagesOut` contains the full conversation including new messages produced by the LLM. Passing `0` for `maxToolRounds` uses the configured default.

## ChatLoop Workflow

For conversations that may require many tool rounds or human interaction, the `ChatLoop` workflow orchestrates the same flow across multiple durable steps:

```go
flowID, _ := foremanapi.NewClient(svc).Create(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "messages": messages,
    "tools":    tools,
})
foremanapi.NewClient(svc).Start(ctx, flowID)
status, state, _ := foremanapi.NewClient(svc).Await(ctx, flowID)
```

The workflow uses `ReducerAppend` for the `messages` state field, so `foremanapi.Continue` appends new messages to the conversation naturally. The `flowID` returned by `Create` doubles as a `threadKey` - pass it (or any flowKey in the thread) to `Continue`:

```go
newFlowID, _ := foremanapi.NewClient(svc).Continue(ctx, flowID, map[string]any{
    "messages": []llmapi.Message{{Role: "user", Content: "Follow-up question"}},
})
```

## Configuration

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `Provider` | string | `claude` | LLM provider: `claude`, `gemini`, or `openai` |
| `BaseURL` | url | `https://api.anthropic.com` | Base URL of the LLM API |
| `APIKey` | secret | | API key for the LLM provider |
| `Model` | string | `claude-haiku-4-5` | Model identifier |
| `MaxToolRounds` | int | `10` | Maximum tool call round-trips per invocation |

The `APIKey` is a secret configuration property and should be set in `config.local.yaml`:

```yaml
llm.core:
  APIKey: sk-ant-your-key-here
```

## Mocking

To mock the LLM microservice in tests:

```go
llmMock := llm.NewMock()
llmMock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool, maxToolRounds int) (messagesOut []llmapi.Message, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, nil
})
```

Note that the `Chat` endpoint listens on internal Microbus [port](../tech/ports.md) `:444` rather than `:443`, and the task and workflow endpoints use port `:428`. These ports are not accessible from outside the bus via the [HTTP ingress proxy](../structure/coreservices-httpingress.md).
