# Package `coreservices/mcpportal`

The MCP portal core microservice exposes the bus's tools to LLM clients via the [Model Context Protocol](../blocks/mcp.md). MCP-aware clients such as Claude Desktop and Claude Code connect to a single endpoint, list the tools they're authorized to invoke, and call them - the portal resolves each tool to a Microbus endpoint and dispatches the call over the bus.

Tool definitions come from the [OpenAPI documents](../blocks/openapi.md) the framework already produces, so authorization, descriptions, and input schemas are kept in sync without duplication. The caller sees only the tools they're authorized to invoke.
