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
	"strings"
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
	HintBody string // the fully-rendered t.Run pattern for the HINT comment, tailored to the feature's signature

	imports []string
}

// emitServiceTests generates a placeholder test in service_test.go for each feature that does not yet
// have one, matching the per-kind pattern the add-* skills describe. Existing tests are never rewritten:
// a feature is skipped once a function of its test's name (Test<Name>_<Suffix>) exists, so hand-filled
// tests and re-runs are left untouched. It returns a single output for service_test.go when scaffolds were
// added, or nil when every feature is already covered (so -check does not flag an up-to-date directory).
func emitServiceTests(dir, pkg string, svc *service, apiPath string, resolveSource func(string) (*service, error)) ([]output, error) {
	views, err := testScaffoldViews(svc, apiPath, resolveSource)
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
func testScaffoldViews(svc *service, apiPath string, resolveSource func(string) (*service, error)) ([]scaffoldView, error) {
	var views []scaffoldView
	for _, f := range svc.features {
		base := []string{impTesting, impApplication}
		v := scaffoldView{Marker: f.name, APIPkg: svc.apiPkg}
		switch f.kind {
		case "Function":
			v.Kind, v.Handler, v.FuncName = "function", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
			claims := attrString(f.attrs, "RequiredClaims")
			mv := standardMock(f.name, f.name, svc.apiPkg, false, svc.fieldsOf(f.in), svc.fieldsOf(f.out))
			v.HintBody = wrapRun(callInner(hintCaller("client", f.name, claims), mv, claims))
		case "Web":
			v.Kind, v.Handler, v.FuncName = "web", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
			v.HintBody = wrapRun(webInner(f.name, attrString(f.attrs, "Method"), attrString(f.attrs, "RequiredClaims")))
		case "Task":
			v.Kind, v.Handler, v.FuncName = "task", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
			claims := attrString(f.attrs, "RequiredClaims")
			mv := standardMock(f.name, f.name, svc.apiPkg, false, svc.fieldsOf(f.in), svc.fieldsOf(f.out))
			v.HintBody = wrapRun(callInner(hintCaller("exec", f.name, claims), mv, claims))
		case "Workflow":
			v.Kind, v.Handler, v.FuncName = "workflow", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath, impForeman, impForemanAPI)
			claims := attrString(f.attrs, "RequiredClaims")
			mv := standardMock(f.name, f.name, svc.apiPkg, false, svc.fieldsOf(f.in), svc.fieldsOf(f.out))
			v.HintBody = wrapRun(workflowInner(hintCaller("exec", f.name, claims), mv, claims))
		case "InboundEvent":
			srcPath, ok := svc.imports[f.srcPkg]
			if !ok {
				return nil, fmt.Errorf("inbound event %q: unknown source package %q", f.name, f.srcPkg)
			}
			v.Kind, v.Handler, v.SrcPkg, v.FuncName = "inbound", f.name, f.srcPkg, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, srcPath)
			iv, _, _, err := inboundView(svc, f, resolveSource)
			if err != nil {
				return nil, err
			}
			claims := attrString(f.attrs, "RequiredClaims")
			mv := standardMock(f.name, f.name, f.srcPkg, false, iv.inFields, iv.outFields)
			v.HintBody = wrapRun(inboundInner(hintCaller("trigger", f.name, claims), mv, claims))
		case "OutboundEvent":
			v.Kind, v.Handler, v.FuncName = "outbound", f.name, testFuncName(svc, f.name)
			v.imports = append(base, impConnector, apiPath)
			mv := standardMock(f.name, f.name, svc.apiPkg, false, svc.fieldsOf(f.in), svc.fieldsOf(f.out))
			v.HintBody = wrapRun(outboundInner(f.name, mv))
		case "Ticker":
			v.Kind, v.Handler, v.FuncName = "ticker", f.name, testFuncName(svc, f.name)
			v.imports = base
			v.HintBody = wrapRun(noArgInner("svc." + f.name))
		case "Config":
			if !attrBool(f.attrs, "Callback") {
				continue
			}
			v.Kind, v.Handler, v.FuncName = "config", "Set"+f.name, testFuncName(svc, "OnChanged"+f.name)
			v.imports = base
			v.HintBody = wrapRun(configInner("Set" + f.name))
		case "Metric":
			if !attrBool(f.attrs, "Observable") {
				continue
			}
			v.Kind, v.Handler, v.FuncName = "metric", "OnObserve"+f.name, testFuncName(svc, "OnObserve"+f.name)
			v.imports = base
			v.HintBody = wrapRun(noArgInner("svc.OnObserve" + f.name))
		default:
			continue
		}
		views = append(views, v)
	}
	return views, nil
}

