# Package `coreservices/llm`

The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations. Callers pass a conversation and a list of endpoint URLs they want to expose as tools. The service drives the tool-calling loop, dispatches tool calls over the bus, and returns the completed conversation.

Key capabilities:

- **Provider-agnostic** - delegates to a provider microservice (`claudellm`, `chatgptllm`, or `geminillm`) selected by the `ProviderHostname` config. Swapping providers requires no code changes.
- **OpenAPI-derived tool schemas** - callers identify tools by their canonical Microbus URL. At chat time the LLM service fetches each host's `:888/openapi.json` document in parallel (the connector's built-in handler) and reflects the matching operation's request-body schema into a JSON Schema for the LLM. Authorization flows through automatically: the OpenAPI handler filters by the caller's actor claims, so the LLM only sees tools the actor is authorized to invoke.
- **Tool execution over the bus** - invokes Microbus endpoints directly when the LLM requests a tool call, with security context propagated through the request frame.
- **Multi-turn workflow** - the built-in `ChatLoop` workflow orchestrates conversations that exceed a single request's time budget, with durability, human-in-the-loop support via `flow.Interrupt()`, and natural continuation via `foremanapi.Continue`.

## Chat

The `Chat` functional endpoint sends messages to an LLM with optional tools and returns the updated conversation. It handles the tool-calling loop internally, up to `MaxToolRounds` rounds:

```go
messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
tools := []string{calculatorapi.Arithmetic.URL()}
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, tools)
```

Each entry in `tools` is the URL of a downstream microservice's endpoint. Only `function`, `web`, and `workflow` endpoints can be exposed as tools - tasks and outbound events are silently skipped by the connector's OpenAPI handler. When two endpoints share the same operation name, the first keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order.

The output `messagesOut` contains the full conversation including new messages produced by the LLM.

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

The `llm.core` service holds only orchestration settings. Provider-specific settings (`BaseURL`, `APIKey`, `Model`) live on the provider microservice.

### `llm.core`

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `ProviderHostname` | string | `claude.llm.core` | Hostname of the provider microservice that implements the `Turn` endpoint. Set to `chatgpt.llm.core` or `gemini.llm.core` to switch providers. |
| `MaxToolRounds` | int | `10` | Maximum tool call round-trips per `Chat` invocation. |

### Provider service (`claudellm`, `chatgptllm`, `geminillm`)

| Config | Type | Default (Claude) | Description |
|--------|------|---------|-------------|
| `BaseURL` | url | `https://api.anthropic.com` | Base URL of the provider's API. |
| `APIKey` | secret | | API key for the provider. |
| `Model` | string | `claude-haiku-4-5` | Model identifier. |

The `APIKey` is a secret and should be set in `config.local.yaml`, scoped by the provider's hostname:

```yaml
claude.llm.core:
  APIKey: sk-ant-your-key-here
```

To point `llm.core` at a different provider:

```yaml
llm.core:
  ProviderHostname: chatgpt.llm.core

chatgpt.llm.core:
  Model: gpt-4
```

## Mocking

To mock the LLM microservice in tests:

```go
llmMock := llm.NewMock()
llmMock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []string) (messagesOut []llmapi.Message, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, nil
})
```

Note that the `Chat` endpoint listens on internal Microbus [port](../tech/ports.md) `:444` rather than `:443`, and the task and workflow endpoints use port `:428`. These ports are not accessible from outside the bus via the [HTTP ingress proxy](../structure/coreservices-httpingress.md).
