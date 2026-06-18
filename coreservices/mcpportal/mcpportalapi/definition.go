/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mcpportalapi

import (
	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "mcp.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "MCPPortal"

// Version is the major version of the microservice's public API.
const Version = 1

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The MCP portal exposes the bus's tools to LLM clients via the Model Context Protocol.
Clients send JSON-RPC 2.0 envelopes to a single wire endpoint (POST //mcp:0); the service
dispatches on the JSON-RPC method to handlers for initialize, tools/list, and tools/call.`

// MCP is the JSON-RPC 2.0 wire endpoint for Model Context Protocol clients. Dispatches on the JSON-RPC method to internal handlers for initialize, tools/list, and tools/call.
var MCP = define.Web{
	Host: Hostname, Method: "POST", Route: "//mcp:0",
}
