## ChatGPT LLM Provider Microservice

Create a core microservice at hostname `chatgpt.llm.core` that implements the `Turn` endpoint for the OpenAI Chat Completions API. Use the HTTP egress proxy for all outbound requests. Import `Message`, `Tool`, `ToolCall`, and `TurnCompletion` types from `llmapi` to ensure a uniform interface.

Expose one endpoint:

- `Turn(messages []Message, tools []Tool) (completion *TurnCompletion)` on `POST :444/turn`

### Turn Logic

1. Convert `[]llmapi.Message` to `[]openaiMessage`:
   - Pass `role` and `content` through directly (OpenAI and Microbus share the same role names: `system`, `user`, `assistant`, `tool`).
   - `assistant` messages with `ToolCalls` JSON: unmarshal and populate `openaiToolCall` entries with `id`, `type: "function"`, and `function: {name, arguments}`. Note that OpenAI tool call arguments are a JSON-encoded string, not raw JSON.
   - `tool` messages: set `tool_call_id` from `msg.ToolCallID`.
2. Convert `[]llmapi.Tool` to `[]openaiTool` with `type: "function"` and a nested `function` object (name, description, parameters as `json.RawMessage`).
3. POST to `svc.BaseURL() + "/v1/chat/completions"` with `Authorization: Bearer <APIKey>`.
4. On non-200 status, return an error with the status code and body.
5. Parse the first choice's message: set `completion.Content` and convert `tool_calls` to `[]llmapi.ToolCall` (OpenAI tool call arguments are a string, decode as `json.RawMessage`).

### Config Properties

- `BaseURL` — OpenAI API base URL, default `https://api.openai.com`, validated as URL.
- `APIKey` — OpenAI API key, `secret: true`.
- `Model` — OpenAI model identifier, default `gpt-4`.