// The HINT comment sits inside the test function's /* */ block at one tab of indentation. Its rendered
// t.Run pattern therefore starts at two tabs; deeper statements step in from there. These builders emit
// the exact tab runs since gofmt does not reindent block-comment interiors.
const (
	hintI2 = "\t\t"
	hintI3 = "\t\t\t"
	hintI4 = "\t\t\t\t"
	hintI5 = "\t\t\t\t\t"
	hintI6 = "\t\t\t\t\t\t"
)

// wrapRun wraps a rendered inner body in the standard t.Run + asserter scaffold.
func wrapRun(inner string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%st.Run(\"test_case_name\", func(t *testing.T) {\n", hintI2)
	fmt.Fprintf(&b, "%sassert := testarossa.For(t)\n\n", hintI3)
	b.WriteString(inner)
	fmt.Fprintf(&b, "%s})", hintI2)
	return b.String()
}

// hintAssertPairs emits an `out, expectedOut,` assertion line per output name.
func hintAssertPairs(indent, retNoErr string) string {
	var b strings.Builder
	for _, n := range splitNames(retNoErr) {
		fmt.Fprintf(&b, "%s%s, expected%s,\n", indent, n, upperFirst(n))
	}
	return b.String()
}

// splitNames splits a comma-separated identifier list, returning nil for the empty string.
func splitNames(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ", ")
}

// upperFirst upper-cases the first rune, for deriving an expected<Output> placeholder name.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// hintCaller renders the method-call receiver (e.g. "client.Greet"), wrapping it with an actor option when
// the feature declares requiredClaims so the gated call passes the claims check in the TESTING deployment.
func hintCaller(recv, method, claims string) string {
	if claims == "" {
		return recv + "." + method
	}
	return recv + ".WithOptions(pub.Actor(actor))." + method
}

// hintClaims renders a comment naming the requiredClaims expression the placeholder actor must satisfy, or
// nothing when the feature is ungated.
func hintClaims(claims string) string {
	if claims == "" {
		return ""
	}
	return fmt.Sprintf("%s// actor's claims must satisfy: %s\n", hintI3, claims)
}

// callInner renders a direct request/response call (function, task): invoke the caller with the input
// argument names as placeholders, and assert each output against an expected placeholder.
func callInner(caller string, mv *mockView, claims string) string {
	var b strings.Builder
	b.WriteString(hintClaims(claims))
	if mv.SingleErr {
		fmt.Fprintf(&b, "%serr := %s(%s)\n", hintI3, caller, mv.Args)
		fmt.Fprintf(&b, "%sassert.NoError(err)\n", hintI3)
		return b.String()
	}
	fmt.Fprintf(&b, "%s%s := %s(%s)\n", hintI3, mv.LValues, caller, mv.Args)
	fmt.Fprintf(&b, "%sassert.Expect(\n", hintI3)
	b.WriteString(hintAssertPairs(hintI4, mv.RetNoErr))
	fmt.Fprintf(&b, "%serr, nil,\n", hintI4)
	fmt.Fprintf(&b, "%s)\n", hintI3)
	return b.String()
}

// workflowInner renders a workflow run: like callInner but the executor also returns a workflow.Status.
func workflowInner(caller string, mv *mockView, claims string) string {
	var b strings.Builder
	b.WriteString(hintClaims(claims))
	lhs := "status, err"
	if mv.RetNoErr != "" {
		lhs = mv.RetNoErr + ", status, err"
	}
	fmt.Fprintf(&b, "%s%s := %s(%s)\n", hintI3, lhs, caller, mv.Args)
	fmt.Fprintf(&b, "%sassert.Expect(\n", hintI3)
	fmt.Fprintf(&b, "%serr, nil,\n", hintI4)
	fmt.Fprintf(&b, "%sstatus, workflow.StatusCompleted,\n", hintI4)
	b.WriteString(hintAssertPairs(hintI4, mv.RetNoErr))
	fmt.Fprintf(&b, "%s)\n", hintI3)
	return b.String()
}

