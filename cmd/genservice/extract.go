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
	"go/printer"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// service is the model extracted from a microservice's api package definition.go (+ sibling files).
type service struct {
	apiPkg   string                // api package name, e.g. "svcapi"
	hostname string                // value of the Hostname const
	features []feature             // declared in source order
	structs  map[string]*structDef // In/Out and domain structs, by type name
	imports  map[string]string     // alias -> import path, from the api package files
	fset     *token.FileSet        // for rendering value expressions back to source
}

// feature is one define.* declaration.
type feature struct {
	name     string // the var name, e.g. "VerifyCredit"
	kind     string // Function | Web | Task | Workflow | OutboundEvent | InboundEvent | Config | Metric | Ticker
	doc      string // godoc on the var, cleaned; may be ""
	in       string // In struct type name; "" if the kind has none
	out      string // Out struct type name; "" if the kind has none
	srcPkg   string // InboundEvent only: the Source's package alias, e.g. "eventsourceapi"
	srcEvent string // InboundEvent only: the Source outbound event name, e.g. "OnRegistered"

	attrs map[string]ast.Expr // the literal's keyed fields (Method, RequiredClaims, Default, Kind, ...)
}

// structDef is a struct type and its (flattened) fields.
type structDef struct {
	name   string
	fields []fieldDef
}

// fieldDef is a single struct field.
type fieldDef struct {
	goName string // Go field name, e.g. "CreditScore" or "HTTPRequestBody"
	typ    string // rendered Go type, e.g. "int", "[]string", "*Pet"
}

// defineKinds is the set of define.* type names recognized as feature declarations.
var defineKinds = map[string]bool{
	"Function": true, "Web": true, "Task": true, "Workflow": true,
	"OutboundEvent": true, "InboundEvent": true, "Config": true, "Metric": true, "Ticker": true,
}

// parseService reads the api package in dir and builds the service model.
func parseService(dir string) (*service, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	svc := &service{
		structs: map[string]*structDef{},
		imports: map[string]string{},
		fset:    fset,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if svc.apiPkg == "" {
			svc.apiPkg = f.Name.Name
		}
		maps.Copy(svc.imports, importMap(f))
		collectStructs(svc, f)
		collectConsts(svc, f)
		collectFeatures(svc, f)
	}
	if svc.apiPkg == "" {
		return nil, fmt.Errorf("no Go package found in %s", dir)
	}
	return svc, nil
}

// collectStructs records every struct type definition and its fields.
func collectStructs(svc *service, f *ast.File) {
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
			sd := &structDef{name: ts.Name.Name}
			if st.Fields != nil {
				for _, field := range st.Fields.List {
					typ := exprString(field.Type)
					for _, n := range field.Names {
						sd.fields = append(sd.fields, fieldDef{goName: n.Name, typ: typ})
					}
				}
			}
			svc.structs[sd.name] = sd
		}
	}
}

// collectConsts records the Hostname const value.
func collectConsts(svc *service, f *ast.File) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				if n.Name == "Hostname" && i < len(vs.Values) {
					if bl, ok := vs.Values[i].(*ast.BasicLit); ok {
						svc.hostname = strings.Trim(bl.Value, "`\"")
					}
				}
			}
		}
	}
}

// collectFeatures records every var bound to a define.* composite literal.
func collectFeatures(svc *service, f *ast.File) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			cl, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			sel, ok := cl.Type.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "define" || !defineKinds[sel.Sel.Name] {
				continue
			}
			ft := feature{name: vs.Names[0].Name, kind: sel.Sel.Name, attrs: attrsOf(cl)}
			ft.doc = cleanDoc(vs.Doc, gen.Doc)
			ft.in, ft.out = inOutOf(cl)
			if ft.kind == "InboundEvent" {
				ft.srcPkg, ft.srcEvent = sourceOf(cl)
			}
			svc.features = append(svc.features, ft)
		}
	}
}

// attrsOf returns the keyed fields of a composite literal (Method, RequiredClaims, Default, Kind, ...).
func attrsOf(cl *ast.CompositeLit) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		out[key.Name] = kv.Value
	}
	return out
}

// inOutOf returns the In and Out struct type names from a define.* composite literal.
func inOutOf(cl *ast.CompositeLit) (in, out string) {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if key.Name != "In" && key.Name != "Out" {
			continue
		}
		name := ""
		if vcl, ok := kv.Value.(*ast.CompositeLit); ok {
			name = exprString(vcl.Type)
		}
		if key.Name == "In" {
			in = name
		} else {
			out = name
		}
	}
	return in, out
}

