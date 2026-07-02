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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

// TestHandlerMethodName pins the handler-name mapping and the callback/observable filters.
func TestHandlerMethodName(t *testing.T) {
	assert := testarossa.For(t)
	callback := map[string]ast.Expr{"Callback": &ast.Ident{Name: "true"}}
	observable := map[string]ast.Expr{"Observable": &ast.Ident{Name: "true"}}
	cases := []struct {
		f    feature
		name string
		ok   bool
	}{
		{feature{name: "Greet", kind: "Function"}, "Greet", true},
		{feature{name: "MainFlow", kind: "Workflow"}, "MainFlow", true},
		{feature{name: "MaxItems", kind: "Config", attrs: callback}, "OnChangedMaxItems", true},
		{feature{name: "APIKey", kind: "Config", attrs: map[string]ast.Expr{}}, "", false},
		{feature{name: "QueueDepth", kind: "Metric", attrs: observable}, "OnObserveQueueDepth", true},
		{feature{name: "Hits", kind: "Metric", attrs: map[string]ast.Expr{}}, "", false},
	}
	for _, c := range cases {
		name, _, ok := handlerMethodName(c.f)
		assert.Equal(c.ok, ok)
		assert.Equal(c.name, name)
	}
}

// TestEmitServiceHandlersStubsMissing pins the stub path: a service.go implementing only some of its
// features gains a compiling stub for each missing handler (with the projected signature, a TODO body,
// and a NewGraph starter for the workflow), keeps its existing handler untouched, and picks up the
// imports the stubs need.
func TestEmitServiceHandlersStubsMissing(t *testing.T) {
	assert := testarossa.For(t)
	dir := t.TempDir()
	serviceSrc := []byte(`package svc

import "context"

func (svc *Service) Ping(ctx context.Context) (err error) { return nil }
`)
	err := os.WriteFile(filepath.Join(dir, "service.go"), serviceSrc, 0o644)
	assert.NoError(err)

	svc := &service{
		apiPkg:  "svcapi",
		imports: map[string]string{},
		features: []feature{
			{name: "Ping", kind: "Function"},
			{name: "Reconcile", kind: "Ticker", doc: "Reconcile syncs state."},
			{name: "MainFlow", kind: "Workflow"},
		},
	}
	noSource := func(string) (*service, error) { return nil, nil }

	out, changed, err := emitServiceHandlers(dir, svc, "example.com/svc/svcapi", noSource, serviceSrc)
	assert.NoError(err)
	assert.True(changed)

	// Valid Go.
	_, perr := parser.ParseFile(token.NewFileSet(), "", out, parser.ParseComments)
	assert.NoError(perr)

	s := string(out)
	// The existing handler is not duplicated.
	assert.Equal(1, strings.Count(s, "func (svc *Service) Ping("))
	// The missing ticker handler is stubbed with its godoc, marker, and TODO body.
	assert.Contains(s, "func (svc *Service) Reconcile(ctx context.Context) (err error) { // MARKER: Reconcile")
	assert.Contains(s, "Reconcile syncs state.")
	assert.Contains(s, "// TODO: Implement Reconcile")
	// The workflow graph builder gets the NewGraph starter that fails validation until defined.
	assert.Contains(s, "func (svc *Service) MainFlow(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: MainFlow")
	assert.Contains(s, `graph = workflow.NewGraph("MainFlow")`)
	// The workflow package import was added.
	assert.Contains(s, impWorkflow)
}

// TestEmitServiceHandlersNoop pins that a fully implemented service regenerates to no change.
func TestEmitServiceHandlersNoop(t *testing.T) {
	assert := testarossa.For(t)
	dir := t.TempDir()
	serviceSrc := []byte(`package svc

import "context"

func (svc *Service) Ping(ctx context.Context) (err error) { return nil }
`)
	err := os.WriteFile(filepath.Join(dir, "service.go"), serviceSrc, 0o644)
	assert.NoError(err)

	svc := &service{
		apiPkg:   "svcapi",
		imports:  map[string]string{},
		features: []feature{{name: "Ping", kind: "Function"}},
	}
	noSource := func(string) (*service, error) { return nil, nil }

	out, changed, err := emitServiceHandlers(dir, svc, "example.com/svc/svcapi", noSource, serviceSrc)
	assert.NoError(err)
	assert.False(changed)
	assert.Equal(string(serviceSrc), string(out))
}
