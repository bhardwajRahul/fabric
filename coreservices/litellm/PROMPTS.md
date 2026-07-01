## LiteLLM Provider Microservice

Create a core microservice at hostname `lite.llm.core` that implements the `Turn` endpoint for a LiteLLM
proxy. LiteLLM speaks the OpenAI Responses wire format (`/v1/responses`), so model this provider on the
existing `chatgptllm` provider: reuse the same OpenAI-shaped request/response structs and translation logic,
the same `Authorization: Bearer <APIKey>` header, and the HTTP egress proxy for all outbound requests. Import
`Message`, `Tool`, `ToolCall`, `Usage`, and `TurnOptions` from `llmapi` for a uniform cross-provider
interface. No changes to `llm.core` are required - it routes to providers by hostname.

Expose one endpoint:

- `Turn(model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage)` on `POST :444/turn`. The output tuple must match the `llm.core` `Turn` contract exactly (including `stopReason`), since `llm.core` dispatches to providers through the shared `llmapi` client.

### Turn Logic

Identical to the ChatGPT provider (OpenAI Responses schema):

1. Convert `[]llmapi.Message` to the Responses `input` array: `system` folds into `instructions`, `user`/
   `assistant` become `message` items (`input_text`/`output_text` content parts), assistant `ToolCalls` JSON
   expands into `function_call` items, and `tool` messages become `function_call_output` items keyed by
   `call_id`.
2. Convert `[]llmapi.Tool` to the flat Responses `function` tool shape (parameters as `json.RawMessage`).
3. Always send `num_retries: 0` to disable LiteLLM's internal retries. Map `options.MaxTokens` to
   `max_output_tokens`.
4. POST the request to `svc.ResponsesURL()` with `Authorization: Bearer <APIKey>`.
5. On non-200, return an error with the status code and body; attach `retryAfter` on any `429`.
6. Walk the response `output` array (join `output_text` parts, collect `function_call` items); derive
   `stopReason` from `status` / `incomplete_details.reason` / presence of tool calls; map the Responses
   `usage` block into `llmapi.Usage`.

### Config Properties

- `ResponsesURL` - LiteLLM proxy responses endpoint URL, default
  `http://localhost:4000/v1/responses` (LiteLLM's documented default proxy address; no canonical
  public endpoint exists since the proxy is self-hosted), validated as URL.
- `APIKey` - LiteLLM proxy virtual key, `secret: true`.

No typed model constants are shipped: valid model strings depend entirely on the operator's LiteLLM
`model_list`, so `model` is a passthrough string.
