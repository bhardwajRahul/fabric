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
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

// serviceTestModel is handed to service_test.txt: the ordered scaffolds to render as test functions.
type serviceTestModel struct {
	Scaffolds []scaffoldView
}

// scaffoldView drives one placeholder test function in service_test.txt. Kind selects the template
// branch; the remaining fields fill in the feature's names. imports is Go-only (the template does not
// read it) and lists the paths the compiling part of the scaffold references.
type scaffoldView struct {
	Kind     string // function | web | task | workflow | inbound | outbound | ticker | config | metric
	FuncName string // full test function name, e.g. TestMyService_MyFunction; also the coverage key
	Marker   string // the feature name, emitted as the // MARKER comment for feature-code navigation
	APIPkg   string // this service's api package alias, e.g. myserviceapi
	SrcPkg   string // inbound only: the source event's api package alias
	Handler  string // the method the HINT block invokes, e.g. MyFunction, SetMyConfig, OnObserveMyMetric

	imports []string
}

// emitServiceTests generates a placeholder test in service_test.go for each feature that does not yet
// have one, matching the per-kind pattern the add-* skills describe. Existing tests are never rewritten:
// a feature is skipped once a function of its test's name (Test<Name>_<Suffix>) exists, so hand-filled
// tests and re-runs are left untouched. It returns a single output for service_test.go when scaffolds were
// added, or nil when every feature is already covered (so -check does not flag an up-to-date directory).
func emitServiceTests(dir, pkg string, svc *service, apiPath string) ([]output, error) {
	views, err := testScaffoldViews(svc, apiPath)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, nil
	}
	testPath := filepath.Join(dir, "service_test.go")
	orig, readErr := os.ReadFile(testPath)
	exists := readErr == nil

	existing, err := existingTestFuncs(dir)
	if err != nil {
		return nil, err
	}
	var missing []scaffoldView
	needed := map[string]bool{}
	for _, v := range views {
		if existing[v.FuncName] {
			continue
		}
		missing = append(missing, v)
		for _, p := range v.imports {
			needed[p] = true
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}

	funcs, err := renderTestScaffolds(missing)
	if err != nil {
		return nil, err
	}
	var src []byte
	if exists {
		src, err = appendGoDecls(orig, needed, funcs)
	} else {
		src, err = createGoFile(pkg, needed, funcs)
	}
	if err != nil {
		return nil, err
	}
	return []output{{testPath, src}}, nil
}

// testScaffoldViews projects the service's features into the scaffolds to emit, in source order. Config
// and metric features are included only when they carry a hand-written callback (Callback / Observable),
// matching the kinds that have a *Service method to test.
func testScaffoldViews(svc *service, apiPath string) ([]scaffoldView, error) {
	var views []scaffoldView
	for _, f := range svc.features {
		base := []string{impTesting, impApplication}
		v := scaffoldView{Marker: f.name, APIPkg: svc.apiPkg}
		switch f.kind {
		case "Function":
			v.Kind, v.Handler, v.FuncName = "function", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
		case "Web":
			v.Kind, v.Handler, v.FuncName = "web", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
		case "Task":
			v.Kind, v.Handler, v.FuncName = "task", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
		case "Workflow":
			v.Kind, v.Handler, v.FuncName = "workflow", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath, impForeman, impForemanAPI)
		case "InboundEvent":
			srcPath, ok := svc.imports[f.srcPkg]
			if !ok {
				return nil, fmt.Errorf("inbound event %q: unknown source package %q", f.name, f.srcPkg)
			}
			v.Kind, v.Handler, v.SrcPkg, v.FuncName = "inbound", f.name, f.srcPkg, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, srcPath)
		case "OutboundEvent":
			v.Kind, v.Handler, v.FuncName = "outbound", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
		case "Ticker":
			v.Kind, v.Handler, v.FuncName = "ticker", f.name, testFuncName(svc, f.name)
			v.imports = base
		case "Config":
			if !attrBool(f.attrs, "Callback") {
				continue
			}
			v.Kind, v.Handler, v.FuncName = "config", "Set"+f.name, testFuncName(svc, "OnChanged"+f.name)
			v.imports = base
		case "Metric":
			if !attrBool(f.attrs, "Observable") {
				continue
			}
			v.Kind, v.Handler, v.FuncName = "metric", "OnObserve"+f.name, testFuncName(svc, "OnObserve"+f.name)
			v.imports = base
		default:
			continue
		}
		views = append(views, v)
	}
	return views, nil
}

