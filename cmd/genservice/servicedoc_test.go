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
	"testing"

	"github.com/microbus-io/testarossa"
)

// TestHandlerDocs pins the mapping from features to the godoc body of their *Service handler: a bare
// description for name-matched kinds, and a genservice-fixed first line plus a description paragraph for
// an observable metric's OnObserveXxx and a callback config's OnChangedXxx.
func TestHandlerDocs(t *testing.T) {
	assert := testarossa.For(t)
	truthy := map[string]ast.Expr{"Observable": &ast.Ident{Name: "true"}}
	callback := map[string]ast.Expr{"Callback": &ast.Ident{Name: "true"}}
	svc := &service{features: []feature{
		{name: "Greet", kind: "Function", doc: "Greet returns a greeting."},
		{name: "Nodoc", kind: "Function", doc: ""},
		{name: "QueueDepth", kind: "Metric", doc: "QueueDepth records the depth.", attrs: truthy},
		{name: "Plain", kind: "Metric", doc: "Plain is not observable.", attrs: map[string]ast.Expr{}},
		{name: "MaxItems", kind: "Config", doc: "MaxItems caps items.", attrs: callback},
		{name: "APIKey", kind: "Config", doc: "APIKey is a secret.", attrs: map[string]ast.Expr{}},
	}}
	docs := handlerDocs(svc)

	assert.Equal("Greet returns a greeting.", docs["Greet"])
	// A feature with no description contributes no entry for its name-matched handler.
	_, ok := docs["Nodoc"]
	assert.False(ok)
	// Observable metric: fixed first line, blank line, then the description.
	assert.Equal("OnObserveQueueDepth emits the observed value of the QueueDepth metric.\n\nQueueDepth records the depth.",
		docs["OnObserveQueueDepth"])
	// Callback config: fixed first line, blank line, then the description.
	assert.Equal("OnChangedMaxItems is called when the MaxItems config property changes.\n\nMaxItems caps items.",
		docs["OnChangedMaxItems"])
	// A non-observable metric and a non-callback config have no handler entry.
	_, ok = docs["OnObservePlain"]
	assert.False(ok)
	_, ok = docs["OnChangedAPIKey"]
	assert.False(ok)
}

// TestSyncHandlerDocs pins the doc-sync rules that the goldens do not exercise: replacing a // comment,
// inserting where none exists, multi-line block bodies, and leaving unmatched or non-Service methods
// alone.
func TestSyncHandlerDocs(t *testing.T) {
	assert := testarossa.For(t)
	src := `package svc

// Greet stale comment.
func (svc *Service) Greet(ctx context.Context) (err error) { return nil }

func (svc *Service) Adopt(ctx context.Context) (err error) { return nil }

// Helper is not a feature.
func (svc *Service) Helper(ctx context.Context) (err error) { return nil }

// Greet on another receiver must be left alone.
func (other *Other) Greet(ctx context.Context) (err error) { return nil }
`
	docs := map[string]string{
		"Greet": "Greet returns a greeting.\nSecond line.",
		"Adopt": "Adopt registers a pet.",
	}
	out, err := syncHandlerDocs([]byte(src), docs)
	assert.NoError(err)
	got := string(out)

	// The // doc is replaced by a /* */ block carrying the new multi-line text.
	assert.Contains(got, "/*\nGreet returns a greeting.\nSecond line.\n*/\nfunc (svc *Service) Greet")
	assert.NotContains(got, "// Greet stale comment.")
	// A handler with no prior doc gets one inserted.
	assert.Contains(got, "/*\nAdopt registers a pet.\n*/\nfunc (svc *Service) Adopt")
	// A *Service method with no matching feature is untouched.
	assert.Contains(got, "// Helper is not a feature.")
	// A same-named method on a different receiver is untouched.
	assert.Contains(got, "// Greet on another receiver must be left alone.")

	// Re-running is a no-op: the output is already in sync.
	again, err := syncHandlerDocs(out, docs)
	assert.NoError(err)
	assert.Equal(string(out), string(again))

	// Empty docs leaves the source verbatim.
	untouched, err := syncHandlerDocs([]byte(src), map[string]string{})
	assert.NoError(err)
	assert.Equal(src, string(untouched))
}
