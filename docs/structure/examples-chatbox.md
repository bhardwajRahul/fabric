# Package `examples/chatbox`

The chatbox example microservice demonstrates the LLM tool-calling features of the framework without requiring a real LLM API key. It implements a mock LLM provider that pattern-matches user messages and decides whether to call a downstream tool - exercising the same code path a real provider takes.

The microservice plugs into [`llm.core`](../structure/coreservices-llm.md) as a provider by exposing the same `Turn` functional endpoint that `claudellm`, `chatgptllm`, and `geminillm` expose. To use it, point `llm.core` at the chatbox via configuration:

```yaml
llm.core:
  ProviderHostname: chatbox.example
```

When chatbox sees a math-shaped question (e.g. *"What is 6 times 7?"*), it returns a tool-call request for the [calculator microservice](../structure/examples-calculator.md)'s `Arithmetic` endpoint. `llm.core` resolves the requested tool URL against the calculator's OpenAPI document, dispatches the call over the bus, feeds the result back to chatbox, and chatbox produces the final natural-language answer. Unrecognized questions are echoed back verbatim.

The example also serves an interactive demo page at `//chatbox.example/demo` that wires a browser UI to the LLM service so the full tool-calling loop can be exercised in a browser without external dependencies.

This example is the LLM counterpart to the [creditflow example](../structure/examples-creditflow.md): creditflow demonstrates the foreman's workflow engine, while chatbox demonstrates the LLM service's tool-calling engine.
