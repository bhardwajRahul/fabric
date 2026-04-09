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

package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/pub"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// resolveTools fetches OpenAPI schemas for the given tools and converts them to tool definitions.
func (svc *Service) resolveTools(ctx context.Context, tools []llmapi.Tool) ([]llmapi.ToolDef, error) {
	defs := make([]llmapi.ToolDef, 0, len(tools))
	for _, tool := range tools {
		def, err := svc.resolveToolFromOpenAPI(ctx, tool)
		if err != nil {
			return nil, errors.Trace(err)
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// resolveToolFromOpenAPI fetches the OpenAPI spec for a tool's endpoint and extracts the tool definition.
func (svc *Service) resolveToolFromOpenAPI(ctx context.Context, tool llmapi.Tool) (llmapi.ToolDef, error) {
	toolURL := tool.URL
	if !strings.Contains(toolURL, "://") {
		toolURL = "https://" + toolURL
	}

	// The OpenAPI spec is at the same host:port with the path replaced by /openapi.json
	var openAPIURL string
	doubleSlash := strings.Index(toolURL, "://")
	pathSlash := strings.Index(toolURL[doubleSlash+3:], "/")
	if pathSlash > 0 {
		openAPIURL = toolURL[:doubleSlash+3+pathSlash] + "/openapi.json"
	} else {
		openAPIURL = toolURL + "/openapi.json"
	}

	resp, err := svc.Request(
		ctx,
		pub.GET(openAPIURL),
	)
	if err != nil {
		return llmapi.ToolDef{}, errors.Trace(err)
	}
	if resp == nil || resp.Body == nil {
		return llmapi.ToolDef{}, errors.New("nil response from OpenAPI endpoint: %s", openAPIURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmapi.ToolDef{}, errors.New("OpenAPI endpoint returned %d: %s", resp.StatusCode, openAPIURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return llmapi.ToolDef{}, errors.Trace(err)
	}

	// Parse the OpenAPI spec
	var doc openapi.Doc
	err = json.Unmarshal(body, &doc)
	if err != nil {
		return llmapi.ToolDef{}, errors.Trace(err)
	}

	// Find the matching path and operation
	fullPath := toolURL[doubleSlash+2:]
	var op *openapi.Operation
	var httpMethod string
	for specPath, methods := range doc.Paths {
		if specPath == fullPath {
			for method, operation := range methods {
				httpMethod = strings.ToUpper(method)
				op = operation
				break
			}
			break
		}
	}
	if op == nil {
		return llmapi.ToolDef{}, errors.New("endpoint not found in OpenAPI spec: %s", tool.URL)
	}

	// Build the tool name from the summary or path
	name := op.Summary
	if idx := strings.Index(name, "("); idx > 0 {
		name = name[:idx]
	}
	if name == "" {
		name = strings.ReplaceAll(strings.TrimPrefix(fullPath, "/"), "/", "_")
	}

	// Build the input schema
	var inputSchema json.RawMessage
	if op.RequestBody != nil && op.RequestBody.Content != nil {
		// POST-style: use the request body schema
		if jsonContent, ok := op.RequestBody.Content["application/json"]; ok && jsonContent.Schema != nil {
			schema := jsonContent.Schema
			// Follow $ref if present
			if schema.Ref != "" && doc.Components != nil {
				parts := strings.Split(schema.Ref, "/")
				refName := parts[len(parts)-1]
				if resolved, ok := doc.Components.Schemas[refName]; ok {
					schema = resolved
				}
			}
			inputSchema, _ = json.Marshal(schema)
		}
	} else if len(op.Parameters) > 0 {
		// GET-style: build a schema from query parameters
		inputSchema = buildSchemaFromParams(op.Parameters)
	}
	if inputSchema == nil {
		inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
	}

	description := op.Description
	if tool.Description != "" {
		description = tool.Description
	}

	return llmapi.ToolDef{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		URL:         tool.URL,
		Method:      httpMethod,
		FeatureType: op.XFeatureType,
	}, nil
}

// buildSchemaFromParams constructs a JSON Schema from OpenAPI query parameters.
func buildSchemaFromParams(params []*openapi.Parameter) json.RawMessage {
	properties := map[string]json.RawMessage{}
	required := []string{}
	for _, p := range params {
		if p.In == "query" || p.In == "path" {
			schemaJSON, _ := json.Marshal(p.Schema)
			if p.Description != "" {
				// Merge description into the schema
				var s map[string]any
				if json.Unmarshal(schemaJSON, &s) == nil {
					s["description"] = p.Description
					schemaJSON, _ = json.Marshal(s)
				}
			}
			properties[p.Name] = schemaJSON
			if p.Required {
				required = append(required, p.Name)
			}
		}
	}
	result := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	data, _ := json.Marshal(result)
	return data
}

// executeTool invokes a Microbus endpoint over the bus with the given arguments and returns the JSON result.
func (svc *Service) executeTool(ctx context.Context, tc llmapi.ToolCall, tools []llmapi.ToolDef) (json.RawMessage, error) {
	// Find the tool definition to get the URL and method
	var def llmapi.ToolDef
	for _, t := range tools {
		if t.Name == tc.Name {
			def = t
			break
		}
	}
	if def.URL == "" {
		return nil, errors.New("tool not found: %s", tc.Name)
	}

	// Ensure the URL has a scheme
	toolURL := def.URL
	if !strings.Contains(toolURL, "://") {
		toolURL = "https://" + toolURL
	}

	// Use the method from OpenAPI, default to POST
	method := def.Method
	if method == "" || method == "ANY" {
		method = "POST"
	}

	// Invoke the endpoint over the bus
	resp, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(toolURL),
		pub.Body(tc.Arguments),
		pub.ContentType("application/json"),
	)
	if err != nil {
		// Return the error as a tool result rather than failing the whole chat
		errResult, _ := json.Marshal(map[string]string{"error": err.Error()})
		return errResult, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		errResult, _ := json.Marshal(map[string]string{"error": string(body)})
		return errResult, nil
	}

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return result, nil
}
