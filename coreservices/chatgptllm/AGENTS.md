**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

This microservice implements the `Turn` endpoint for the ChatGPT LLM provider. It translates between the Microbus LLM message format and the OpenAI Chat Completions API format, handling tool_calls on assistant messages and tool_call_id on tool result messages.

Types (`Message`, `Tool`, `ToolCall`, `TurnCompletion`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.
