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
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/invopop/jsonschema"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/pub"

	"github.com/microbus-io/fabric/coreservices/control/controlapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// fetchTools resolves a list of Microbus endpoint URLs into the LLM tool definitions consumed
// by the chat loop. URLs are grouped by host:port; each distinct host's OpenAPI document is
// fetched once via the control microservice and scanned for the operation matching each URL.
func (svc *Service) fetchTools(ctx context.Context, toolURLs []string) ([]llmapi.Tool, error) {
	if len(toolURLs) == 0 {
		return nil, nil
	}

	type urlInfo struct {
		raw      string
		hostPort string
		docKey   string
	}
	infos := make([]urlInfo, 0, len(toolURLs))
	hostOrder := make([]string, 0)
	seenHost := map[string]bool{}
	for _, raw := range toolURLs {
		hostPort, docKey, err := canonicalizeToolURL(raw)
		if err != nil {
			return nil, errors.Trace(err, "url", raw)
		}
		infos = append(infos, urlInfo{raw: raw, hostPort: hostPort, docKey: docKey})
		if !seenHost[hostPort] {
			seenHost[hostPort] = true
			hostOrder = append(hostOrder, hostPort)
		}
	}

	docs := make(map[string]*openapi.Document, len(hostOrder))
	var docsMu sync.Mutex
	jobs := make([]func() error, 0, len(hostOrder))
	for _, hp := range hostOrder {
		hp := hp
		jobs = append(jobs, func() error {
			doc, err := svc.fetchOpenAPIDoc(ctx, hp)
			if err != nil {
				return errors.Trace(err, "host", hp)
			}
			docsMu.Lock()
			docs[hp] = doc
			docsMu.Unlock()
			return nil
		})
	}
	if err := svc.Parallel(jobs...); err != nil {
		return nil, err // No trace
	}

	tools := make([]llmapi.Tool, 0, len(infos))
	usedNames := map[string]bool{}
	for _, info := range infos {
		doc := docs[info.hostPort]
		ops, ok := doc.Paths[info.docKey]
		if !ok {
			return nil, errors.New("no openapi entry for tool URL", "url", info.raw)
		}
		for method, op := range ops {
			tool, err := operationToTool(info.raw, method, op, doc)
			if err != nil {
				return nil, errors.Trace(err, "url", info.raw)
			}
			name := tool.Name
			for i := 2; usedNames[name]; i++ {
				name = tool.Name + "_" + strconv.Itoa(i)
			}
			usedNames[name] = true
			tool.Name = name
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

// fetchOpenAPIDoc retrieves the OpenAPI document for a specific host via the connector's
// built-in /openapi.json handler, which lives on :888 regardless of the port a tool listens
// on. The returned doc covers endpoints across all ports (filtered by the caller's claims);
// the per-tool path lookup keys remain port-qualified, so the call site can match on the
// tool's actual host:port.
func (svc *Service) fetchOpenAPIDoc(ctx context.Context, hostPort string) (*openapi.Document, error) {
	host, _, _ := strings.Cut(hostPort, ":")
	doc, status, err := controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if status != 0 && status != http.StatusOK {
		return nil, errors.New("openapi fetch failed", "status", status)
	}
	if doc == nil {
		return nil, errors.New("empty openapi document")
	}
	return doc, nil
}

// canonicalizeToolURL parses a tool URL and returns the host:port (port defaulting to 443) and
// the canonical OpenAPI path key (e.g. "/calculator.example:443/arithmetic").
func canonicalizeToolURL(raw string) (hostPort, docKey string, err error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	if u.Hostname() == "" {
		return "", "", errors.New("missing hostname in tool URL")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	hostPort = u.Hostname() + ":" + port
	path := u.Path
	if path == "" {
		path = "/"
	}
	docKey = "/" + hostPort + path
	return hostPort, docKey, nil
}

// operationToTool converts an OpenAPI operation into an LLM-callable tool.
func operationToTool(toolURL, method string, op *openapi.Operation, doc *openapi.Document) (llmapi.Tool, error) {
	switch op.XFeatureType {
	case openapi.FeatureFunction, openapi.FeatureWeb, openapi.FeatureWorkflow:
	default:
		return llmapi.Tool{}, errors.New("unsupported feature type", "type", op.XFeatureType)
	}
	name := op.XName
	if name == "" {
		name = firstWord(op.Summary)
	}
	if name == "" {
		name = firstWord(op.Description)
	}
	schema, err := operationInputSchema(op, doc)
	if err != nil {
		return llmapi.Tool{}, errors.Trace(err)
	}
	return llmapi.Tool{
		Name:        name,
		Description: op.Description,
		InputSchema: schema,
		URL:         toolURL,
		Method:      strings.ToUpper(method),
		Type:        op.XFeatureType,
	}, nil
}

// operationInputSchema builds the JSON Schema describing an operation's input. For operations
// with a request body, the body schema is returned (resolving any $ref). Otherwise an object
// schema is synthesized from the operation's parameters.
func operationInputSchema(op *openapi.Operation, doc *openapi.Document) (json.RawMessage, error) {
	if op.RequestBody != nil {
		media := op.RequestBody.Content["application/json"]
		if media != nil && media.Schema != nil {
			schema := resolveSchemaRef(media.Schema, doc)
			data, err := json.Marshal(schema)
			if err != nil {
				return nil, errors.Trace(err)
			}
			return data, nil
		}
	}
	props := jsonschema.NewProperties()
	required := []string{}
	for _, p := range op.Parameters {
		if p == nil || p.Name == "" {
			continue
		}
		propSchema := resolveSchemaRef(p.Schema, doc)
		if propSchema == nil {
			propSchema = &jsonschema.Schema{Type: "string"}
		}
		if p.Description != "" && propSchema.Description == "" {
			propSchema.Description = p.Description
		}
		props.Set(p.Name, propSchema)
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := &jsonschema.Schema{
		Type:                 "object",
		Properties:           props,
		Required:             required,
		AdditionalProperties: nil,
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return data, nil
}

// resolveSchemaRef resolves a "#/components/schemas/Foo" $ref against the document's components,
// returning the referenced schema if found or the original schema otherwise.
func resolveSchemaRef(schema *jsonschema.Schema, doc *openapi.Document) *jsonschema.Schema {
	if schema == nil || schema.Ref == "" || doc == nil || doc.Components == nil {
		return schema
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(schema.Ref, prefix) {
		return schema
	}
	if resolved, ok := doc.Components.Schemas[schema.Ref[len(prefix):]]; ok && resolved != nil {
		return resolved
	}
	return schema
}

// firstWord extracts the leading identifier-like token (letters, digits, underscore) from s,
// skipping any leading non-identifier characters. Returns "" if no such token exists.
func firstWord(s string) string {
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		isIdent := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
		if start < 0 {
			if isIdent {
				start = i
			}
			continue
		}
		if !isIdent {
			return s[start:i]
		}
	}
	if start < 0 {
		return ""
	}
	return s[start:]
}

// executeTool invokes a Microbus endpoint over the bus with the given arguments and returns the JSON result.
func (svc *Service) executeTool(ctx context.Context, tc llmapi.ToolCall, tools []llmapi.Tool) (json.RawMessage, error) {
	// Find the tool definition to get the URL and method
	var def llmapi.Tool
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

	// Use the method from the tool definition, default to POST
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
