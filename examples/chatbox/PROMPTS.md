## Create Chatbox Demo LLM Provider

Create an example microservice at hostname `chatbox.example` that implements a demo LLM provider. It pattern-matches math questions to generate tool calls against the calculator service, demonstrating the full LLM tool-calling flow without requiring a real API key.

## Types

All types (`Message`, `Tool`, `ToolCall`, `TurnCompletion`) are imported from `llmapi` (package `github.com/microbus-io/fabric/coreservices/llm/llmapi`). Do not define local types.

## Turn Endpoint

- `Turn` on `POST :444/turn` — signature `Turn(messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion)`. This is the LLM provider interface endpoint.

Logic:
1. If `messages` is empty, return a welcome message.
2. Inspect `lastMsg := messages[len(messages)-1]`.
3. If `lastMsg.Role == "tool"`: parse the tool result JSON, extract the `"result"` field, and return `"The answer is <result>."`. If no result field, return `"The result is: <content>"`.
4. If `lastMsg.Role == "user"`: run `mathPattern` against the content. `mathPattern` is a compiled regex:
   ```
   (?i)(?:what is|how much is|calculate|compute|what's|whats)\s+(\d+)\s*([\+\-\*\/x]|plus|minus|times|multiplied by|divided by|over)\s*(\d+)
   ```
   Extract groups: `x int`, `opStr string`, `y int`. Normalize `opStr` via a static `opMap` that maps English operator names and `x` to symbols (`+`, `-`, `*`, `/`).
   - If a tool whose name contains `"arithmetic"` or `"calculator"` (case-insensitive) is found in `tools`: return a `TurnCompletion` with a `ToolCall` (ID `"chatbox_1"`, Name from the tool, Arguments JSON `{"x": x, "op": op, "y": y}`).
   - If no such tool: compute the answer directly (`switch op { case "+": ... }`) and return the result as a string. Handle division by zero.
   - If pattern does not match: return an explanation that only math questions are understood.
5. Default: return `"I don't understand that message."`

## Demo Endpoint

- `Demo` on `ANY //chatbox.example/demo` (absolute route mapped to ingress root) — serves an interactive chat UI from `resources/demo.html`.

On GET: render the template with no data.
On POST: read `message` form value (return `400` if empty), build a single-message conversation `[]llmapi.Message{{Role: "user", Content: userMessage}}`, call `llmapi.NewClient(svc).Chat(r.Context(), messages, tools)` with `tools = []string{calculatorapi.Arithmetic.URL()}`, and return the resulting conversation as JSON (`Content-Type: application/json`).

## Non-obvious Details

- The `Turn` endpoint is the provider interface; it is called by `llm.core` during its tool-calling orchestration loop. The chatbox must never call `llm.core` from within `Turn`.
- The `Demo` endpoint calls `llmapi.NewClient(svc).Chat(...)` (the LLM core service), which internally delegates back to this chatbox's `Turn` endpoint. This round-trip is the intended demonstration.
- The `opMap` must handle both symbol aliases (`"x"` → `"*"`) and full English phrases (`"multiplied by"` → `"*"`, `"divided by"` → `"/"`, `"over"` → `"/"`).
- To configure the LLM core service to use this provider, set `ProviderHostname: chatbox.example` in `config.yaml` for `llm.core`.
