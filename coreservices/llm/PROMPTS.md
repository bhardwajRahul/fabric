## LLM Core Service

Create a core microservice at hostname `llm.core` that bridges LLM tool-calling with Microbus endpoint invocations. It is provider-agnostic: each call names the provider microservice (e.g. `claude.llm.core`) and the model, and `llm.core` delegates the actual LLM call to that provider's `Turn` endpoint.

### Config Properties

- `MaxToolRounds` - maximum number of tool-call round-trips per invocation, default `10`, range `[1, 50]`. A caller can override per call via `ChatOptions.MaxToolRounds`.

### Functional Endpoints (both on `:444`)

**`Turn(model string, items []Item, tools []Tool) (itemsOut []Item, stopReason string, usage Usage)`** - a single LLM turn. On `llm.core` this is a stub returning 501; the real implementation lives in each provider microservice (`claudellm`, `chatgptllm`, `geminillm`, `litellm`). The conversation is an ordered `[]Item` (message / tool_call / tool_result / reasoning); `Turn` returns only the assistant turn's new items.

**`Chat(provider, model string, items []Item, toolURLs []string, options *ChatOptions) (itemsOut []Item, usage Usage)`** - the full multi-turn chat loop with tool execution. `toolURLs` are canonical Microbus endpoint URLs (e.g. `calculatorapi.Arithmetic.URL()`), resolved to `[]llmapi.Tool` via `fetchTools`. It iterates up to `MaxToolRounds` turns; after each turn that returns tool calls, it executes each via `executeTool` and appends `tool_result` items. When no tool calls remain it returns; after exhausting `MaxToolRounds` it makes one final call without tools to force a text answer. `itemsOut` is the full conversation - the input `items` plus every item the LLM produced - so a caller can resume by re-invoking with it.

### Tool Resolution (`fetchTools`)

Group tool URLs by `host:port` (default port `443`). Fetch each host's OpenAPI document in parallel via `controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)` (the `:888/openapi.json` endpoint). For each requested URL, canonicalize path arguments (strip `...` from greedy args, name anonymous args `path1`, `path2`, ...) to match the OpenAPI document's path keys, then resolve the operation. Only `FeatureFunction`, `FeatureWeb`, and `FeatureWorkflow` operations are accepted. Disambiguate name collisions with `_2`, `_3`, ... suffixes in argument order.

### Tool Execution (`executeTool`)

Invoke the endpoint over the bus using `svc.Request` with the tool's URL, method, and JSON arguments as the body. On transport error, return the error as a JSON tool result rather than failing the whole chat. On HTTP 4xx/5xx, return the response body as a JSON error result.

### ChatLoop Workflow (on `:428`)

The `ChatLoop` workflow orchestrates multi-turn LLM conversations as durable workflow steps:

```
InitChat -> FirstLLM -> ProcessResponse
                             |- (no tool calls, or round limit) -> END
                             |- forEach(pendingToolCalls, currentTool) -> ExecuteTool -> NextLLM -> ProcessResponse
```

`FirstLLM` and `NextLLM` are two graph positions dispatching to the same `CallLLM` task (the split lets `NextLLM` be the fan-in nexus). A `ReducerAppend` reducer on `items` merges the parallel `ExecuteTool` branches' `tool_result` deltas at the fan-in.

**Task endpoints (all on `:428`):**

- `InitChat(items []Item, toolURLs []string, options *ChatOptions)` - a pure setup step with no outputs; resolves `toolURLs` to tool schemas via each host's OpenAPI document and seeds the ambient flow state the rest of the loop reads - `toolSchemas`, `turnOptions`, `maxToolRounds`, and the `toolRounds` counter (0) - all via `flow.Set`.
- `CallLLM(items []Item) (turnItems []Item, pendingToolCalls any, turnUsage Usage)` - reads `toolSchemas`/`turnOptions` ambiently from flow state, calls the provider, and returns the assistant turn's items, the pending tool calls, and the turn's usage.
- `ProcessResponse(turnItems []Item, pendingToolCalls []ToolCall, turnUsage Usage, toolRounds int) (toolsRequested bool, toolRoundsOut int, usageOut Usage)` - reads `maxToolRounds` ambiently and appends `turnItems` to the `items` state key (append reducer). If no pending calls or the round limit is reached, sets `toolsRequested=false` and `flow.Goto(END)`. Otherwise increments `toolRoundsOut` and sets `toolsRequested=true`, signaling the `forEach` fan-out over `pendingToolCalls`.
- `ExecuteTool(currentTool ToolCall) (items []Item)` - executes the single tool call named by the `currentTool` forEach variable. Workflow tools run as dynamic subgraphs via `flow.Subgraph` (parks on the first call, returns the child's `final_state` on re-entry); other tools run via a direct bus call. Returns the `tool_result` item, which the append reducer merges into the `items` state key.
