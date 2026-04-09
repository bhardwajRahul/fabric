# Model Context Protocol

The [Model Context Protocol (MCP)](https://modelcontextprotocol.io) is an open standard for connecting LLM clients to external tools and data sources. MCP-aware clients such as Claude Desktop and Claude Code can discover and invoke tools exposed by an MCP server, without knowing anything about how those tools are implemented behind the scenes.

The [MCP portal core microservice](../structure/coreservices-mcpportal.md) makes every Microbus endpoint invokable as an MCP tool. Functional, web and workflow endpoints across the bus surface through the standard MCP protocol - the LLM client connects to one URL and gets a unified view of the application.

Tool definitions are derived from the [OpenAPI documents](../blocks/openapi.md) the framework already produces, so [authorization](../blocks/authorization.md), descriptions and input schemas are kept in sync without duplication. The caller sees only the tools they're authorized to invoke.
