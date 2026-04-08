**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

This microservice implements the `Turn` endpoint for the Claude (Anthropic) LLM provider. It translates between the Microbus LLM message format and the Claude Messages API format, handling system messages, tool_use/tool_result content blocks, and error parsing.

Types (`Message`, `ToolDef`, `ToolCall`, `TurnCompletion`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.
