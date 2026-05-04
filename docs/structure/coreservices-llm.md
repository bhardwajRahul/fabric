# Package `coreservices/llm`

The LLM microservice bridges LLM tool-calling protocols with Microbus endpoint invocations. Callers pass a conversation, a provider, a model, and a list of endpoint URLs they want to expose as tools. The service drives the tool-calling loop, dispatches tool calls over the bus, and returns the completed conversation along with token usage.

Key capabilities:

- **Caller-selected provider and model** - every `Chat` call specifies the provider hostname (e.g. `claude.llm.core`) and model identifier explicitly. There is no `ProviderHostname` config and providers do not carry a `Model` config. Hiding the model behind operator config is dangerous - Opus is roughly 100x the cost of Haiku - so the model is always visible at the call site.
- **OpenAPI-derived tool schemas** - callers identify tools by their canonical Microbus URL. At chat time the LLM service fetches each host's `:888/openapi.json` document in parallel (the connector's built-in handler) and reflects the matching operation's request-body schema into a JSON Schema for the LLM. Authorization flows through automatically: the OpenAPI handler filters by the caller's actor claims, so the LLM only sees tools the actor is authorized to invoke.
- **Tool execution over the bus** - invokes Microbus endpoints directly when the LLM requests a tool call, with security context propagated through the request frame.
- **Token usage tracking** - each turn returns an `llmapi.Usage` carrying input, output, cache-read and cache-write tokens plus the resolved model identifier. `Chat` aggregates per-turn usage and reports the totals. The `microbus_llm_tokens_total` counter (labeled by `provider`, `model`, `direction`) feeds the LLM Grafana dashboard.
- **Anthropic prompt caching** - the `claudellm` provider sets two `cache_control` breakpoints on requests so Anthropic's prompt cache can be reused across turns. Cached input is reflected in `Usage.CacheReadTokens`/`CacheWriteTokens`.
- **Multi-turn workflow** - the built-in `ChatLoop` workflow orchestrates conversations that exceed a single request's time budget, with durability, human-in-the-loop support via `flow.Interrupt()`, and natural continuation via `foremanapi.Continue`.

## Chat

The `Chat` functional endpoint sends messages to an LLM with optional tools and returns the updated conversation along with aggregated token usage. It handles the tool-calling loop internally, up to `MaxToolRounds` rounds:

```go
import (
    "github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
    "github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
toolURLs := []string{calculatorapi.Arithmetic.URL()}
messagesOut, usage, err := llmapi.NewClient(svc).Chat(
    ctx,
    claudellmapi.Hostname,        // provider
    claudellmapi.ModelHaiku45,    // model
    messages,
    toolURLs,
    nil,                          // *llmapi.ChatOptions, optional
)
```

`provider` is the hostname of a provider microservice (`claude.llm.core`, `chatgpt.llm.core`, `gemini.llm.core`, or any custom provider that implements the `Turn` contract). Each provider's `*api` package exports typed model constants such as `claudellmapi.ModelHaiku45`, `claudellmapi.ModelSonnet46`, `claudellmapi.ModelOpus47` so a typo is a compile error rather than a runtime failure.

Each entry in `toolURLs` is the URL of a downstream microservice's endpoint. Only `function`, `web`, and `workflow` endpoints can be exposed as tools - tasks and outbound events are silently skipped by the connector's OpenAPI handler. When two endpoints share the same operation name, the first keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order.

`messagesOut` contains the full conversation including new messages produced by the LLM. `usage` is the aggregated `llmapi.Usage` across all turns.

### ChatOptions

Pass `*llmapi.ChatOptions` to override defaults for a single call:

```go
opts := &llmapi.ChatOptions{
    MaxToolRounds: 5,        // overrides the MaxToolRounds config for this call
    MaxTokens:     1024,     // caps response length per turn
    Temperature:   0.2,      // sampling randomness
}
```

`MaxToolRounds` remains as a service-level config (operational guardrail); `ChatOptions.MaxToolRounds` is an optional per-call override.

## ChatLoop Workflow

For conversations that may require many tool rounds or human interaction, the `ChatLoop` workflow orchestrates the same flow across multiple durable steps. Inputs match `Chat` (`provider`, `model`, `messages`, `tools`, `options`); outputs are `messages` and `usage`:

```go
flowID, _ := foremanapi.NewClient(svc).Create(ctx, llmapi.ChatLoop.URL(), map[string]any{
    "provider": claudellmapi.Hostname,
    "model":    claudellmapi.ModelHaiku45,
    "messages": messages,
    "tools":    tools,
})
foremanapi.NewClient(svc).Start(ctx, flowID)
status, state, _ := foremanapi.NewClient(svc).Await(ctx, flowID)
```

The workflow uses `ReducerAppend` for the `messages` state field, so `foremanapi.Continue` appends new messages to the conversation naturally. `ProcessResponse` accumulates per-turn usage into the `usage` state field. The `flowID` returned by `Create` doubles as a `threadKey` - pass it (or any flowKey in the thread) to `Continue`:

```go
newFlowID, _ := foremanapi.NewClient(svc).Continue(ctx, flowID, map[string]any{
    "messages": []llmapi.Message{{Role: "user", Content: "Follow-up question"}},
})
```

## Configuration

The `llm.core` service holds only one config. Provider-specific settings (`BaseURL`, `APIKey`) live on the provider microservice.

### `llm.core`

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `MaxToolRounds` | int | `10` | Maximum tool call round-trips per `Chat` invocation. May be overridden per call via `ChatOptions.MaxToolRounds`. |

### Provider service (`claudellm`, `chatgptllm`, `geminillm`)

| Config | Type | Default (Claude) | Description |
|--------|------|---------|-------------|
| `BaseURL` | url | `https://api.anthropic.com` | Base URL of the provider's API. |
| `APIKey` | secret | | API key for the provider. |

The `APIKey` is a secret and should be set in `config.local.yaml`, scoped by the provider's hostname:

```yaml
claude.llm.core:
  APIKey: sk-ant-your-key-here
chatgpt.llm.core:
  APIKey: sk-your-openai-key
gemini.llm.core:
  APIKey: your-gemini-key
```

To use a different provider for a call, simply pass that provider's hostname and model. There is no global "active provider" config to flip.

## Mocking

To mock the `Chat` endpoint of `llm.core`:

```go
llmMock := llm.NewMock()
llmMock.MockChat(func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) {
    return []llmapi.Message{{Role: "assistant", Content: "Mocked response"}}, llmapi.Usage{Turns: 1}, nil
})
```

To exercise the real `llm.core` against a mocked provider, mock the provider's `Turn` instead:

```go
claudeMock := claudellm.NewMock()
claudeMock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
    return "Hello from mock!", nil, llmapi.Usage{Model: model, Turns: 1}, nil
})
```

Mocking at the provider boundary exercises the full tool-calling loop, schema resolution, and bus dispatch in `llm.core` while keeping the test offline.

## `Turn` on `llm.core`

The `Turn` endpoint is part of the contract that provider microservices implement. `llm.core` itself is not a provider - calling its `Turn` endpoint returns 501. Use `llmapi.NewClient(svc).ForHost(<providerHost>).Turn(...)` to invoke a specific provider directly, or use `Chat` for the conversation loop.

The endpoint stub is registered (rather than removed) because `llmapi.Turn.URL()` is referenced as the canonical form of the contract.

The `Chat` endpoint listens on internal Microbus [port](../tech/ports.md) `:444` rather than `:443`, and the task and workflow endpoints use port `:428`. These ports are not accessible from outside the bus via the [HTTP ingress proxy](../structure/coreservices-httpingress.md).
