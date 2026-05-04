# Package `examples/chatbox`

The chatbox example microservice demonstrates the LLM tool-calling features of the framework. It serves an interactive demo page at `//chatbox.example/demo` with a provider dropdown that switches between a built-in mock provider and the real Claude, ChatGPT, and Gemini providers, so the same UI can be exercised offline or against a live LLM API depending on what credentials are configured.

The microservice plugs into [`llm.core`](../structure/coreservices-llm.md) as a provider by exposing the same `Turn` functional endpoint that `claudellm`, `chatgptllm`, and `geminillm` expose. To use the chatbox itself as the provider, pass `chatbox.example` as the provider hostname when calling `llm.core`'s `Chat` endpoint:

```go
messagesOut, usage, err := llmapi.NewClient(svc).Chat(
    ctx,
    "chatbox.example",   // provider
    "",                  // model (chatbox ignores this)
    messages,
    toolURLs,
    nil,
)
```

There is no `ProviderHostname` config to set - provider selection is per-call, and the demo page passes whichever provider the user picked from the dropdown.

When the chatbox sees a math-shaped question (e.g. *"What is 6 times 7?"*), it returns a tool-call request for the [calculator microservice](../structure/examples-calculator.md)'s `Arithmetic` endpoint. `llm.core` resolves the requested tool URL against the calculator's OpenAPI document, dispatches the call over the bus, feeds the result back to the chatbox, and the chatbox produces the final natural-language answer. Unrecognized questions are echoed back verbatim.

For the real-LLM options (Claude, ChatGPT, Gemini), the demo page uses the same dropdown to select the provider hostname; the corresponding provider microservice must be running and have a valid `APIKey` configured in `config.local.yaml`.

This example is the LLM counterpart to the [creditflow example](../structure/examples-creditflow.md): creditflow demonstrates the foreman's workflow engine, while chatbox demonstrates the LLM service's tool-calling engine.
