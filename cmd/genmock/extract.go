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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// param is a single function parameter or result.
type param struct {
	name string
	typ  string // Go source rendering of the type
}

// method is one entry of the ToDo interface, excluding OnStartup/OnShutdown.
type method struct {
	name    string  // Go method name (matches intermediate.go)
	marker  string  // MARKER: <X> comment text, defaults to name
	params  []param // including ctx and any other inputs, in declaration order
	results []param // including the trailing err, in declaration order

	// Convenience fields derived from params/results.
	isWeb      bool // signature is (w http.ResponseWriter, r *http.Request) (err error)
	isGraph    bool // signature is (ctx context.Context) (graph *workflow.Graph, err error)
	resultsHas struct {
		flow bool // any param is *workflow.Flow (a task method)
	}
}

// generated bundles everything emit needs.
type generated struct {
	pkgName    string            // Go package name of the service directory
	apiPkg     string            // <svc>api import alias (the package's own api)
	apiPath    string            // full import path of the <svc>api package
	imports    map[string]string // import path -> alias (alias "" means default)
	methods    []method
	graphPairs map[string]inOutPair // base name -> In/Out fields (for workflow graphs)
}

// inOutPair carries the In/Out struct field shapes for one workflow graph.
type inOutPair struct {
	in  []structField
	out []structField
}

// structField describes one field of an In/Out struct.
type structField struct {
	goName string // Go field name, first letter lowercased (e.g. ListMessagesOut -> listMessagesOut)
	typ    string // Go source rendering of the type
	rawGo  string // raw Go field name as declared (e.g. ListMessagesOut)
}

// generate produces the data needed to emit mock.go for the microservice at dir.
func generate(dir string) (*generated, error) {
	intermediatePath := filepath.Join(dir, "intermediate.go")
	f, _, err := parseFile(intermediatePath)
	if err != nil {
		return nil, fmt.Errorf("parse intermediate.go: %w", err)
	}

	g := &generated{
		pkgName: f.Name.Name,
		imports: map[string]string{},
	}

	// Map alias -> import path from intermediate.go's import block; pick out the
	// <svc>api package (the one whose path ends in `/<pkgName>api` or whose
	// alias matches that name).
	intAliasPath := importMap(f)
	for alias, path := range intAliasPath {
		if isOwnAPIPath(path, g.pkgName) {
			g.apiPkg = alias
			g.apiPath = path
			break
		}
	}

	// Collect ToDo methods, skipping OnStartup/OnShutdown (which the mock
	// provides directly).
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "ToDo" {
				continue
			}
			it, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range it.Methods.List {
				if len(m.Names) == 0 {
					continue
				}
				name := m.Names[0].Name
				if name == "OnStartup" || name == "OnShutdown" {
					continue
				}
				ft, ok := m.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				me := method{
					name:    name,
					marker:  extractMarker(m.Comment),
					params:  paramsFromFields(ft.Params),
					results: normalizeResults(paramsFromFields(ft.Results)),
				}
				if me.marker == "" {
					me.marker = name
				}
				classify(&me)
				g.methods = append(g.methods, me)
			}
		}
	}

	// Walk every package referenced by ToDo methods. Each unfamiliar selector's
	// path is looked up in intermediate.go's import block.
	collectImports(g, intAliasPath)

	// For workflow graphs, parse the api package's In/Out struct shapes so we
	// can render the typed mock handler signature.
	if hasGraph(g.methods) {
		pairs, err := parseInOutStructs(filepath.Join(dir, lastPathSegment(g.apiPath)))
		if err != nil {
			return nil, fmt.Errorf("parse api package: %w", err)
		}
		g.graphPairs = pairs
	}

	return g, nil
}

func hasGraph(ms []method) bool {
	for _, m := range ms {
		if m.isGraph {
			return true
		}
	}
	return false
}

// paramsFromFields flattens an ast.FieldList. Anonymous (unnamed) entries are
// kept with an empty name so that result-only `error` returns survive.
func paramsFromFields(fl *ast.FieldList) []param {
	if fl == nil {
		return nil
	}
	var out []param
	for _, field := range fl.List {
		typ := exprString(field.Type)
		if len(field.Names) == 0 {
			out = append(out, param{name: "", typ: typ})
			continue
		}
		for _, n := range field.Names {
			out = append(out, param{name: n.Name, typ: typ})
		}
	}
	return out
}

