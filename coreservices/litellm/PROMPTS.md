## LiteLLM Provider Microservice

Create a core microservice at hostname `lite.llm.core` that implements the `Turn` endpoint for a LiteLLM
proxy. LiteLLM speaks the OpenAI Chat Completions wire format, so model this provider on the existing
`chatgptllm` provider: reuse the same OpenAI-shaped request/response structs and translation logic, the
same `Authorization: Bearer <APIKey>` header, and the HTTP egress proxy for all outbound requests. Import
`Message`, `Tool`, `ToolCall`, `Usage`, and `TurnOptions` from `llmapi` for a uniform cross-provider
interface. No changes to `llm.core` are required - it routes to providers by hostname.

Expose one endpoint:

- `Turn(model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage)` on `POST :444/turn`

### Turn Logic

Identical to the ChatGPT provider (OpenAI Chat Completions schema):

1. Convert `[]llmapi.Message` to OpenAI messages, passing `role`/`content` through, expanding assistant
   `ToolCalls` JSON into `tool_calls`, and setting `tool_call_id` on `tool` messages.
2. Convert `[]llmapi.Tool` to OpenAI `function` tools (parameters as `json.RawMessage`).
3. POST the request to `svc.CompletionURL()` with `Authorization: Bearer <APIKey>`.
4. On non-200, return an error with the status code and body.
5. Parse the first choice; map the OpenAI `usage` block into `llmapi.Usage`.

### Config Properties

- `CompletionURL` - LiteLLM proxy chat completions endpoint URL, default
  `http://localhost:4000/v1/chat/completions` (LiteLLM's documented default proxy address; no canonical
  public endpoint exists since the proxy is self-hosted), validated as URL.
- `APIKey` - LiteLLM proxy virtual key, `secret: true`.

No typed model constants are shipped: valid model strings depend entirely on the operator's LiteLLM
`model_list`, so `model` is a passthrough string.
