## Create the MCP portal core service

Create a new core service `mcpportal` that exposes Microbus endpoints to LLM clients
via the Model Context Protocol (MCP). The service handles three logical MCP methods -
`initialize`, `tools/list`, and `tools/call` - over a single HTTP wire endpoint
`POST //mcp:0` (per the MCP spec, clients send JSON-RPC 2.0 envelopes to one URL).

`tools/list` should source its tool definitions from the openapi portal's aggregate doc
(`openapiportalapi.NewClient(svc).Document(...)`), filtered by the caller's claims and
the request's port (which the portal already does). Each operation in the doc that has a
matching `x-feature-type` (function/web/workflow) becomes one MCP tool with name = `x-name`,
description = operation description, inputSchema = operation request body / parameters
schema, and the operation's URL/method captured for invocation.

`tools/call` should invoke the named tool by issuing a Microbus request to the captured
URL/method with the provided arguments, then return the response body as the tool result.

`initialize` should respond with the MCP server's protocol version and capabilities.
Capabilities: `tools` (without `listChanged`, since the tool list is effectively static
within a session - see the design discussion).

## Refinements

- Hostname is `mcp.core` (matching the `.core` convention) so the wire URL is `/mcp` while
  the Go package directory remains `mcpportal`.
- Capture all design decisions in `AGENTS.md` rather than long Go comments.
- `tools/call` does not set `isError` based on response status code. In Microbus, real
  failures come back as `err != nil` from `Request`; non-2XX with `err == nil` is a normal
  response (e.g. a web handler returning 404 for a lookup miss). `isError: true` is set
  only when `Request` returned an error.
- `errorContent` accepts an `error` value so the message can be prefixed with the HTTP
  status code (`[404] tool not found`). Plain text - MCP clients pass `content[0].text`
  to the model as natural language; JSON wrappers add no signal a model can use.
- `fetchAggregate` uses `openapiportalapi.Document.URL()` and substitutes the wildcard `:0`
  port with the request's actual port, so the override URL stays in sync with any future
  route change in the openapi portal.
- No `defer res.Body.Close()` and no status-code check after `svc.Request` - Microbus
  closes the body itself, and a real failure already returns `err != nil`.
