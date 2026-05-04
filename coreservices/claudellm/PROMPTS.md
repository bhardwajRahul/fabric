## Claude LLM Provider Microservice

Create a core microservice at hostname `claude.llm.core` that implements the `Turn` endpoint for the Anthropic Claude Messages API. Use the HTTP egress proxy for all outbound requests. Import `Message`, `Tool`, `ToolCall`, and `TurnCompletion` types from `llmapi` to ensure a uniform interface.

Expose one endpoint:

- `Turn(messages []Message, tools []Tool) (completion *TurnCompletion)` on `POST :444/turn`

### Turn Logic

1. Extract the `system` role message (if any) into a separate `systemMsg` string; Claude's API takes the system prompt as a top-level field.
2. Convert `[]llmapi.Message` to `[]claudeMessage`:
   - `system` messages: skip (already extracted).
   - `assistant` messages with `ToolCalls` JSON: unmarshal the tool calls and emit a `content` array of blocks — a `text` block if there is content, plus one `tool_use` block per tool call (carrying `id`, `name`, and `input` as `json.RawMessage`).
   - `assistant` messages without tool calls: emit a simple JSON-encoded string content.
   - `tool` messages: emit a `user` message with a single `tool_result` block, carrying `tool_use_id` and `content`.
   - All other roles: emit as-is with JSON-encoded string content.
3. Convert `[]llmapi.Tool` to `[]claudeTool` (name, description, input_schema as `json.RawMessage`).
4. Build a `claudeRequest` with `model`, `max_tokens: 4096`, `messages`, `tools`, and `system`.
5. POST to `svc.BaseURL() + "/v1/messages"` via `httpegressapi.NewClient(svc).Do(ctx, req)` with headers `Content-Type: application/json`, `x-api-key`, and `anthropic-version: 2023-06-01`.
6. On non-200 status, attempt to parse the error envelope (`error.type`, `error.message`, `request_id`) and return a structured error with the HTTP status code attached.
7. Parse the `claudeResponse` content blocks: accumulate `text` blocks into `completion.Content`, convert `tool_use` blocks to `llmapi.ToolCall` entries.

### Config Properties

- `BaseURL` — Claude API base URL, default `https://api.anthropic.com`, validated as URL.
- `APIKey` — Anthropic API key, `secret: true`.
- `Model` — Claude model identifier, default `claude-haiku-4-5`.
