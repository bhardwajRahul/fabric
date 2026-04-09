**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

This microservice implements the `Turn` endpoint for the Google Gemini LLM provider. It translates between the Microbus LLM message format and the Gemini generateContent API format, handling functionCall/functionResponse parts and role mapping (assistant→model).

Types (`Message`, `Tool`, `ToolCall`, `TurnCompletion`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.
