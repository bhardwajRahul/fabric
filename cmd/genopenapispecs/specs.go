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

package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/microbus-io/errors"
)

// Specs is the intermediate representation written to the specs file. It is the
// contract between this tool and the feature-scaffolding skills: this tool fills
// it deterministically from the OpenAPI document, the skills render Microbus code
// from it. Schema bodies stay as raw JSON Schema fragments; mapping them to Go
// types is the skills' responsibility.
type Specs struct {
	OpenAPIVersion string                     `json:"openapiVersion,omitempty"`
	Info           SpecsInfo                  `json:"info"`
	Remote         SpecsRemote                `json:"remote"`
	Types          map[string]json.RawMessage `json:"types,omitempty"`
	Endpoints      []SpecsEndpoint            `json:"endpoints"`
}

type SpecsInfo struct {
	Title       string `json:"title,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// SpecsRemote describes the external API the generated microservice forwards
// to. In Microbus terms it is downstream of the microservice (the microservice
// calls it); "remote" matches the framework's "Connecting to Remote APIs"
// vocabulary and avoids the reverse-proxy sense of "upstream".
type SpecsRemote struct {
	BaseURL  string         `json:"baseURL"`
	Security *SpecsSecurity `json:"security,omitempty"`
}

// SpecsSecurity is the resolved authentication scheme. Type is one of "apiKey",
// "http-bearer", "http-basic", or "oauth2". For "apiKey", In is "header" or
// "query" and Name is the header or query parameter name. For the http and
// oauth2 types the credential rides in the Authorization header.
type SpecsSecurity struct {
	Type string `json:"type"`
	In   string `json:"in,omitempty"`
	Name string `json:"name,omitempty"`
}

// SpecsEndpoint is one OpenAPI operation classified as a Microbus function or
// web handler.
type SpecsEndpoint struct {
	Name        string         `json:"name"`
	Feature     string         `json:"feature"`
	Method      string         `json:"method"`
	Route       string         `json:"route"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Params      []SpecsParam   `json:"params,omitempty"`
	RequestBody *SpecsBody     `json:"requestBody,omitempty"`
	Response    *SpecsResponse `json:"response,omitempty"`
}

type SpecsParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required,omitempty"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	// GoType is an advisory Go type for unambiguous scalar (or scalar-slice)
	// parameters: int, int64, float64, bool, string, time.Time, or a []T of
	// those. Empty when the parameter is a $ref or otherwise not a clear
	// scalar - the skill maps those per its own conventions. It is a hint, not
	// generated code; the skill may override it.
	GoType string `json:"goType,omitempty"`
}

type SpecsBody struct {
	ContentType string          `json:"contentType"`
	Required    bool            `json:"required,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

type SpecsResponse struct {
	Status      int             `json:"status"`
	ContentType string          `json:"contentType,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

// encode marshals the specs as indented JSON with a trailing newline.
func (s *Specs) encode() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, errors.Trace(err)
	}
	return append(b, '\n'), nil
}

// buildSpecs transforms an OpenAPI document into the specs IR. baseURLOverride,
// when non-empty, takes precedence over the document's first server URL.
func buildSpecs(doc *oasDocument, baseURLOverride string) (*Specs, error) {
	s := &Specs{
		OpenAPIVersion: doc.OpenAPI,
		Info: SpecsInfo{
			Title:       doc.Info.Title,
			Version:     doc.Info.Version,
			Description: doc.Info.Description,
		},
		Types: doc.Components.Schemas,
	}

	s.Remote.BaseURL = baseURLOverride
	if s.Remote.BaseURL == "" && len(doc.Servers) > 0 {
		s.Remote.BaseURL = doc.Servers[0].URL
	}
	s.Remote.Security = resolveSecurity(doc)

	for path, item := range doc.Paths {
		for _, e := range item.operations() {
			ep := buildEndpoint(doc, path, e.method, e.op, item.Parameters)
			s.Endpoints = append(s.Endpoints, ep)
		}
	}

	// Stable order: by route, then method. Keeps regeneration diffs clean.
	sort.Slice(s.Endpoints, func(i, j int) bool {
		if s.Endpoints[i].Route != s.Endpoints[j].Route {
			return s.Endpoints[i].Route < s.Endpoints[j].Route
		}
		return s.Endpoints[i].Method < s.Endpoints[j].Method
	})

	if len(s.Endpoints) == 0 {
		return nil, errors.New("OpenAPI document declares no operations")
	}

	// Rewrite "$ref": "#/components/schemas/X" to "$ref": "X" so refs match the
	// bare names the types map is keyed by. The OpenAPI pointer prefix is not
	// meaningful in the IR.
	for name, sch := range s.Types {
		s.Types[name] = stripRefPrefix(sch)
	}
	for i := range s.Endpoints {
		for j := range s.Endpoints[i].Params {
			s.Endpoints[i].Params[j].Schema = stripRefPrefix(s.Endpoints[i].Params[j].Schema)
		}
		if b := s.Endpoints[i].RequestBody; b != nil {
			b.Schema = stripRefPrefix(b.Schema)
		}
		if r := s.Endpoints[i].Response; r != nil {
			r.Schema = stripRefPrefix(r.Schema)
		}
	}

	return s, nil
}

