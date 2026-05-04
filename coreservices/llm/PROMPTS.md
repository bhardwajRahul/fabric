## LLM Core Service

Create a core microservice at hostname `llm.core` that bridges LLM tool-calling with Microbus endpoint invocations. The service is provider-agnostic: it delegates all LLM calls to a configurable provider microservice via the `Turn` endpoint.

### Config Properties

- `ProviderHostname` — hostname of the LLM provider to delegate to, default `claude.llm.core`.
- `MaxToolRounds` — maximum number of tool-call round-trips per invocation, default `10`, range `[1, 50]`.

### Functional Endpoints (both on `:444`)

**`Turn(messages []Message, tools []Tool) (completion *TurnCompletion)`** — single LLM turn, delegates to `llmapi.NewClient(svc).ForHost(svc.ProviderHostname()).Turn(ctx, messages, tools)`. This allows `llm.core` itself to be the configured provider for upstream callers, while still routing to the actual provider.

**`Chat(messages []Message, tools []string) (messagesOut []Message)`** — full multi-turn chat loop with tool execution. Takes `[]string` of canonical Microbus endpoint URLs (e.g. `calculatorapi.Arithmetic.URL()`). Resolves URLs to `[]llmapi.Tool` via `fetchTools`, then iterates up to `MaxToolRounds` turns. After each turn, if the LLM returns tool calls, execute each one via `executeTool` and append tool result messages. When no tool calls remain, return accumulated messages. After exhausting `MaxToolRounds`, make one final call without tools to force a text response.

### Tool Resolution (`fetchTools`)

Group tool URLs by `host:port` (default port `443`). Fetch each host's OpenAPI document in parallel via `controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)` (the `:888/openapi.json` endpoint). For each requested URL, canonicalize path arguments (strip `...` from greedy args, name anonymous args `path1`, `path2`, ...) to match the OpenAPI document's path keys, then resolve the operation. Only `FeatureFunction`, `FeatureWeb`, and `FeatureWorkflow` operations are accepted. Disambiguate name collisions with `_2`, `_3`, ... suffixes in argument order.

### Tool Execution (`executeTool`)

Invoke the endpoint over the bus using `svc.Request` with the tool's URL, method, and JSON arguments as the body. On transport error, return the error as a JSON tool result rather than failing the whole chat. On HTTP 4xx/5xx, return the response body as a JSON error result.

### ChatLoop Workflow (on `:428`)

The `ChatLoop` workflow orchestrates multi-turn LLM conversations as durable workflow steps:

```
InitChat → CallLLM → ProcessResponse
                          ↓ (!toolsRequested) → END
                          ↓ forEach(pendingToolCalls, currentTool) → ExecuteTool → CallLLM
```

A `ReducerAppend` reducer on `messages` merges parallel `ExecuteTool` branches.

**Task endpoints (all on `:428`):**

- `InitChat(messages []Message, tools []Tool) (maxToolRounds int, toolRounds int)` — stores `tools` in flow state as `toolSchemas`; returns `MaxToolRounds` and `0`.
- `CallLLM(messages []Message) (llmContent string, pendingToolCalls any)` — reads `toolSchemas` from flow state, calls the provider, returns content and tool calls.
- `ProcessResponse(llmContent string, toolRounds int, maxToolRounds int) (messagesOut []Message, toolsRequested bool, toolRoundsOut int)` — reads `pendingToolCalls` and `messagesOut` from flow state. If no pending calls or round limit reached, sets `toolsRequested=false` and returns. Otherwise increments `toolRoundsOut` and sets `toolsRequested=true`, signaling the `forEach` fan-out.
- `ExecuteTool(toolExecuted bool) (toolExecutedOut bool)` — uses the `Out` suffix pattern for subgraph re-entry detection. On first run (`toolExecuted=false`): if `Tool.Type == "workflow"`, snapshot current state keys into `preSubgraphKeys`, inject input state, and call `flow.Subgraph(def.URL, inputState)` for dynamic subgraph execution. Otherwise execute via direct bus call and append a `tool` message. On re-entry (`toolExecuted=true`): diff state keys against `preSubgraphKeys` snapshot to find child output fields, marshal them as the tool result.