// sourceOf returns the package alias and event name of an InboundEvent's Source (e.g. for
// Source: eventsourceapi.OnRegistered it returns "eventsourceapi", "OnRegistered").
func sourceOf(cl *ast.CompositeLit) (pkg, name string) {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Source" {
			continue
		}
		sel, ok := kv.Value.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		return x.Name, sel.Sel.Name
	}
	return "", ""
}

// fieldsOf returns the fields of the named struct, or nil if the name is empty or the struct has none.
func (svc *service) fieldsOf(name string) []fieldDef {
	if name == "" {
		return nil
	}
	if sd := svc.structs[name]; sd != nil {
		return sd.fields
	}
	return nil
}

// exprSource renders an expression node back to Go source (e.g. "10 * time.Second", "[]float64{1, 2}").
func exprSource(fset *token.FileSet, e ast.Expr) string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	err := printer.Fprint(&b, fset, e)
	if err != nil {
		return ""
	}
	return b.String()
}

// attrString returns the unquoted value of a string-literal attribute, or "".
func attrString(attrs map[string]ast.Expr, key string) string {
	e, ok := attrs[key]
	if !ok {
		return ""
	}
	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return ""
	}
	return strings.Trim(bl.Value, "`\"")
}

// attrBool reports whether the attribute is the identifier true.
func attrBool(attrs map[string]ast.Expr, key string) bool {
	id, ok := attrs[key].(*ast.Ident)
	return ok && id.Name == "true"
}

// carrierTypeName returns the type carried by a value carrier: int(0) -> "int", MyStruct{} ->
// "MyStruct", time.Duration(0) -> "time.Duration". Empty if the expression is not a carrier.
func carrierTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.CallExpr:
		return exprString(t.Fun)
	case *ast.CompositeLit:
		return exprString(t.Type)
	}
	return ""
}

// stringSlice returns the string literals of a []string composite literal, e.g. []string{"a","b"}.
func stringSlice(e ast.Expr) []string {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var out []string
	for _, elt := range cl.Elts {
		bl, ok := elt.(*ast.BasicLit)
		if ok {
			out = append(out, strings.Trim(bl.Value, "`\""))
		}
	}
	return out
}

// metricKind resolves a Metric's Kind: define.Counter -> "counter", or a bare string literal.
func metricKind(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		return strings.ToLower(t.Sel.Name)
	case *ast.BasicLit:
		return strings.Trim(t.Value, "`\"")
	}
	return ""
}

// loadBalancingValue resolves a LoadBalancing field: define.None -> "none", define.Default ->
// "default", a bare string -> that string, absent/empty -> "".
func loadBalancingValue(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		switch t.Sel.Name {
		case "None":
			return "none"
		case "Default":
			return "default"
		}
	case *ast.BasicLit:
		return strings.Trim(t.Value, "`\"")
	}
	return ""
}

// cleanDoc extracts the godoc text from the first non-nil comment group, trimmed.
func cleanDoc(groups ...*ast.CommentGroup) string {
	for _, g := range groups {
		if g != nil {
			return strings.TrimSpace(g.Text())
		}
	}
	return ""
}

// importMap maps each import's alias (or default package name) to its path.
func importMap(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := path
		if i := strings.LastIndex(alias, "/"); i >= 0 {
			alias = alias[i+1:]
		}
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		out[alias] = path
	}
	return out
}

// exprString renders a type expression as Go source.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprString(t.Elt)
		}
		return "[" + exprString(t.Len) + "]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "any"
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	default:
		return fmt.Sprintf("%T", e)
	}
}

// selectorsIn returns the leading identifiers of any pkg.Type selectors in a rendered type string.
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
		j := i
		for j < len(typ) && isIdentCont(typ[j]) {
			j++
		}
		ident := typ[i:j]
		if j < len(typ) && typ[j] == '.' && !seen[ident] {
			seen[ident] = true
			out = append(out, ident)
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

// lowerFirst lowercases the leading run of capitals, leaving the last capital of an acronym run that
// is followed by lowercase. HTTPRequestBody -> httpRequestBody, SSN -> ssn, ApplicantName -> applicantName.
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
		r[i] = r[i] - 'A' + 'a'
	}
	return string(r)
}

func isUpperRune(r rune) bool { return r >= 'A' && r <= 'Z' }
