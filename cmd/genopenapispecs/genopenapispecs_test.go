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
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

// updateGoldens, when set, rewrites the committed specs golden in
// cmd/genopenapispecs/testdata/ from the live output instead of comparing
// against it. Use after an intentional change to the transform:
//
//	go test ./cmd/genopenapispecs/ -update
//
// Then review the diff in `git status` and commit the regenerated file.
var updateGoldens = flag.Bool("update", false, "rewrite committed specs golden from transform output")

func TestPascal(t *testing.T) {
	assert := testarossa.For(t)
	cases := []struct{ in, want string }{
		{"listPets", "ListPets"},
		{"list_pets", "ListPets"},
		{"get-pet-by-id", "GetPetById"},
		{"GetPetByID", "GetPetByID"},
		{"create.pet.v2", "CreatePetV2"},
		{"123abc", "123abc"},
		{"a", "A"},
		{"", ""},
		{"  spaced  out ", "SpacedOut"},
	}
	for _, c := range cases {
		assert.Equal(c.want, pascal(c.in), "pascal(%q)", c.in)
	}
}

func TestEndpointName(t *testing.T) {
	assert := testarossa.For(t)
	cases := []struct {
		operationID, method, path, want string
	}{
		{"listPets", "GET", "/pets", "ListPets"},
		{"", "GET", "/pets/{petId}/avatar", "GetPetsPetIdAvatar"},
		{"", "POST", "/", "Post"},
		{"", "GET", "/{greedy...}", "GetGreedy"},
	}
	for _, c := range cases {
		got := endpointName(c.operationID, c.method, c.path)
		assert.Equal(c.want, got, "endpointName(%q,%q,%q)", c.operationID, c.method, c.path)
	}
}

func TestPickContent(t *testing.T) {
	assert := testarossa.For(t)

	ct, schema := pickContent(map[string]oasMediaType{
		"application/json": {Schema: json.RawMessage(`{"type":"object"}`)},
	})
	assert.Equal("application/json", ct, "json content type")
	assert.NotNil(schema, "json schema present")

	ct, schema = pickContent(map[string]oasMediaType{
		"application/hal+json": {Schema: json.RawMessage(`{"type":"object"}`)},
	})
	assert.Equal("application/hal+json", ct, "+json suffix matched")
	assert.NotNil(schema, "hal+json schema present")

	ct, schema = pickContent(map[string]oasMediaType{"image/png": {}})
	assert.Equal("image/png", ct, "non-json content type surfaced")
	assert.Nil(schema, "non-json has no schema")

	ct, schema = pickContent(nil)
	assert.Equal("", ct, "empty content type for absent body")
	assert.Nil(schema, "no schema for absent body")
}

func TestGoScalarType(t *testing.T) {
	assert := testarossa.For(t)
	cases := []struct{ schema, want string }{
		{`{"type":"integer","format":"int64"}`, "int64"},
		{`{"type":"integer"}`, "int"},
		{`{"type":"number"}`, "float64"},
		{`{"type":"boolean"}`, "bool"},
		{`{"type":"string"}`, "string"},
		{`{"type":"string","format":"date-time"}`, "time.Time"},
		{`{"type":"array","items":{"type":"string"}}`, "[]string"},
		{`{"type":"array","items":{"type":"integer","format":"int64"}}`, "[]int64"},
		{`{"$ref":"#/components/schemas/Pet"}`, ""},
		{`{"type":"object"}`, ""},
		{`{"type":"array","items":{"$ref":"#/components/schemas/Pet"}}`, ""},
		{``, ""},
	}
	for _, c := range cases {
		assert.Equal(c.want, goScalarType(json.RawMessage(c.schema)), "goScalarType(%s)", c.schema)
	}
}

func TestStripRefPrefix(t *testing.T) {
	assert := testarossa.For(t)

	got := string(stripRefPrefix(json.RawMessage(`{"$ref":"#/components/schemas/Pet"}`)))
	assert.Equal(`{"$ref":"Pet"}`, got, "top-level ref stripped")

	got = string(stripRefPrefix(json.RawMessage(
		`{"type":"object","properties":{"cat":{"$ref":"#/components/schemas/Category"}},` +
			`"tags":{"type":"array","items":{"$ref":"#/components/schemas/Tag"}}}`)))
	assert.Contains(got, `"$ref":"Category"`, "nested ref stripped")
	assert.Contains(got, `"$ref":"Tag"`, "array item ref stripped")
	assert.NotContains(got, "#/components/schemas/", "no prefix remains")

	// A ref that is not a schema pointer is left untouched.
	got = string(stripRefPrefix(json.RawMessage(`{"$ref":"#/components/parameters/Foo"}`)))
	assert.Equal(`{"$ref":"#/components/parameters/Foo"}`, got, "non-schema ref untouched")

	assert.Equal("", string(stripRefPrefix(nil)), "empty stays empty")
}

