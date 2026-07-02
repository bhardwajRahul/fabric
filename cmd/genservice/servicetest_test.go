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
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

// TestCoveredMarkers pins that a feature is recognized as covered from the // MARKER comment on its test
// function, regardless of the test function's own name.
func TestCoveredMarkers(t *testing.T) {
	assert := testarossa.For(t)
	src := []byte(`package svc
func TestSvc_Greet(t *testing.T) { // MARKER: Greet
}
func TestSvc_OnChangedMaxItems(t *testing.T) { //MARKER:MaxItems
}
`)
	covered := coveredMarkers(src)
	assert.True(covered["Greet"])
	assert.True(covered["MaxItems"])
	assert.False(covered["Absent"])
}

// TestTestScaffoldViews pins the kind selection: callback-less configs and non-observable metrics
// contribute no scaffold, and the OnChanged/OnObserve prefixes land on the function name while the marker
// stays the bare feature name.
func TestTestScaffoldViews(t *testing.T) {
	assert := testarossa.For(t)
	callback := map[string]ast.Expr{"Callback": &ast.Ident{Name: "true"}}
	observable := map[string]ast.Expr{"Observable": &ast.Ident{Name: "true"}}
	svc := &service{
		name:   "Svc",
		apiPkg: "svcapi",
		features: []feature{
			{name: "Greet", kind: "Function"},
			{name: "MaxItems", kind: "Config", attrs: callback},
			{name: "APIKey", kind: "Config", attrs: map[string]ast.Expr{}},
			{name: "QueueDepth", kind: "Metric", attrs: observable},
			{name: "Hits", kind: "Metric", attrs: map[string]ast.Expr{}},
		},
	}
	views, err := testScaffoldViews(svc, "example.com/svc/svcapi")
	assert.NoError(err)
	assert.Equal(3, len(views))

	byMarker := map[string]scaffoldView{}
	for _, v := range views {
		byMarker[v.Marker] = v
	}
	assert.Equal("TestSvc_Greet", byMarker["Greet"].FuncName)
	assert.Equal("TestSvc_OnChangedMaxItems", byMarker["MaxItems"].FuncName)
	assert.Equal("SetMaxItems", byMarker["MaxItems"].Handler)
	assert.Equal("TestSvc_OnObserveQueueDepth", byMarker["QueueDepth"].FuncName)
	// The callback-less config and non-observable metric produced no scaffold.
	_, hasKey := byMarker["APIKey"]
	assert.False(hasKey)
	_, hasHits := byMarker["Hits"]
	assert.False(hasHits)
}

// TestMergeTestFileAppends pins the append path: an existing file keeps its hand-written test verbatim,
// gains the new function at the end, and picks up only the imports it lacks (foreman/foremanapi here)
// while leaving the ones it already declares untouched. The result must be valid, parseable Go.
func TestMergeTestFileAppends(t *testing.T) {
	assert := testarossa.For(t)
	orig := []byte(`package svc

import (
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
)

func TestSvc_Greet(t *testing.T) { // MARKER: Greet
	// hand-written, must survive verbatim
	_ = 42
}
`)
	funcs := []byte("func TestSvc_MainFlow(t *testing.T) { // MARKER: MainFlow\n}\n")
	needed := map[string]bool{
		impTesting: true, impApplication: true, impConnector: true,
		impForeman: true, impForemanAPI: true,
	}
	out, err := mergeTestFile(orig, needed, funcs)
	assert.NoError(err)

	// Valid Go.
	_, perr := parser.ParseFile(token.NewFileSet(), "", out, parser.ParseComments)
	assert.NoError(perr)

	s := string(out)
	// The hand-written test is preserved.
	assert.Contains(s, "// hand-written, must survive verbatim")
	// The new function is appended.
	assert.Contains(s, "func TestSvc_MainFlow(t *testing.T) { // MARKER: MainFlow")
	// The missing imports were added; the pre-existing ones are not duplicated.
	assert.Contains(s, impForeman)
	assert.Contains(s, impForemanAPI)
	assert.Equal(1, strings.Count(s, `"`+impConnector+`"`))
	assert.Equal(1, strings.Count(s, `"`+impApplication+`"`))
}
