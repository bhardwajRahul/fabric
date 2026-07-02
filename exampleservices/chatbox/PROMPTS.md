## Create Chatbox Demo LLM Provider

Create an example microservice at hostname `chatbox.example` that implements a demo LLM provider. It pattern-matches math questions to generate tool calls against the calculator service, demonstrating the full LLM tool-calling flow without requiring a real API key.

## Types

All types (`Item`, `Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` (package `github.com/microbus-io/fabric/coreservices/llm/llmapi`). Do not define local types. The conversation is an ordered `[]llmapi.Item`; build it with an `[]llmapi.Item` literal, wrapping each part via its `AsItem()` method (e.g. `llmapi.NewMessage("user", text).AsItem()`, `llmapi.ToolCall{...}.AsItem()`).

## Turn Endpoint

- `Turn` on `POST :444/turn` - signature `Turn(model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (outItems []llmapi.Item, stopReason string, usage llmapi.Usage, err error)`. This is the LLM provider interface endpoint, the same contract the real provider microservices implement. `outItems` is only the assistant turn's new items.

Logic:
1. If `items` is empty, return a welcome message item with `stopReason = llmapi.StopReasonEndTurn`.
2. Inspect `last := items[len(items)-1]` and branch on `last.Type()`.
3. If `last.Type() == llmapi.ItemToolResult`: parse the tool result JSON (`last.ToolResult.Output`), extract the `"result"` field, and return `"The answer is <result>."` (else `"The result is: <output>"`), as an assistant message with `StopReasonEndTurn`.
4. If `last.Type() == llmapi.ItemMessage` with role `user`: run `mathPattern` against `last.Message.Content`. `mathPattern` is a compiled regex:
   ```
   (?i)(?:what is|how much is|calculate|compute|what's|whats)\s+(\d+)\s*([\+\-\*\/x]|plus|minus|times|multiplied by|divided by|over)\s*(\d+)
   ```
   Extract groups: `x int`, `opStr string`, `y int`. Normalize `opStr` via a static `opMap` that maps English operator names and `x` to symbols (`+`, `-`, `*`, `/`).
   - If a tool whose name contains `"arithmetic"` or `"calculator"` (case-insensitive) is found in `tools`: return an assistant message plus a `ToolCall` item (ID `"chatbox_1"`, Name from the tool, Arguments JSON `{"x": x, "op": op, "y": y}`) with `stopReason = llmapi.StopReasonToolUse`.
   - If no such tool: compute the answer directly (`switch op { case "+": ... }`) and return the result as a message. Handle division by zero.
   - If pattern does not match: return an explanation that only math questions are understood.
5. Default: return `"I don't understand that message."`

## Demo Endpoint

- `Demo` on `ANY //chatbox.example/demo` (absolute route mapped to ingress root) - serves an interactive chat UI from `resources/demo.html`.

On GET: render the template with no data.
On POST: read the `message` form value (return `400` if empty), and read `provider` (default `chatbox.example`, i.e. `Hostname`) and `model` (default `chatbox-default`). Build a single-item conversation `[]llmapi.Item{llmapi.NewMessage("user", userMessage).AsItem()}`, call `llmapi.NewClient(svc).Chat(r.Context(), provider, model, items, tools, nil)` with `tools = []string{calculatorapi.Arithmetic.URL()}`, and return the resulting conversation as JSON (`Content-Type: application/json`).

## Non-obvious Details

- The `Turn` endpoint is the provider interface; it is called by `llm.core` during its tool-calling orchestration loop. The chatbox must never call `llm.core` from within `Turn`.
- The `Demo` endpoint calls `llmapi.NewClient(svc).Chat(...)` (the LLM core service), which internally delegates back to this chatbox's `Turn` endpoint. This round-trip is the intended demonstration.
- The `opMap` must handle both symbol aliases (`"x"` → `"*"`) and full English phrases (`"multiplied by"` → `"*"`, `"divided by"` → `"/"`, `"over"` → `"/"`).
- The provider is selected per call, not by config: pass `chatbox.example` as the `provider` argument to `Chat` (the `Demo` endpoint reads it from the `provider` form value, defaulting to `chatbox.example`). There is no `ProviderHostname` config on `llm.core`.