// normalizeResults renames an unnamed trailing `error` result to `err`, which
// matches what the mock body expects ToDo signatures like
// `Foo(...) error` and `Foo(...) (err error)` are treated identically.
func normalizeResults(rs []param) []param {
	out := make([]param, len(rs))
	copy(out, rs)
	for i := range out {
		if out[i].name == "" && out[i].typ == "error" {
			out[i].name = "err"
		}
	}
	return out
}

// classify sets the convenience flags on m based on its signature.
func classify(m *method) {
	// Web handler: (w http.ResponseWriter, r *http.Request) (err error).
	if len(m.params) == 2 &&
		m.params[0].typ == "http.ResponseWriter" &&
		m.params[1].typ == "*http.Request" {
		m.isWeb = true
	}
	// Workflow graph: (ctx context.Context) (graph *workflow.Graph, err error).
	if len(m.params) == 1 && m.params[0].typ == "context.Context" &&
		len(m.results) == 2 && m.results[0].typ == "*workflow.Graph" {
		m.isGraph = true
	}
	// Flow-bearing (task or convenience): any param is *workflow.Flow.
	for _, p := range m.params {
		if p.typ == "*workflow.Flow" {
			m.resultsHas.flow = true
			break
		}
	}
}

// extractMarker pulls "MARKER: Foo" out of a comment group attached to a ToDo
// method. Returns "" if not present.
func extractMarker(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	for _, c := range g.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		text = strings.TrimSuffix(text, "*/")
		idx := strings.Index(text, "MARKER:")
		if idx < 0 {
			continue
		}
		marker := strings.TrimSpace(text[idx+len("MARKER:"):])
		if i := strings.IndexAny(marker, " \t"); i >= 0 {
			marker = marker[:i]
		}
		return marker
	}
	return ""
}

// importMap returns the alias -> path mapping for f's import block. For
// imports without an explicit alias, the alias is the last segment of the path
// (after stripping a trailing `apiPkg` lookup, the api packages declare
// `package fooapi` so the alias equals the path's last segment).
func importMap(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		path := unquote(imp.Path.Value)
		alias := lastPathSegment(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		out[alias] = path
	}
	return out
}

func isOwnAPIPath(path, pkgName string) bool {
	last := lastPathSegment(path)
	return last == pkgName+"api"
}

