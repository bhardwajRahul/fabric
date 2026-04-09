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

package openapiportal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/coreservices/control/controlapi"
	"github.com/microbus-io/fabric/coreservices/openapiportal/openapiportalapi"
	"github.com/microbus-io/fabric/dlru"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/openapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ *openapiportalapi.Client
)

/*
Service implements the openapiportal.core microservice. It exposes two web endpoints -
`Document` (machine-readable JSON) and `Explorer` (human-readable HTML) - both surfacing
the bus's OpenAPI documents. See AGENTS.md for design rationale.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	cache := svc.DistribCache()
	if err := cache.SetMaxAge(30 * time.Second); err != nil {
		return errors.Trace(err)
	}
	if err := cache.SetMaxMemory(64 * 1024 * 1024); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Document returns the OpenAPI 3.1 document as JSON. Without `?hostname=<host>` it returns
// an aggregate; with the query arg it proxies to a single service's `:888/openapi.json`.
func (svc *Service) Document(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Document
	if hostname := r.URL.Query().Get("hostname"); hostname != "" {
		return svc.openAPISingle(w, r, hostname)
	}
	return svc.openAPIAggregate(w, r)
}

// openAPIAggregate multicasts `//all:888/openapi.json`, merges the per-service docs into one
// document with one tag per service, filters paths by the request's port, and prunes orphan
// schemas + tags. Result is cached in DistribCache by (claims-digest, request-port).
func (svc *Service) openAPIAggregate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	reqPort := r.URL.Port()
	cacheKey := "agg:" + claimsDigest(r) + ":" + reqPort
	cache := svc.DistribCache()

	var cached []byte
	found, err := cache.Get(ctx, cacheKey, &cached)
	if err != nil {
		return errors.Trace(err)
	}
	if found {
		return writeJSON(w, cached)
	}

	// Schemas are spliced in at marshal time (assembleAggregate) so we don't need to import
	// the jsonschema typing into this file.
	aggregate := &openapi.Document{
		OpenAPI: "3.1.0",
		Info: openapi.Info{
			Title:   "Microbus",
			Version: "1",
		},
		Paths: map[string]map[string]*openapi.Operation{},
		Components: &openapi.Components{
			SecuritySchemes: map[string]*openapi.SecurityScheme{},
		},
	}
	mergedSchemas := map[string]any{}

	tagsByName := map[string]*openapi.Tag{}
	for r := range controlapi.NewMulticastClient(svc).ForHost("all").OpenAPI(ctx) {
		doc, status, err := r.Get()
		if err != nil {
			return errors.Trace(err)
		}
		if status != 0 && status != http.StatusOK {
			return errors.New("openapi fetch failed", "status", status)
		}
		if doc == nil {
			continue
		}
		hostname := doc.Info.Title
		if _, exists := tagsByName[hostname]; !exists && hostname != "" {
			tag := &openapi.Tag{Name: hostname, Description: doc.Info.Description}
			tagsByName[hostname] = tag
			aggregate.Tags = append(aggregate.Tags, tag)
		}
		// Merge paths, port-filtered and tagged.
		for path, methods := range doc.Paths {
			if !portMatches(path, reqPort) {
				continue
			}
			for method, op := range methods {
				if op == nil {
					continue
				}
				if hostname != "" {
					op.Tags = append(op.Tags, hostname)
				}
				if aggregate.Paths[path] == nil {
					aggregate.Paths[path] = map[string]*openapi.Operation{}
				}
				aggregate.Paths[path][method] = op
			}
		}
		// Merge components.
		if doc.Components != nil {
			for k, v := range doc.Components.Schemas {
				mergedSchemas[k] = v
			}
			for k, v := range doc.Components.SecuritySchemes {
				aggregate.Components.SecuritySchemes[k] = v
			}
		}
	}

	// Prune schemas no longer referenced by any retained path. Walk transitively because
	// schemas can reference other schemas.
	keptRefs := collectRefs(aggregate)
	for {
		grew := false
		for k := range keptRefs {
			schema, ok := mergedSchemas[k]
			if !ok {
				continue
			}
			for r := range collectRefsFromValue(schema) {
				if !keptRefs[r] {
					keptRefs[r] = true
					grew = true
				}
			}
		}
		if !grew {
			break
		}
	}
	for k := range mergedSchemas {
		if !keptRefs[k] {
			delete(mergedSchemas, k)
		}
	}

	// Prune tags whose service contributed no kept operation.
	usedTags := map[string]bool{}
	for _, methods := range aggregate.Paths {
		for _, op := range methods {
			for _, t := range op.Tags {
				usedTags[t] = true
			}
		}
	}
	keptTags := aggregate.Tags[:0]
	for _, t := range aggregate.Tags {
		if usedTags[t.Name] {
			keptTags = append(keptTags, t)
		}
	}
	aggregate.Tags = keptTags

	body, err := assembleAggregate(aggregate, mergedSchemas)
	if err != nil {
		return errors.Trace(err)
	}

	if err := cache.Set(ctx, cacheKey, body, dlru.Compress(true)); err != nil {
		return errors.Trace(err)
	}
	return writeJSON(w, body)
}

// Explorer renders a human-friendly HTML browser. Without `?hostname=<host>` it lists every
// service; with the query arg it shows that service's endpoints expanded.
func (svc *Service) Explorer(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Explorer
	if hostname := r.URL.Query().Get("hostname"); hostname != "" {
		return svc.exploreHost(w, r, hostname)
	}
	return svc.exploreList(w, r)
}

// exploreList multicasts to every service and renders a one-line summary per service.
func (svc *Service) exploreList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	reqPort := r.URL.Port()

	var entries []*exploreSvc
	for resp := range controlapi.NewMulticastClient(svc).ForHost("all").OpenAPI(ctx) {
		doc, _, err := resp.Get()
		if err != nil || doc == nil {
			continue
		}
		count := 0
		for path, methods := range doc.Paths {
			if !portMatches(path, reqPort) {
				continue
			}
			count += len(methods)
		}
		if count == 0 {
			continue
		}
		entries = append(entries, &exploreSvc{
			Hostname:    doc.Info.Title,
			Description: doc.Info.Description,
			Count:       count,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Hostname < entries[j].Hostname })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	return errors.Trace(svc.WriteResTemplate(w, "explore.html", &exploreData{Services: entries}))
}

// exploreOp is the per-operation data passed into the explore template's host view.
// Schemas are pre-marshaled to indented JSON strings so the template can drop them straight
// into a `<pre>` block.
type exploreOp struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Name        string
	FeatureType string
	Parameters  []*exploreParam
	RequestBody string
	Responses   []*exploreResp
}

// exploreSvc is one row in the list view: hostname, description, and the count of operations
// the caller can reach on the requested port.
type exploreSvc struct {
	Hostname    string
	Description string
	Count       int
}

// exploreData is the unified template input for both the list view (Hostname empty,
// Services populated) and the host view (Hostname set, Operations populated). One struct
// keeps the template happy regardless of which branch runs.
type exploreData struct {
	Hostname    string
	Description string
	Services    []*exploreSvc
	Operations  []*exploreOp
}

// exploreParam describes one parameter on an operation. `In` is "query" / "path" / "header".
type exploreParam struct {
	Name        string
	In          string
	Required    bool
	Description string
	Schema      string
}

// exploreResp describes one response code on an operation. `Schema` is empty when the
// response has no JSON content (e.g. plain web responses).
type exploreResp struct {
	Code        string
	Description string
	Schema      string
}

// exploreHost fetches a single service's doc and renders its operations sorted by path
// then method.
func (svc *Service) exploreHost(w http.ResponseWriter, r *http.Request, hostname string) error {
	ctx := r.Context()
	reqPort := r.URL.Port()

	doc, status, err := controlapi.NewClient(svc).ForHost(hostname).OpenAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if status != 0 && status != http.StatusOK {
		return errors.New("openapi fetch failed", "status", status)
	}
	if doc == nil {
		return errors.New("not found", http.StatusNotFound)
	}

	var ops []*exploreOp
	for path, methods := range doc.Paths {
		if !portMatches(path, reqPort) {
			continue
		}
		for method, op := range methods {
			if op == nil {
				continue
			}
			ops = append(ops, buildExploreOp(doc, method, path, op))
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	return errors.Trace(svc.WriteResTemplate(w, "explore.html", &exploreData{
		Hostname:    hostname,
		Description: doc.Info.Description,
		Operations:  ops,
	}))
}

// openAPISingle is a thin proxy to a single service's `:888/openapi.json`.
func (svc *Service) openAPISingle(w http.ResponseWriter, r *http.Request, hostname string) error {
	doc, status, err := controlapi.NewClient(svc).ForHost(hostname).OpenAPI(r.Context())
	if err != nil {
		return errors.Trace(err)
	}
	if status != 0 && status != http.StatusOK {
		return errors.New("openapi fetch failed", "status", status)
	}
	if doc == nil {
		return errors.New("empty openapi document", http.StatusBadGateway)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return errors.Trace(err)
	}
	return writeJSON(w, body)
}

// assembleAggregate marshals the aggregate doc, splicing in the merged schemas at marshal
// time via decode-modify-encode. Schemas held as `any` keep the map agnostic of jsonschema
// typing. Schema keys are sorted for stable output regardless of map iteration order.
func assembleAggregate(doc *openapi.Document, schemas map[string]any) ([]byte, error) {
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(schemas) == 0 {
		return docBytes, nil
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(docBytes, &asMap); err != nil {
		return nil, errors.Trace(err)
	}
	var components map[string]json.RawMessage
	if raw, ok := asMap["components"]; ok {
		if err := json.Unmarshal(raw, &components); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		components = map[string]json.RawMessage{}
	}
	keys := make([]string, 0, len(schemas))
	for k := range schemas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]any, len(schemas))
	for _, k := range keys {
		ordered[k] = schemas[k]
	}
	schemasJSON, err := json.Marshal(ordered)
	if err != nil {
		return nil, errors.Trace(err)
	}
	components["schemas"] = schemasJSON
	componentsJSON, err := json.Marshal(components)
	if err != nil {
		return nil, errors.Trace(err)
	}
	asMap["components"] = componentsJSON
	return json.Marshal(asMap)
}

// collectRefs walks every kept path/operation in the doc and returns the set of schema keys
// reachable via `$ref` strings of the form `#/components/schemas/<key>`. The caller expands
// this transitively through `collectRefsFromValue` to follow schema-to-schema references.
func collectRefs(doc *openapi.Document) map[string]bool {
	keys := map[string]bool{}
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return keys
	}
	scanRefs(string(docBytes), keys)
	return keys
}

// collectRefsFromValue marshals a single schema (held as `any` since the merged schema map
// stores raw passthrough values) and returns the set of `$ref` schema keys it contains.
func collectRefsFromValue(v any) map[string]bool {
	keys := map[string]bool{}
	b, err := json.Marshal(v)
	if err != nil {
		return keys
	}
	scanRefs(string(b), keys)
	return keys
}

// scanRefs adds every `#/components/schemas/<key>` reference found in s to keys.
func scanRefs(s string, keys map[string]bool) {
	const prefix = `"$ref":"#/components/schemas/`
	for {
		i := strings.Index(s, prefix)
		if i < 0 {
			return
		}
		s = s[i+len(prefix):]
		j := strings.IndexByte(s, '"')
		if j < 0 {
			return
		}
		keys[s[:j]] = true
		s = s[j:]
	}
}

// buildExploreOp assembles the expanded data for one operation: parameters, request body
// schema, and per-response-code schemas. Schema $refs are followed to the underlying schema
// before marshaling so the user sees the actual shape rather than a pointer. JSON output is
// indented to drop straight into a `<pre>` block.
func buildExploreOp(doc *openapi.Document, method, path string, op *openapi.Operation) *exploreOp {
	eop := &exploreOp{
		Method:      strings.ToUpper(method),
		Path:        path,
		Summary:     op.Summary,
		Description: op.Description,
		Name:        op.XName,
		FeatureType: op.XFeatureType,
	}
	for _, p := range op.Parameters {
		if p == nil {
			continue
		}
		eop.Parameters = append(eop.Parameters, &exploreParam{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required,
			Description: p.Description,
			Schema:      schemaToJSON(doc, p.Schema),
		})
	}
	if op.RequestBody != nil {
		if media := op.RequestBody.Content["application/json"]; media != nil {
			eop.RequestBody = schemaToJSON(doc, media.Schema)
		}
	}
	codes := make([]string, 0, len(op.Responses))
	for code := range op.Responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	// Collapse responses with identical schemas into one row. Merged code list joined by
	// ", "; merged descriptions joined by " / ".
	bySchema := map[string]*exploreResp{}
	for _, code := range codes {
		resp := op.Responses[code]
		if resp == nil {
			continue
		}
		schema := ""
		if media := resp.Content["application/json"]; media != nil {
			schema = schemaToJSON(doc, media.Schema)
		}
		if schema != "" {
			if existing, ok := bySchema[schema]; ok {
				existing.Code += ", " + code
				if resp.Description != "" && !strings.Contains(existing.Description, resp.Description) {
					if existing.Description == "" {
						existing.Description = resp.Description
					} else {
						existing.Description += " / " + resp.Description
					}
				}
				continue
			}
		}
		er := &exploreResp{
			Code:        code,
			Description: resp.Description,
			Schema:      schema,
		}
		if schema != "" {
			bySchema[schema] = er
		}
		eop.Responses = append(eop.Responses, er)
	}
	return eop
}

// schemaToJSON returns an indented JSON sample object for a schema - what the user would
// send or receive on the wire - rather than the schema definition itself. Leaves are Go
// zero values; `$ref`s are followed recursively (cycles short-circuit at the second visit).
func schemaToJSON(doc *openapi.Document, schema *jsonschema.Schema) string {
	if schema == nil {
		return ""
	}
	var buf strings.Builder
	writeExample(&buf, doc, schema, map[string]bool{}, "")
	return buf.String()
}

// writeExample emits a sample JSON value for `schema` into `buf` with `indent` as the
// current line's leading whitespace. The walk is hand-written so property order matches
// the schema's declared order.
func writeExample(buf *strings.Builder, doc *openapi.Document, schema *jsonschema.Schema, visiting map[string]bool, indent string) {
	if schema == nil {
		buf.WriteString("null")
		return
	}
	if schema.Ref != "" {
		const prefix = "#/components/schemas/"
		if strings.HasPrefix(schema.Ref, prefix) && doc != nil && doc.Components != nil {
			key := schema.Ref[len(prefix):]
			if visiting[key] {
				buf.WriteString("null")
				return
			}
			if target, ok := doc.Components.Schemas[key]; ok && target != nil {
				visiting[key] = true
				writeExample(buf, doc, target, visiting, indent)
				delete(visiting, key)
				return
			}
		}
		buf.WriteString("null")
		return
	}
	switch schema.Type {
	case "object", "":
		writeExampleObject(buf, doc, schema, visiting, indent)
	case "array":
		writeExampleArray(buf, doc, schema, visiting, indent)
	case "string":
		// Date placeholders use Go's reference date so they read as idiomatic to anyone
		// who's seen a `time.Format` layout.
		switch schema.Format {
		case "date-time":
			buf.WriteString(`"2006-01-02T15:04:05Z"`)
		case "date":
			buf.WriteString(`"2006-01-02"`)
		default:
			buf.WriteString(`"string"`)
		}
	case "integer":
		buf.WriteString("0")
	case "number":
		buf.WriteString("0")
	case "boolean":
		buf.WriteString("false")
	case "null":
		buf.WriteString("null")
	default:
		buf.WriteString("null")
	}
}

// writeExampleObject walks the schema's declared properties in order and emits one indented
// `"key": <value>` line per property. When no fixed properties are declared but
// `additionalProperties` is, a single placeholder entry under `"key"` shows the value shape.
func writeExampleObject(buf *strings.Builder, doc *openapi.Document, schema *jsonschema.Schema, visiting map[string]bool, indent string) {
	type kv struct {
		key    string
		schema *jsonschema.Schema
	}
	var pairs []kv
	if schema.Properties != nil {
		for p := schema.Properties.Oldest(); p != nil; p = p.Next() {
			pairs = append(pairs, kv{p.Key, p.Value})
		}
	}
	if len(pairs) == 0 && schema.AdditionalProperties != nil {
		pairs = append(pairs, kv{"key", schema.AdditionalProperties})
	}
	if len(pairs) == 0 {
		buf.WriteString("{}")
		return
	}
	buf.WriteString("{\n")
	inner := indent + "  "
	for i, p := range pairs {
		buf.WriteString(inner)
		kb, _ := json.Marshal(p.key)
		buf.Write(kb)
		buf.WriteString(": ")
		writeExample(buf, doc, p.schema, visiting, inner)
		if i < len(pairs)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString(indent)
	buf.WriteByte('}')
}

// writeExampleArray emits a single-element array showing the item shape, or `[]` when no
// item schema is declared.
func writeExampleArray(buf *strings.Builder, doc *openapi.Document, schema *jsonschema.Schema, visiting map[string]bool, indent string) {
	if schema.Items == nil {
		buf.WriteString("[]")
		return
	}
	buf.WriteString("[\n")
	inner := indent + "  "
	buf.WriteString(inner)
	writeExample(buf, doc, schema.Items, visiting, inner)
	buf.WriteByte('\n')
	buf.WriteString(indent)
	buf.WriteByte(']')
}

// portMatches reports whether the OpenAPI path key matches the requested port. Path keys have
// the form `/host:port/...`; a path on `:0` is treated as a wildcard that matches any port.
func portMatches(path, port string) bool {
	p := pathPort(path)
	return p == port || p == "0"
}

// pathPort extracts the port segment from a `/host:port/...` path key. Returns "" if the path
// does not match that shape.
func pathPort(path string) string {
	rest := strings.TrimPrefix(path, "/")
	host, _, _ := strings.Cut(rest, "/")
	_, port, ok := strings.Cut(host, ":")
	if !ok {
		return ""
	}
	return port
}

// claimsDigest returns a short, stable digest of the caller's actor token to use as a cache
// key component. Anonymous callers map to a single shared key.
func claimsDigest(r *http.Request) string {
	actor := r.Header.Get(frame.HeaderActor)
	if actor == "" {
		return "anon"
	}
	sum := sha256.Sum256([]byte(actor))
	return hex.EncodeToString(sum[:8])
}

// writeJSON writes the body as application/json with Cache-Control: private, no-store.
func writeJSON(w http.ResponseWriter, body []byte) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_, err := w.Write(body)
	return errors.Trace(err)
}
