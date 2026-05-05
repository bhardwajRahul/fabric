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
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// def is the resolved Method/Route for one Def literal in *api/endpoints.go.
type def struct {
	Method string
	Route  string
}

// structField describes one field of an In/Out struct.
type structField struct {
	Name   string // JSON tag name when present, otherwise lowerFirst of the Go field name
	GoName string // Go field name lower-cased on its first letter (preserves any Out suffix)
	Type   string // Go type rendered as source
}

// inOutPair groups the input and output struct field lists of one endpoint.
type inOutPair struct {
	in  []structField
	out []structField
}

// parseEndpoints walks *api/endpoints.go and returns the set of Def literals
// (keyed by var name) plus the package-level Hostname constant.
func parseEndpoints(path string) (defs map[string]def, hostname string, err error) {
	f, _, err := parseFile(path)
	if err != nil {
		return nil, "", err
	}
	defs = make(map[string]def)
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if gen.Tok == token.CONST && name.Name == "Hostname" {
					if bl, ok := vs.Values[i].(*ast.BasicLit); ok {
						hostname = unquote(bl.Value)
					}
					continue
				}
				if gen.Tok != token.VAR {
					continue
				}
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					continue
				}
				if !isDefType(cl.Type) {
					continue
				}
				d := def{}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, _ := kv.Key.(*ast.Ident)
					val, _ := kv.Value.(*ast.BasicLit)
					if key == nil || val == nil {
						continue
					}
					switch key.Name {
					case "Method":
						d.Method = unquote(val.Value)
					case "Route":
						d.Route = unquote(val.Value)
					}
				}
				defs[name.Name] = d
			}
		}
	}
	return defs, hostname, nil
}

// isDefType reports whether the given type expression is the bare identifier `Def`.
func isDefType(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "Def"
}

// parseOutboundEvents reads *api/client.go and returns a map of event name →
// godoc description for each MulticastTrigger method (which is exactly the set
// of outbound events). The first sentence/paragraph of the godoc is the
// description; CRITICAL is preserved verbatim.
func parseOutboundEvents(path string) (map[string]string, error) {
	f, _, err := parseFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		recvType := receiverTypeName(fn.Recv.List[0].Type)
		if recvType != "MulticastTrigger" {
			continue
		}
		// Skip framework helpers like ForHost / WithOptions.
		switch fn.Name.Name {
		case "ForHost", "WithOptions":
			continue
		}
		desc := docText(fn.Doc)
		if desc == "" {
			continue
		}
		out[fn.Name.Name] = desc
	}
	return out, nil
}

// parseInOutStructs walks every .go file in apiDir and gathers In/Out struct
// shapes. Result keys are the endpoint base name (e.g. "Mint", "OnFlowStopped").
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
		path := filepath.Join(apiDir, e.Name())
		f, _, err := parseFile(path)
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
				fields := flattenFields(st)
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

// flattenFields extracts (name, typeText) pairs from a struct, in declaration
// order. Anonymous/embedded fields are skipped. Each field's `Name` is taken
// from the json struct tag when present (so `SSN string \`json:"ssn,…"\“
// renders as `ssn` rather than `sSN`); falls back to lowerFirst of the field
// name otherwise.
func flattenFields(st *ast.StructType) []structField {
	var out []structField
	if st.Fields == nil {
		return out
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		typeStr := exprString(field.Type)
		jsonName := jsonTagName(field.Tag)
		for _, n := range field.Names {
			goName := lowerFirst(n.Name)
			name := jsonName
			if name == "" {
				name = goName
			}
			out = append(out, structField{Name: name, GoName: goName, Type: typeStr})
		}
	}
	return out
}

// jsonTagName extracts the JSON field name from a struct field's tag. Returns
// "" if no json tag is present.
func jsonTagName(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	raw := strings.Trim(tag.Value, "`")
	// Look for json:"name,..." inside the tag.
	idx := strings.Index(raw, `json:"`)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(`json:"`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	val := rest[:end]
	if comma := strings.Index(val, ","); comma >= 0 {
		val = val[:comma]
	}
	if val == "-" {
		return ""
	}
	return val
}

// exprString renders a type expression as Go source. It supports the shapes
// the framework uses in In/Out structs: identifiers, selectors, pointers,
// slices, arrays, maps, and `any`/`interface{}`.
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
	default:
		return fmt.Sprintf("%T", e)
	}
}

// unquote unwraps a Go string literal. Double-quoted strings are decoded via
// strconv.Unquote so escapes like `\n` / `\t` / `\"` become their runtime
// values; backtick raw strings are returned with the backticks stripped (they
// have no escapes by definition). Falls back to a naive strip when the input
// isn't a recognizable Go string literal - descriptions that fail strconv
// parsing still degrade gracefully.
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

// receiverTypeName returns the type name of a method receiver, e.g. for
// `(_c MulticastTrigger)` it returns "MulticastTrigger". Pointer receivers
// are dereferenced.
func receiverTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// docText returns the doc comment as a single string with leading "//" or "/*"
// markers stripped. Blank trailing lines are trimmed but the multi-line shape
// of the comment is preserved.
func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	text := g.Text()
	return strings.TrimSpace(text)
}