// testFuncName builds the test function name from the decorative service Name const and a suffix, e.g.
// TestMyService_MyFunction. This matches the name a human following the add-* skills would author.
func testFuncName(svc *service, suffix string) string {
	return "Test" + svc.name + "_" + suffix
}

// existingTestFuncs returns the set of top-level test function names defined across all of the
// microservice's hand-written _test.go files, so a feature whose test already exists (in any file) is not
// scaffolded again. This mirrors existingServiceMethods on the handler side, keying coverage on the
// function name rather than on the // MARKER comment.
func existingTestFuncs(dir string) (map[string]bool, error) {
	return scanFuncDecls(dir, true, false)
}

// renderTestScaffolds runs the missing scaffolds through service_test.txt. The result is gofmt-normalized
// by the caller when it assembles the final file.
func renderTestScaffolds(views []scaffoldView) ([]byte, error) {
	var buf bytes.Buffer
	err := clientTemplate.ExecuteTemplate(&buf, "service_test.txt", serviceTestModel{Scaffolds: views})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// createGoFile assembles a fresh source file from the package clause, the needed imports, and the
// rendered declarations, then gofmts the whole thing. Used when service_test.go does not yet exist.
func createGoFile(pkg string, needed map[string]bool, decls []byte) ([]byte, error) {
	set := splitImports(needed)
	var b bytes.Buffer
	b.WriteString("package ")
	b.WriteString(pkg)
	b.WriteString("\n\nimport (\n")
	writeImportLines(&b, set.Std)
	if len(set.Std) > 0 && len(set.Ext) > 0 {
		b.WriteByte('\n')
	}
	writeImportLines(&b, set.Ext)
	b.WriteString(")\n\n")
	b.Write(decls)
	return format.Source(b.Bytes())
}

// mergeTestFile appends the rendered scaffolds to an existing service_test.go and adds any imports they
// reference that the file does not already have. The added imports go into the existing import block; the
// final gofmt sorts them into their group without disturbing the file's other imports or hand-written
// code. gofmt operates on source text (not an AST reprint), so comments are preserved. Shared by the
// service_test.go and service.go emitters.
func appendGoDecls(orig []byte, needed map[string]bool, decls []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", orig, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var importDecl *ast.GenDecl
	existing := map[string]bool{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		importDecl = gd
		for _, s := range gd.Specs {
			if p, err := strconv.Unquote(s.(*ast.ImportSpec).Path.Value); err == nil {
				existing[p] = true
			}
		}
		break
	}
	toAdd := map[string]bool{}
	for p := range needed {
		if !existing[p] {
			toAdd[p] = true
		}
	}

	out := orig
	if len(toAdd) > 0 {
		set := splitImports(toAdd)
		var ins bytes.Buffer
		writeImportLines(&ins, append(set.Std, set.Ext...))
		if importDecl != nil && importDecl.Rparen.IsValid() {
			at := fset.Position(importDecl.Rparen).Offset
			out = spliceBytes(out, at, at, ins.Bytes())
		} else {
			// No block import to extend: open a fresh one right after the package clause.
			at := fset.Position(f.Name.End()).Offset
			block := "\n\nimport (\n" + ins.String() + ")"
			out = spliceBytes(out, at, at, []byte(block))
		}
	}
	out = append(out, '\n', '\n')
	out = append(out, decls...)
	return format.Source(out)
}

// writeImportLines writes each path as a tab-indented quoted import line.
func writeImportLines(b *bytes.Buffer, paths []string) {
	for _, p := range paths {
		b.WriteByte('\t')
		b.WriteString(strconv.Quote(p))
		b.WriteByte('\n')
	}
}

// spliceBytes replaces src[start:end] with repl.
func spliceBytes(src []byte, start, end int, repl []byte) []byte {
	out := make([]byte, 0, len(src)-(end-start)+len(repl))
	out = append(out, src[:start]...)
	out = append(out, repl...)
	out = append(out, src[end:]...)
	return out
}