func TestClassify(t *testing.T) {
	assert := testarossa.For(t)

	jsonBody := &SpecsBody{ContentType: "application/json", Schema: json.RawMessage(`{}`)}
	rawBody := &SpecsBody{ContentType: "multipart/form-data"}
	jsonResp := &SpecsResponse{Status: 200, ContentType: "application/json", Schema: json.RawMessage(`{}`)}
	binResp := &SpecsResponse{Status: 200, ContentType: "image/png"}
	noResp := &SpecsResponse{Status: 204}

	assert.Equal("function", classify(SpecsEndpoint{RequestBody: jsonBody, Response: jsonResp}), "json in/out")
	assert.Equal("function", classify(SpecsEndpoint{Response: noResp}), "body-less")
	assert.Equal("function", classify(SpecsEndpoint{}), "no body at all")
	assert.Equal("web", classify(SpecsEndpoint{RequestBody: rawBody}), "non-json upload")
	assert.Equal("web", classify(SpecsEndpoint{Response: binResp}), "binary download")
}

func TestResolveSecurity(t *testing.T) {
	assert := testarossa.For(t)
	mk := func(scheme oasSecurityScheme) *oasDocument {
		return &oasDocument{
			Security:   []map[string][]string{{"s": {}}},
			Components: oasComponents{SecuritySchemes: map[string]oasSecurityScheme{"s": scheme}},
		}
	}

	got := resolveSecurity(mk(oasSecurityScheme{Type: "apiKey", In: "query", Name: "api_key"}))
	assert.Equal(&SpecsSecurity{Type: "apiKey", In: "query", Name: "api_key"}, got, "apiKey")

	got = resolveSecurity(mk(oasSecurityScheme{Type: "http", Scheme: "bearer"}))
	assert.Equal(&SpecsSecurity{Type: "http-bearer", In: "header", Name: "Authorization"}, got, "bearer")

	got = resolveSecurity(mk(oasSecurityScheme{Type: "http", Scheme: "basic"}))
	assert.Equal(&SpecsSecurity{Type: "http-basic", In: "header", Name: "Authorization"}, got, "basic")

	got = resolveSecurity(mk(oasSecurityScheme{Type: "oauth2"}))
	assert.Equal(&SpecsSecurity{Type: "oauth2", In: "header", Name: "Authorization"}, got, "oauth2")

	assert.Nil(resolveSecurity(&oasDocument{}), "no scheme")

	// No document-level security: the scheme operations reference most often
	// wins, even though it sorts after the rarely-referenced one alphabetically.
	perOp := &oasDocument{
		Components: oasComponents{SecuritySchemes: map[string]oasSecurityScheme{
			"akey":    {Type: "apiKey", In: "header", Name: "X-Key"},
			"zbearer": {Type: "http", Scheme: "bearer"},
		}},
		Paths: map[string]oasPathItem{
			"/a": {Get: &oasOperation{Security: []map[string][]string{{"zbearer": {}}}}},
			"/b": {
				Get:  &oasOperation{Security: []map[string][]string{{"zbearer": {}}}},
				Post: &oasOperation{Security: []map[string][]string{{"akey": {}}}},
			},
		},
	}
	got = resolveSecurity(perOp)
	assert.Equal(&SpecsSecurity{Type: "http-bearer", In: "header", Name: "Authorization"}, got,
		"most-referenced operation scheme beats alphabetical fallback")
}