// buildEndpoint converts one operation into a specs endpoint, merging the
// path-level shared parameters with the operation's own.
func buildEndpoint(doc *oasDocument, path, method string, op *oasOperation, shared []oasParameter) SpecsEndpoint {
	ep := SpecsEndpoint{
		Name:        endpointName(op.OperationID, method, path),
		Method:      method,
		Route:       path,
		Summary:     op.Summary,
		Description: op.Description,
	}

	for _, p := range append(append([]oasParameter{}, shared...), op.Parameters...) {
		p = doc.resolveParameter(p)
		if p.In == "cookie" || p.Name == "" {
			continue
		}
		ep.Params = append(ep.Params, SpecsParam{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required || p.In == "path",
			Description: p.Description,
			Schema:      p.Schema,
			GoType:      goScalarType(p.Schema),
		})
	}

	body := doc.resolveRequestBody(op.RequestBody)
	if body != nil {
		ct, schema := pickContent(body.Content)
		if ct != "" {
			ep.RequestBody = &SpecsBody{ContentType: ct, Required: body.Required, Schema: schema}
		}
	}

	ep.Response = buildResponse(doc, op.Responses)
	ep.Feature = classify(ep)
	return ep
}

// buildResponse selects the lowest 2xx response and its JSON body, if any.
func buildResponse(doc *oasDocument, responses map[string]oasResponse) *SpecsResponse {
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		n, err := strconv.Atoi(code)
		if err != nil || n < 200 || n > 299 {
			continue
		}
		r := doc.resolveResponse(responses[code])
		ct, schema := pickContent(r.Content)
		return &SpecsResponse{Status: n, ContentType: ct, Schema: schema}
	}
	return nil
}

// pickContent selects the representative media type of a body. JSON is
// preferred and returned with its schema. When the body exists but has no JSON
// representation (binary, HTML, multipart), the first content type is returned
// with a nil schema, which signals a non-typed body that must be served raw. An
// absent body returns an empty content type.
func pickContent(content map[string]oasMediaType) (string, json.RawMessage) {
	if mt, ok := content["application/json"]; ok {
		return "application/json", mt.Schema
	}
	cts := make([]string, 0, len(content))
	for ct := range content {
		cts = append(cts, ct)
	}
	sort.Strings(cts)
	for _, ct := range cts {
		if strings.HasPrefix(ct, "application/json") || strings.HasSuffix(ct, "+json") {
			return ct, content[ct].Schema
		}
	}
	if len(cts) > 0 {
		return cts[0], nil
	}
	return "", nil
}

// classify decides whether an endpoint is a Microbus function or web handler. A
// non-JSON request or response body (HTML, binary, multipart, streaming) cannot
// be represented by a typed signature and becomes a raw web handler. Everything
// else - a JSON body, or a body-less operation - maps cleanly to a typed
// function with magic HTTP arguments.
func classify(ep SpecsEndpoint) string {
	if ep.RequestBody != nil && ep.RequestBody.Schema == nil {
		return "web"
	}
	if ep.Response != nil && ep.Response.ContentType != "" && ep.Response.Schema == nil {
		return "web"
	}
	return "function"
}

