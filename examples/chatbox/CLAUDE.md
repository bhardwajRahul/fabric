## Design Rationale

### Purpose

Chatbox is a demo LLM provider that implements the `Turn` endpoint without calling a real LLM API. It exists to demonstrate the full LLM tool-calling flow in the guided tour without requiring an API key. It can also serve as a template for building other mock providers for testing.

### How It Works

The `Turn` endpoint pattern-matches user messages using a regex that recognizes math questions in the form "what is X op Y?", "how much is X op Y?", or "calculate X op Y". Supported operators include `+ - * /` and their English equivalents (`plus`, `minus`, `times`, `multiplied by`, `divided by`, `over`).

When a calculator tool is available in the `tools` list, the chatbox generates a `ToolCall` for it - exactly as a real LLM would. When no tool is available, it computes the answer directly. For tool results (messages with `role: "tool"`), it formats the result as a natural-language response.

### Demo Page

The `/demo` web endpoint serves an interactive chat UI. On POST, it calls `llm.core`'s `Chat` endpoint passing `chatbox.example` as the provider hostname (since chatbox itself implements `Turn`). The LLM service routes back to chatbox over the bus, the chatbox returns a tool call, the LLM service executes it against the calculator, and the chatbox formats the final answer.

### Configuration

No API key is needed. The provider is now selected per-call via the `provider` argument to `Chat` rather than a config; pass `chatbox.example` as the provider hostname.

### Types

All types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to maintain a uniform interface with the real provider microservices. `Turn` returns flat values `(content, toolCalls, usage, err)` matching the v1.28.0 contract.
