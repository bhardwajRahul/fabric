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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/microbus-io/errors"
	"go.yaml.in/yaml/v3"
)

// oasDocument is the ingest model of an OpenAPI 3.x document. It captures only
// the subset needed to wrap an API one-for-one. Schema bodies are kept as raw
// JSON so that OpenAPI keywords this tool does not interpret (nullable,
// discriminator, example) survive untouched into the specs file.
type oasDocument struct {
	OpenAPI    string                 `json:"openapi"`
	Info       oasInfo                `json:"info"`
	Servers    []oasServer            `json:"servers"`
	Paths      map[string]oasPathItem `json:"paths"`
	Components oasComponents          `json:"components"`
	Security   []map[string][]string  `json:"security"`
}

type oasInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type oasServer struct {
	URL string `json:"url"`
}

// oasPathItem holds the operations on a path plus parameters shared by all of
// them. The shared parameters are merged into each operation during the build.
type oasPathItem struct {
	Parameters []oasParameter `json:"parameters"`
	Get        *oasOperation  `json:"get"`
	Put        *oasOperation  `json:"put"`
	Post       *oasOperation  `json:"post"`
	Delete     *oasOperation  `json:"delete"`
	Options    *oasOperation  `json:"options"`
	Head       *oasOperation  `json:"head"`
	Patch      *oasOperation  `json:"patch"`
	Trace      *oasOperation  `json:"trace"`
}

// operations returns the path item's operations keyed by uppercase HTTP method,
// in a fixed order so that the generated specs are deterministic.
func (pi oasPathItem) operations() []struct {
	method string
	op     *oasOperation
} {
	ordered := []struct {
		method string
		op     *oasOperation
	}{
		{"GET", pi.Get},
		{"PUT", pi.Put},
		{"POST", pi.Post},
		{"DELETE", pi.Delete},
		{"OPTIONS", pi.Options},
		{"HEAD", pi.Head},
		{"PATCH", pi.Patch},
		{"TRACE", pi.Trace},
	}
	out := ordered[:0]
	for _, e := range ordered {
		if e.op != nil {
			out = append(out, e)
		}
	}
	return out
}

type oasOperation struct {
	OperationID string                 `json:"operationId"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Parameters  []oasParameter         `json:"parameters"`
	RequestBody *oasRequestBody        `json:"requestBody"`
	Responses   map[string]oasResponse `json:"responses"`
	Security    []map[string][]string  `json:"security"`
}

type oasParameter struct {
	Ref         string          `json:"$ref"`
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Description string          `json:"description"`
	Required    bool            `json:"required"`
	Schema      json.RawMessage `json:"schema"`
}

type oasRequestBody struct {
	Ref         string                  `json:"$ref"`
	Description string                  `json:"description"`
	Required    bool                    `json:"required"`
	Content     map[string]oasMediaType `json:"content"`
}

type oasResponse struct {
	Ref         string                  `json:"$ref"`
	Description string                  `json:"description"`
	Content     map[string]oasMediaType `json:"content"`
}

type oasMediaType struct {
	Schema json.RawMessage `json:"schema"`
}

type oasComponents struct {
	Schemas         map[string]json.RawMessage   `json:"schemas"`
	Parameters      map[string]oasParameter      `json:"parameters"`
	RequestBodies   map[string]oasRequestBody    `json:"requestBodies"`
	Responses       map[string]oasResponse       `json:"responses"`
	SecuritySchemes map[string]oasSecurityScheme `json:"securitySchemes"`
}

type oasSecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat"`
	In           string `json:"in"`
	Name         string `json:"name"`
}

// parseDocument decodes raw, which may be JSON or YAML, into the ingest model.
func parseDocument(raw []byte) (*oasDocument, error) {
	j, err := toJSON(raw)
	if err != nil {
		return nil, err
	}
	var doc oasDocument
	err = json.Unmarshal(j, &doc)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if doc.OpenAPI == "" {
		return nil, errors.New("not an OpenAPI document: missing 'openapi' version field")
	}
	return &doc, nil
}

// toJSON returns raw as JSON. Input already starting with '{' is treated as JSON;
// anything else is decoded as YAML and re-encoded as JSON.
func toJSON(raw []byte) ([]byte, error) {
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return raw, nil
	}
	var tree any
	err := yaml.Unmarshal(raw, &tree)
	if err != nil {
		return nil, errors.Trace(err)
	}
	j, err := json.Marshal(tree)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return j, nil
}

// resolveParameter follows a "#/components/parameters/Name" reference, if any.
func (d *oasDocument) resolveParameter(p oasParameter) oasParameter {
	if p.Ref == "" {
		return p
	}
	name, ok := refName(p.Ref, "parameters")
	if !ok {
		return p
	}
	return d.Components.Parameters[name]
}

// resolveRequestBody follows a "#/components/requestBodies/Name" reference.
func (d *oasDocument) resolveRequestBody(b *oasRequestBody) *oasRequestBody {
	if b == nil || b.Ref == "" {
		return b
	}
	name, ok := refName(b.Ref, "requestBodies")
	if !ok {
		return b
	}
	rb := d.Components.RequestBodies[name]
	return &rb
}

// resolveResponse follows a "#/components/responses/Name" reference.
func (d *oasDocument) resolveResponse(r oasResponse) oasResponse {
	if r.Ref == "" {
		return r
	}
	name, ok := refName(r.Ref, "responses")
	if !ok {
		return r
	}
	return d.Components.Responses[name]
}

// refName extracts "Name" from a "#/components/<kind>/Name" reference.
func refName(ref, kind string) (string, bool) {
	prefix := "#/components/" + kind + "/"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	return strings.TrimPrefix(ref, prefix), true
}
