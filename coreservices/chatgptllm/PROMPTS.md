## ChatGPT LLM Provider Microservice

Create a core microservice at hostname `chatgpt.llm.core` that implements the `Turn` endpoint for the OpenAI Responses API (`/v1/responses`). Use the HTTP egress proxy for all outbound requests. Import `Message`, `Tool`, `ToolCall`, `Usage`, and `TurnOptions` types from `llmapi` to ensure a uniform interface.

Expose one endpoint:

- `Turn(model string, messages []Message, tools []Tool, options *TurnOptions) (content string, toolCalls []ToolCall, stopReason string, usage Usage)` on `POST :444/turn`

### Turn Logic

1. Convert `[]llmapi.Message` to the Responses `input` array of typed items:
   - `system` messages fold into the top-level `instructions` string (concatenated).
   - `user` messages become `message` items with a `content` part of type `input_text`.
   - `assistant` messages: text becomes a `message` item with an `output_text` content part; each entry in the `ToolCalls` JSON becomes a `function_call` item with `call_id`, `name`, and `arguments` (arguments is a JSON-encoded string).
   - `tool` messages become `function_call_output` items with `call_id` from `msg.ToolCallID` and `output` from the content.
2. Convert `[]llmapi.Tool` to the flat Responses tool shape: `type: "function"` with `name`, `description`, and `parameters` (as `json.RawMessage`) directly on the object (no nested `function` wrapper).
3. Map `options.MaxTokens` to `max_output_tokens`.
4. POST to `svc.ResponsesURL()` with `Authorization: Bearer <APIKey>`.
5. On non-200 status, parse OpenAI's `{error:{message,type,code}}` envelope and attach `retryAfter` only on a genuinely transient `429 rate_limit_exceeded` (see the error-classification and rate-limit-preemption notes in `CLAUDE.md`).
6. Parse the response `output` array: join `output_text` parts of `message` items into `content`, and read each `function_call` item into `[]llmapi.ToolCall` (its `call_id` is the `ID`). Derive the normalized `stopReason` from `status`, `incomplete_details.reason`, and whether any tool calls were emitted. Populate `usage` from the `usage` block (`input_tokens`, `output_tokens`, `input_tokens_details.cached_tokens`, `output_tokens_details.reasoning_tokens`).

### Config Properties

- `ResponsesURL` - OpenAI responses endpoint URL, default `https://api.openai.com/v1/responses`, validated as URL.
- `APIKey` - OpenAI API key, `secret: true`.

The `model` is a required per-call argument, not a config property.