func lastPathSegment(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

// collectImports walks every method signature, collects the set of selector
// prefixes referenced (e.g. `time`, `http`, `bearertokenapi`), and resolves
// each to its import path via intermediate.go's import block.
func collectImports(g *generated, intMap map[string]string) {
	// Always-required imports: context, connector, errors.
	g.imports["context"] = ""
	g.imports["github.com/microbus-io/fabric/connector"] = ""
	g.imports["github.com/microbus-io/errors"] = ""

	// Workflow-graph mocks pull in extra packages for the subgraph machinery.
	if hasGraph(g.methods) {
		g.imports["net/http"] = ""
		g.imports["encoding/json"] = ""
		g.imports["github.com/microbus-io/fabric/httpx"] = ""
		g.imports["github.com/microbus-io/fabric/sub"] = ""
		g.imports["github.com/microbus-io/fabric/utils"] = ""
		g.imports["github.com/microbus-io/fabric/workflow"] = ""
	}

	// Walk each method's signature and add the api package + any other
	// selector-based packages.
	add := func(typ string) {
		for _, sel := range selectorsIn(typ) {
			path, ok := intMap[sel]
			if !ok {
				continue
			}
			g.imports[path] = ""
		}
	}
	for _, m := range g.methods {
		for _, p := range m.params {
			add(p.typ)
		}
		for _, r := range m.results {
			add(r.typ)
		}
	}

	// The api package gets referenced as a type only when a ToDo signature uses
	// it. If the service has any ToDo method (function, web, task, ...), the
	// mock still references the api package indirectly via NewMock for some
	// services - but only when there's an actual symbol use. Adding it
	// unconditionally would re-introduce a dead import for the few services
	// (httpingress, control, metrics) whose ToDo doesn't use the api package.
	// So: only add when something in the signatures references it (already
	// handled above).
	if g.apiPath != "" && g.apiPkg != "" {
		// Workflow-graph mocks reference apiPkg.<Name>In / apiPkg.<Name>Out and
		// apiPkg.<Name>.URL() even if the ToDo signature doesn't.
		if hasGraph(g.methods) {
			g.imports[g.apiPath] = ""
		}
	}
}

// selectorsIn returns the set of `Pkg` prefixes in a Go type expression like
// `[]bearertokenapi.JWK` -> ["bearertokenapi"]; `map[string]workflow.Flow` -> ["workflow"].
func selectorsIn(typ string) []string {
	var out []string
	seen := map[string]bool{}
	i := 0
	for i < len(typ) {
		c := typ[i]
		if !isIdentStart(c) {
			i++
			continue
		}
		// Scan an identifier.
		j := i
		for j < len(typ) && isIdentCont(typ[j]) {
			j++
		}
		ident := typ[i:j]
		// A selector means the next char is '.' followed by another identifier.
		if j < len(typ) && typ[j] == '.' {
			if !seen[ident] {
				seen[ident] = true
				out = append(out, ident)
			}
		}
		i = j + 1
	}
	return out
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// parseInOutStructs walks every .go file in apiDir and gathers In/Out struct
// shapes. Result keys are the endpoint base name (e.g. "ChatLoop").
func parseInOutStructs(apiDir string) (map[string]inOutPair, error) {
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		return nil, err
	}
	pairs := map[string]inOutPair{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, _, err := parseFile(filepath.Join(apiDir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				name := ts.Name.Name
				var base, suffix string
				switch {
				case strings.HasSuffix(name, "In"):
					base = name[:len(name)-2]
					suffix = "in"
				case strings.HasSuffix(name, "Out"):
					base = name[:len(name)-3]
					suffix = "out"
				default:
					continue
				}
				fields := flattenStructFields(st)
				pair := pairs[base]
				if suffix == "in" {
					pair.in = fields
				} else {
					pair.out = fields
				}
				pairs[base] = pair
			}
		}
	}
	return pairs, nil
}

func flattenStructFields(st *ast.StructType) []structField {
	var out []structField
	if st.Fields == nil {
		return out
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		typeStr := exprString(field.Type)
		for _, n := range field.Names {
			out = append(out, structField{
				goName: lowerFirst(n.Name),
				rawGo:  n.Name,
				typ:    typeStr,
			})
		}
	}
	return out
}

// parseFile is a thin wrapper around go/parser that always includes comments.
func parseFile(path string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return f, fset, nil
}

// exprString renders a type expression as Go source. Supports the shapes the
// framework uses in ToDo signatures.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "any"
		}
		return "interface{...}"
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.FuncType:
		// Minimal rendering for func types.
		var sb strings.Builder
		sb.WriteString("func(")
		if t.Params != nil {
			for i, p := range t.Params.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(exprString(p.Type))
			}
		}
		sb.WriteString(")")
		return sb.String()
	default:
		return fmt.Sprintf("%T", e)
	}
}

// unquote unwraps a Go string literal.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	if v, err := strconv.Unquote(s); err == nil {
		return v
	}
	return s[1 : len(s)-1]
}

// lowerFirst lowercases the leading run of capital letters in a name, matching
// the convention used by genmanifest (so `URLPath` -> `urlPath`, `URL` -> `url`).
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if !isUpperRune(r[0]) {
		return s
	}
	end := 1
	for end < len(r) && isUpperRune(r[end]) {
		end++
	}
	if end < len(r) {
		end--
		if end < 1 {
			end = 1
		}
	}
	for i := 0; i < end; i++ {
		r[i] = toLowerRune(r[i])
	}
	return string(r)
}

func isUpperRune(r rune) bool { return r >= 'A' && r <= 'Z' }
func toLowerRune(r rune) rune {
	if isUpperRune(r) {
		return r + ('a' - 'A')
	}
	return r
}
