## Design Rationale

### Purpose

Chatbox is a demo LLM provider that implements the `Turn` endpoint without calling a real LLM API. It exists to demonstrate the full LLM tool-calling flow in the guided tour without requiring an API key. It can also serve as a template for building other mock providers for testing.

### How It Works

The `Turn` endpoint pattern-matches user messages using a regex that recognizes math questions in the form "what is X op Y?", "how much is X op Y?", or "calculate X op Y". Supported operators include `+ - * /` and their English equivalents (`plus`, `minus`, `times`, `multiplied by`, `divided by`, `over`).

When a calculator tool is available in the `tools` list, the chatbox generates a `ToolCall` for it - exactly as a real LLM would. When no tool is available, it computes the answer directly. For tool results (messages with `role: "tool"`), it formats the result as a natural-language response.

### Demo Page

The `/demo` web endpoint serves an interactive chat UI. On POST, it calls `llm.core`'s `Chat` endpoint. The provider dropdown offers the simulated chatbox (which pins `provider=chatbox.example`) and "Any configured provider" at the `fast`/`default`/`smart` tiers (which pass `provider=any` + the tier so `llm.core` auto-selects whichever real provider has a key). The LLM service routes back to chatbox (or the resolved real provider) over the bus, the provider returns a tool call, the LLM service executes it against the calculator, and the provider formats the final answer.

### Walled off from provider resolution

Unlike the real provider microservices, chatbox deliberately does **not** subscribe to `llm.core`'s `OnResolveProvider` event. This keeps the simulator out of `"any"` resolution entirely, so a chatbox accidentally left running in a non-dev app can never absorb portable-tier traffic - it is reachable only by explicitly pinning `provider="chatbox.example"`. See `coreservices/llm/CLAUDE.md` "Provider and Model Resolution".

### Configuration

No API key is needed. The provider is now selected per-call via the `provider` argument to `Chat` rather than a config; pass `chatbox.example` as the provider hostname.

### Types

All types (`Item`, `Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to maintain a uniform interface with the real provider microservices. `Turn` takes and returns the `[]llmapi.Item` conversation model and returns `(items, stopReason, usage, err)`, the same contract as the real provider microservices: it reads the last item (a `user` message or a `tool_result`) and returns its assistant reply as a message item, plus a `tool_call` item when it wants the calculator.
