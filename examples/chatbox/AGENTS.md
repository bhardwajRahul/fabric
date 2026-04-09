**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

### Purpose

Chatbox is a demo LLM provider that implements the `Turn` endpoint without calling a real LLM API. It exists to demonstrate the full LLM tool-calling flow in the guided tour without requiring an API key. It can also serve as a template for building other mock providers for testing.

### How It Works

The `Turn` endpoint pattern-matches user messages using a regex that recognizes math questions in the form "what is X op Y?", "how much is X op Y?", or "calculate X op Y". Supported operators include `+ - * /` and their English equivalents (`plus`, `minus`, `times`, `multiplied by`, `divided by`, `over`).

When a calculator tool is available in the `tools` list, the chatbox generates a `ToolCall` for it - exactly as a real LLM would. When no tool is available, it computes the answer directly. For tool results (messages with `role: "tool"`), it formats the result as a natural-language response.

### Demo Page

The `/demo` web endpoint serves an interactive chat UI. On POST, it calls the LLM core service's `Chat` endpoint with the calculator as a tool. The LLM service delegates to the chatbox via the `ProviderHostname` config, the chatbox returns a tool call, the LLM service executes it against the calculator, and the chatbox formats the final answer. The UI displays all messages including the intermediate tool call and result.

### Configuration

To use the chatbox as the provider, set `ProviderHostname: chatbox.example` on the `llm.core` service. No API key is needed.

### Types

All types (`Message`, `Tool`, `ToolCall`, `TurnCompletion`) are imported from `llmapi` to maintain a uniform interface with the real provider microservices.
