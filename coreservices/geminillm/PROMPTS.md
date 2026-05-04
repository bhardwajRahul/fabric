## Gemini LLM Provider Microservice

Create a core microservice at hostname `gemini.llm.core` that implements the `Turn` endpoint for the Google Gemini `generateContent` API. Use the HTTP egress proxy for all outbound requests. Import `Message`, `Tool`, `ToolCall`, and `TurnCompletion` types from `llmapi` to ensure a uniform interface.

Expose one endpoint:

- `Turn(messages []Message, tools []Tool) (completion *TurnCompletion)` on `POST :444/turn`

### Turn Logic

1. Convert `[]llmapi.Message` to `[]geminiContent`:
   - `system` messages: map to role `"user"` with a single text part (Gemini has no top-level system field).
   - `assistant` messages with `ToolCalls` JSON: map to role `"model"` with a text part (if non-empty) followed by one `functionCall` part per tool call (name + args as `map[string]any`).
   - `assistant` messages without tool calls: map to role `"model"` with a text part.
   - `tool` messages: map to role `"user"` with a single `functionResponse` part, carrying the tool name (from `ToolCallID`) and the result as `map[string]any` (attempt JSON unmarshal; fall back to `{"result": content}`).
   - Default: pass role through with a text part.
2. Convert `[]llmapi.Tool` to a `[]geminiToolDec` with a single entry containing all function declarations (name, description, parameters as `json.RawMessage`).
3. POST to `svc.BaseURL() + "/v1beta/models/" + svc.Model() + ":generateContent?key=" + svc.APIKey()`.
4. On non-200 status, return an error with the status code and response body.
5. Parse the first candidate's content parts: accumulate `text` parts, convert `functionCall` parts to `llmapi.ToolCall` (using the function name as both `ID` and `Name`).

### Config Properties

- `BaseURL` — Gemini API base URL, default `https://generativelanguage.googleapis.com`, validated as URL.
- `APIKey` — Google API key, `secret: true`.
- `Model` — Gemini model identifier, default `gemini-2.0-flash`.
