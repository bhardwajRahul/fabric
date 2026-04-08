## Create LLM Core Microservice

Create a core microservice named `llm` with hostname `llm.core` that bridges LLM tool-calling protocols with Microbus endpoint invocations. The service allows callers to pass a prompt and a list of Microbus endpoint URLs as "tools". It handles schema discovery from OpenAPI, LLM communication, and tool execution over the bus.

The service is provider-agnostic, supporting Claude, OpenAI, and Gemini backends via configuration. It fetches endpoint schemas from OpenAPI at call time to build tool definitions, executes tool calls by invoking Microbus endpoints directly over the bus, and caches resolved schemas in the distributed cache.


## Add ChatLoop Workflow and Additional Providers

Add a built-in ChatLoop workflow that orchestrates multi-turn LLM conversations as durable workflow steps. The workflow uses forEach to fan out one ExecuteTool step per tool call, with ReducerAppend on messages for fan-in. ExecuteTool uses toolExecuted/toolExecutedOut for dynamic subgraph re-entry detection.

Add OpenAI and Gemini provider implementations alongside the existing Claude provider.

## Refactor ChatLoop to forEach Pattern

Refactor the ChatLoop workflow from a single ExecuteTools task that processes all tool calls in a loop to a forEach-based pattern where each tool call gets its own ExecuteTool step. This enables parallel tool execution and prepares for dynamic subgraph support where workflow tools are executed as child flows.

ProcessResponse sets pendingToolCalls for forEach fan-out. When the array is empty (no tool calls), the flow terminates naturally.

## Extract Providers into Separate Microservices

Extract the three LLM providers (Claude, OpenAI, Gemini) into separate microservices (`claudellm`, `openaillm`, `geminillm`), each implementing a uniform `Turn` endpoint. The LLM service becomes a pure orchestrator that delegates to the configured provider via `llmapi.NewClient(svc).ForHost(providerHostname).Turn(...)`. Remove the internal provider interface and provider files. Add a `Turn` endpoint to the LLM service that delegates to the provider. Rename `Provider` config to `ProviderHostname` (the hostname IS the config value). Remove `maxToolRounds` parameter from `Chat` - it's now config-only.