func TestIsAbsoluteURL(t *testing.T) {
	assert := testarossa.For(t)
	cases := []struct {
		in   string
		want bool
	}{
		{"https://api.example.com/v1", true},
		{"http://localhost:8080", true},
		{"HTTPS://API.EXAMPLE.COM", true},
		{"/api/v3", false},
		{"api.example.com", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equal(c.want, isAbsoluteURL(c.in), "isAbsoluteURL(%q)", c.in)
	}
}

func TestRun_FilterAndWarning(t *testing.T) {
	assert := testarossa.For(t)
	doc := func(serverURL string) []byte {
		return []byte(`{"openapi":"3.0.0","info":{"title":"X"},` +
			`"servers":[{"url":"` + serverURL + `"}],` +
			`"paths":{"/p":{"get":{"operationId":"getP","responses":{"200":{"description":"ok"}}}}}}`)
	}

	var stdout, stderr bytes.Buffer
	err := run(doc("/api"), "", &stdout, &stderr)
	assert.NoError(err, "relative server still produces output")
	assert.Contains(stdout.String(), `"endpoints"`, "specs written to stdout")
	assert.Contains(stderr.String(), "not absolute", "relative base URL warned on stderr")

	stdout.Reset()
	stderr.Reset()
	err = run(doc("https://api.example.com/v1"), "", &stdout, &stderr)
	assert.NoError(err, "absolute server")
	assert.Equal("", strings.TrimSpace(stderr.String()), "absolute base URL produces no warning")

	stdout.Reset()
	stderr.Reset()
	err = run(doc("/api"), "https://override.example.com", &stdout, &stderr)
	assert.NoError(err, "base-url override")
	assert.Equal("", strings.TrimSpace(stderr.String()), "override silences the relative-URL warning")
}

func TestToJSON(t *testing.T) {
	assert := testarossa.For(t)

	j, err := toJSON([]byte(`  {"openapi":"3.0.3"}`))
	assert.NoError(err, "json passthrough")
	assert.Contains(string(j), `"openapi"`, "json preserved")

	j, err = toJSON([]byte("openapi: 3.0.3\ninfo:\n  title: X\n"))
	assert.NoError(err, "yaml decode")
	var m map[string]any
	assert.NoError(json.Unmarshal(j, &m), "yaml converted to valid json")
	assert.Equal("3.0.3", m["openapi"], "yaml field round-tripped")
}

func TestParseDocument_Invalid(t *testing.T) {
	assert := testarossa.For(t)
	_, err := parseDocument([]byte(`{"info":{"title":"X"}}`))
	assert.Error(err, "missing openapi version is rejected")
}

func TestBuildSpecs_NoOperations(t *testing.T) {
	assert := testarossa.For(t)
	_, err := buildSpecs(&oasDocument{OpenAPI: "3.0.3"}, "")
	assert.Error(err, "document with no operations is rejected")
}

// TestPetstoreGolden runs the full transform against the real-world-ish petstore
// fixture and compares the result to the committed golden. Run with -update to
// regenerate after an intentional transform change.
func TestPetstoreGolden(t *testing.T) {
	assert := testarossa.For(t)

	raw, err := os.ReadFile(filepath.Join("testdata", "petstore.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := parseDocument(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	specs, err := buildSpecs(doc, "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := specs.encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	goldenPath := filepath.Join("testdata", "petstore.specs.json")
	if *updateGoldens {
		err = os.WriteFile(goldenPath, out, 0o644)
		if err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update first): %v", err)
	}
	assert.Equal(string(want), string(out), "specs output drifted from golden")

	// Spot-check the salient transform decisions on top of the byte compare.
	byName := map[string]SpecsEndpoint{}
	for _, ep := range specs.Endpoints {
		byName[ep.Name] = ep
	}
	assert.Equal("function", byName["ListPets"].Feature, "ListPets is a function")
	assert.Equal("function", byName["CreatePet"].Feature, "CreatePet is a function")
	assert.Equal("web", byName["GetPetAvatar"].Feature, "binary avatar is a web handler")
	_, hasFallbackName := byName["GetPetsSearch"]
	assert.True(hasFallbackName, "operationId-less op named from method and path")
	assert.NotNil(byName["GetPetById"].Response, "ref response resolved")
	assert.Equal(201, byName["CreatePet"].Response.Status, "lowest 2xx selected")
	assert.Len(byName["GetPetById"].Params, 1, "path-level shared param merged")
	for _, p := range byName["GetPetsSearch"].Params {
		assert.NotEqual("session", p.Name, "cookie param dropped")
	}
	assert.Equal(&SpecsSecurity{Type: "apiKey", In: "header", Name: "X-Api-Key"}, specs.Remote.Security, "security resolved")
}