// resolveSecurity maps the document's primary security scheme to the IR. The
// scheme is chosen by, in order: a document-level security requirement, the
// scheme most frequently referenced by individual operations, then the
// alphabetically first declared scheme. Real documents often omit document-level
// security and attach an oauth2 or bearer requirement per operation, so the
// frequency pass keeps the wrapper's credential aligned with what the API
// actually enforces.
func resolveSecurity(doc *oasDocument) *SpecsSecurity {
	if len(doc.Components.SecuritySchemes) == 0 {
		return nil
	}

	preferred := firstSchemeName(doc.Security)
	if preferred == "" {
		counts := map[string]int{}
		for _, item := range doc.Paths {
			for _, e := range item.operations() {
				seen := map[string]bool{}
				for _, req := range e.op.Security {
					for name := range req {
						if !seen[name] {
							counts[name]++
							seen[name] = true
						}
					}
				}
			}
		}
		preferred = mostReferenced(counts)
	}
	if preferred == "" {
		names := make([]string, 0, len(doc.Components.SecuritySchemes))
		for name := range doc.Components.SecuritySchemes {
			names = append(names, name)
		}
		sort.Strings(names)
		preferred = names[0]
	}

	scheme, ok := doc.Components.SecuritySchemes[preferred]
	if !ok {
		return nil
	}
	switch strings.ToLower(scheme.Type) {
	case "apikey":
		return &SpecsSecurity{Type: "apiKey", In: scheme.In, Name: scheme.Name}
	case "http":
		if strings.EqualFold(scheme.Scheme, "basic") {
			return &SpecsSecurity{Type: "http-basic", In: "header", Name: "Authorization"}
		}
		return &SpecsSecurity{Type: "http-bearer", In: "header", Name: "Authorization"}
	case "oauth2", "openidconnect":
		return &SpecsSecurity{Type: "oauth2", In: "header", Name: "Authorization"}
	}
	return nil
}

// firstSchemeName returns the first scheme named by a security requirement
// list, deterministically when a requirement names more than one scheme.
func firstSchemeName(reqs []map[string][]string) string {
	for _, req := range reqs {
		keys := make([]string, 0, len(req))
		for name := range req {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			return keys[0]
		}
	}
	return ""
}

// mostReferenced returns the most frequently counted name, breaking ties
// alphabetically so the result is deterministic.
func mostReferenced(counts map[string]int) string {
	best := ""
	bestN := 0
	for name, n := range counts {
		if n > bestN || (n == bestN && (best == "" || name < best)) {
			best, bestN = name, n
		}
	}
	return best
}

// isAbsoluteURL reports whether s has an http or https scheme. An OpenAPI
// document may declare a relative server URL (e.g. "/api/v3"), which the
// delegating microservice cannot call without an explicit origin.
func isAbsoluteURL(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// goScalarType returns an advisory Go type for a scalar (or scalar-slice)
// parameter schema: int, int64, float64, bool, string, time.Time, or a []T of
// those. It returns "" for a $ref, an object, or anything not unambiguously
// scalar - those are the skill's to map. It is a hint only.
func goScalarType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s struct {
		Ref    string          `json:"$ref"`
		Type   string          `json:"type"`
		Format string          `json:"format"`
		Items  json.RawMessage `json:"items"`
	}
	err := json.Unmarshal(raw, &s)
	if err != nil || s.Ref != "" {
		return ""
	}
	switch s.Type {
	case "integer":
		if s.Format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "string":
		if s.Format == "date-time" {
			return "time.Time"
		}
		return "string"
	case "array":
		elem := goScalarType(s.Items)
		if elem == "" {
			return ""
		}
		return "[]" + elem
	}
	return ""
}

const refSchemaPrefix = "#/components/schemas/"

// stripRefPrefix rewrites every "$ref": "#/components/schemas/X" within a raw
// JSON Schema fragment to "$ref": "X", matching the bare names the types map is
// keyed by. The OpenAPI pointer prefix carries no meaning in the IR. raw is
// returned unchanged if it does not parse as JSON.
func stripRefPrefix(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	err := json.Unmarshal(raw, &v)
	if err != nil {
		return raw
	}
	out, err := json.Marshal(walkStripRef(v))
	if err != nil {
		return raw
	}
	return out
}

// walkStripRef recursively trims the schema-ref prefix from every "$ref" string.
func walkStripRef(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "$ref" {
				if s, ok := val.(string); ok {
					t[k] = strings.TrimPrefix(s, refSchemaPrefix)
					continue
				}
			}
			t[k] = walkStripRef(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = walkStripRef(val)
		}
		return t
	}
	return v
}

// endpointName derives a PascalCase feature name from the operationId, falling
// back to the HTTP method and path segments when no operationId is present.
func endpointName(operationID, method, path string) string {
	if name := pascal(operationID); name != "" {
		return name
	}
	parts := []string{strings.ToLower(method)}
	for _, seg := range strings.Split(path, "/") {
		seg = strings.Trim(seg, "{}.")
		if seg != "" {
			parts = append(parts, seg)
		}
	}
	name := pascal(strings.Join(parts, "_"))
	if name == "" {
		return "Endpoint"
	}
	return name
}

// pascal converts an identifier to PascalCase, splitting on any run of
// non-alphanumeric characters and uppercasing the first letter of each token
// while preserving existing interior casing.
func pascal(s string) string {
	var b strings.Builder
	startOfToken := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			if startOfToken && r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			}
			b.WriteRune(r)
			startOfToken = false
		default:
			startOfToken = true
		}
	}
	return b.String()
}
