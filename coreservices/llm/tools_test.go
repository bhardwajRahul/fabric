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
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/openapi"
)

func TestCanonicalizeToolURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		hostPort string
		docKey   string
	}{
		{
			name:     "explicit https port 443",
			raw:      "https://calculator.example:443/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "missing port defaults to 443",
			raw:      "https://calculator.example/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "missing scheme and port",
			raw:      "calculator.example/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "http scheme defaults to 80",
			raw:      "http://calculator.example/arithmetic",
			hostPort: "calculator.example:80",
			docKey:   "/calculator.example:80/arithmetic",
		},
		{
			name:     "non-default internal port preserved",
			raw:      "https://calc.example:444/foo",
			hostPort: "calc.example:444",
			docKey:   "/calc.example:444/foo",
		},
		{
			name:     "named path arg passes through",
			raw:      "https://yellowpages.example:443/persons/{key}",
			hostPort: "yellowpages.example:443",
			docKey:   "/yellowpages.example:443/persons/{key}",
		},
		{
			name:     "greedy path arg has dots stripped",
			raw:      "https://files.example:443/load/{path...}",
			hostPort: "files.example:443",
			docKey:   "/files.example:443/load/{path}",
		},
		{
			name:     "greedy path arg mid-route",
			raw:      "https://files.example:443/load/{category}/{name...}",
			hostPort: "files.example:443",
			docKey:   "/files.example:443/load/{category}/{name}",
		},
		{
			name:     "anonymous path arg gets implicit name",
			raw:      "https://svc.example:443/path/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/path/{path1}",
		},
		{
			name:     "multiple anonymous path args numbered in order",
			raw:      "https://svc.example:443/path/{}/sub/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/path/{path1}/sub/{path2}",
		},
		{
			name:     "anonymous index counts all path args",
			raw:      "https://svc.example:443/a/{x}/b/{}/c/{y}/d/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/a/{x}/b/{path2}/c/{y}/d/{path4}",
		},
		{
			name:     "empty path becomes root",
			raw:      "https://svc.example:443",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			hostPort, docKey, err := canonicalizeToolURL(tc.raw)
			assert.NoError(err)
			assert.Equal(tc.hostPort, hostPort)
			assert.Equal(tc.docKey, docKey)
		})
	}
}

func TestCanonicalizeToolURL_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
	}{
		{name: "missing hostname", raw: ":443/path"},
		{name: "empty string", raw: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			_, _, err := canonicalizeToolURL(tc.raw)
			assert.Error(err)
		})
	}
}

func TestResolveSchemaRef(t *testing.T) {
	t.Parallel()

	// terminal is the fully-typed object schema every multi-hop chain should land on.
	terminal := &jsonschema.Schema{Type: "object"}
	props := jsonschema.NewProperties()
	props.Set("x", &jsonschema.Schema{Type: "integer"})
	terminal.Properties = props

	// wrapper points at terminal (the two-hop case from the Promotron bug: reflected
	// request bodies land as Components.Schemas["FetchURL_IN"] = {$ref to
	// "FetchURL_IN_FetchURLIn"}, with the typed object only at the deeper key).
	wrapper := &jsonschema.Schema{Ref: "#/components/schemas/terminal"}

	// mid -> wrapper -> terminal: lets the "three hops" test enter via {Ref: ".../mid"}.
	mid := &jsonschema.Schema{Ref: "#/components/schemas/wrapper"}

	// cycleA <-> cycleB (the cycle-guard case).
	cycleA := &jsonschema.Schema{Ref: "#/components/schemas/cycleB"}
	cycleB := &jsonschema.Schema{Ref: "#/components/schemas/cycleA"}

	doc := &openapi.Document{
		Components: &openapi.Components{
			Schemas: map[string]*jsonschema.Schema{
				"terminal": terminal,
				"wrapper":  wrapper,
				"mid":      mid,
				"cycleA":   cycleA,
				"cycleB":   cycleB,
			},
		},
	}

	t.Run("nil schema returns nil", func(t *testing.T) {
		assert := testarossa.For(t)
		assert.Nil(resolveSchemaRef(nil, doc))
	})

	t.Run("nil doc returns input untouched", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/components/schemas/terminal"}
		got := resolveSchemaRef(in, nil)
		assert.Equal(in, got)
	})

	t.Run("schema without ref returns input untouched", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Type: "object"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(in, got)
	})

	t.Run("non-components prefix returns input untouched", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/$defs/Foo"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(in, got)
	})

	t.Run("missing component key returns input untouched", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/components/schemas/notpresent"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(in, got)
	})

	t.Run("single hop resolves to terminal", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/components/schemas/terminal"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(terminal, got)
		assert.Equal("object", got.Type)
	})

	t.Run("two hops resolves through wrapper to terminal", func(t *testing.T) {
		assert := testarossa.For(t)
		// This is the Promotron regression: a wrapper schema whose only field is
		// `$ref` would previously be returned as-is, and Claude rejected the tool
		// payload with `input_schema.type: Field required`.
		in := &jsonschema.Schema{Ref: "#/components/schemas/wrapper"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(terminal, got)
		assert.Equal("object", got.Type)
	})

	t.Run("three hops resolves to terminal", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/components/schemas/mid"}
		got := resolveSchemaRef(in, doc)
		assert.Equal(terminal, got)
		assert.Equal("object", got.Type)
	})

	t.Run("cycle guard stops without looping forever", func(t *testing.T) {
		assert := testarossa.For(t)
		in := &jsonschema.Schema{Ref: "#/components/schemas/cycleA"}
		got := resolveSchemaRef(in, doc)
		// The resolver visited cycleA -> cycleB and bailed before re-entering cycleA.
		// The returned schema is the last node in the chain (cycleB), which still
		// has Ref set - callers see this case as "unresolvable" rather than hanging.
		assert.NotNil(got)
		assert.NotEqual("", got.Ref)
	})

	t.Run("self-referential cycle stops at first revisit", func(t *testing.T) {
		assert := testarossa.For(t)
		self := &jsonschema.Schema{Ref: "#/components/schemas/self"}
		selfDoc := &openapi.Document{
			Components: &openapi.Components{
				Schemas: map[string]*jsonschema.Schema{"self": self},
			},
		}
		got := resolveSchemaRef(self, selfDoc)
		assert.NotNil(got)
		assert.NotEqual("", got.Ref)
	})
}