// inboundInner renders firing the source event and asserting the sink received it.
func inboundInner(caller string, mv *mockView, claims string) string {
	var b strings.Builder
	b.WriteString(hintClaims(claims))
	fmt.Fprintf(&b, "%sfor e := range %s(%s) {\n", hintI3, caller, mv.Args)
	if mv.SingleErr {
		fmt.Fprintf(&b, "%serr := e.Get()\n", hintI4)
		fmt.Fprintf(&b, "%sif frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {\n", hintI4)
		fmt.Fprintf(&b, "%sassert.NoError(err)\n", hintI5)
		fmt.Fprintf(&b, "%s}\n", hintI4)
	} else {
		fmt.Fprintf(&b, "%s%s := e.Get()\n", hintI4, mv.LValues)
		fmt.Fprintf(&b, "%sif frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {\n", hintI4)
		fmt.Fprintf(&b, "%sassert.Expect(\n", hintI5)
		b.WriteString(hintAssertPairs(hintI6, mv.RetNoErr))
		fmt.Fprintf(&b, "%serr, nil,\n", hintI6)
		fmt.Fprintf(&b, "%s)\n", hintI5)
		fmt.Fprintf(&b, "%s}\n", hintI4)
	}
	fmt.Fprintf(&b, "%s}\n", hintI3)
	return b.String()
}

// outboundInner renders hooking the event, firing it, and asserting the response, tailored to the event's
// own handler signature.
func outboundInner(handler string, mv *mockView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%sunsub, err := hook.WithOptions(sub.Queue(\"UniqueQueueName\")).%s(\n", hintI3, handler)
	fmt.Fprintf(&b, "%sfunc(%s) (%s) {\n", hintI4, mv.Sig, mv.Results)
	fmt.Fprintf(&b, "%s// Implement event sink here...\n", hintI5)
	if mv.SingleErr {
		fmt.Fprintf(&b, "%sreturn nil\n", hintI5)
	} else {
		fmt.Fprintf(&b, "%sreturn %s, nil\n", hintI5, mv.RetNoErr)
	}
	fmt.Fprintf(&b, "%s},\n", hintI4)
	fmt.Fprintf(&b, "%s)\n", hintI3)
	fmt.Fprintf(&b, "%sif assert.NoError(err) {\n", hintI3)
	fmt.Fprintf(&b, "%sdefer unsub()\n", hintI4)
	fmt.Fprintf(&b, "%s}\n", hintI3)
	fmt.Fprintf(&b, "%sfor e := range trigger.%s(%s) {\n", hintI3, handler, mv.Args)
	fmt.Fprintf(&b, "%sif frame.Of(e.HTTPResponse).FromHost() == tester.Hostname() {\n", hintI4)
	if mv.SingleErr {
		fmt.Fprintf(&b, "%serr := e.Get()\n", hintI5)
		fmt.Fprintf(&b, "%sassert.NoError(err)\n", hintI5)
	} else {
		fmt.Fprintf(&b, "%s%s := e.Get()\n", hintI5, mv.LValues)
		fmt.Fprintf(&b, "%sassert.Expect(\n", hintI5)
		b.WriteString(hintAssertPairs(hintI6, mv.RetNoErr))
		fmt.Fprintf(&b, "%serr, nil,\n", hintI6)
		fmt.Fprintf(&b, "%s)\n", hintI5)
	}
	fmt.Fprintf(&b, "%s}\n", hintI4)
	fmt.Fprintf(&b, "%s}\n", hintI3)
	return b.String()
}

// webInner renders a raw web call whose arity matches the endpoint's HTTP method.
func webInner(handler, method, claims string) string {
	recv := "client"
	if claims != "" {
		recv = "client.WithOptions(pub.Actor(actor))"
	}
	var call string
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH":
		call = fmt.Sprintf("%s.%s(ctx, \"\", nil)", recv, handler)
	case "ANY", "":
		call = fmt.Sprintf("%s.%s(ctx, \"GET\", \"\", nil)", recv, handler)
	default:
		call = fmt.Sprintf("%s.%s(ctx, \"\")", recv, handler)
	}
	var b strings.Builder
	b.WriteString(hintClaims(claims))
	fmt.Fprintf(&b, "%sres, err := %s\n", hintI3, call)
	fmt.Fprintf(&b, "%sif assert.NoError(err) {\n", hintI3)
	fmt.Fprintf(&b, "%sassert.Expect(res.StatusCode, http.StatusOK)\n", hintI4)
	fmt.Fprintf(&b, "%s}\n", hintI3)
	return b.String()
}

// configInner renders setting the config, with the value as a placeholder.
func configInner(handler string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%serr := svc.%s(value)\n", hintI3, handler)
	fmt.Fprintf(&b, "%sassert.NoError(err)\n", hintI3)
	return b.String()
}

// noArgInner renders invoking a ctx-only handler (ticker, observable metric).
func noArgInner(caller string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%serr := %s(ctx)\n", hintI3, caller)
	fmt.Fprintf(&b, "%sassert.NoError(err)\n", hintI3)
	return b.String()
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
