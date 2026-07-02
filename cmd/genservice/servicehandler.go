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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// handlerStub drives one placeholder handler in service_handler.txt. Its signature and godoc are
// projected from definition.go so they cannot drift; the body is a TODO plus a naked return (a workflow
// gets a NewGraph starter that fails validation until its graph is defined).
type handlerStub struct {
	Godoc    string // rendered /* */ block ending in a newline, or "" when the feature has no description
	FuncName string // the *Service method name, e.g. Greet, OnObserveQueueDepth
	Params   string // parameter list without parens, e.g. "ctx context.Context, name string"
	Results  string // result list without parens, e.g. "greeting string, err error"
	Marker   string // the feature name, placed as the // MARKER comment
	Kind     string // feature kind; Workflow selects the NewGraph body
	Name     string // the feature name, used in the TODO text and NewGraph label

	imports []string
}

// emitServiceCode performs genservice's in-place edits of the microservice's hand-written Go: it syncs
// each existing handler's godoc to its feature description (emitServiceDocs) and appends a compiling stub
// to service.go for every feature whose handler is not defined yet. Both touch service.go, so they are
// combined here into a single output for that file rather than two conflicting ones.
func emitServiceCode(dir string, svc *service, apiPath string, resolveSource func(string) (*service, error)) ([]output, error) {
	docOuts, err := emitServiceDocs(dir, svc)
	if err != nil {
		return nil, err
	}
	servicePath := filepath.Join(dir, "service.go")
	// Build the stubs on top of any godoc sync already applied to service.go this run.
	base := outputContent(docOuts, servicePath)
	if base == nil {
		base, err = os.ReadFile(servicePath)
		if err != nil {
			// No service.go to extend (unusual): leave the godoc sync as the only code edit.
			return docOuts, nil
		}
	}
	newSrc, changed, err := emitServiceHandlers(dir, svc, apiPath, resolveSource, base)
	if err != nil {
		return nil, err
	}
	if !changed {
		return docOuts, nil
	}
	return replaceOrAppendOutput(docOuts, servicePath, newSrc), nil
}

// emitServiceHandlers appends a placeholder handler to serviceSrc for each feature whose *Service method
// is not already defined anywhere in the service directory, adding any imports the stub signatures need.
// It returns serviceSrc unchanged with changed=false when every handler already exists (so a fully
// implemented microservice regenerates to a no-op and -check stays stable).
func emitServiceHandlers(dir string, svc *service, apiPath string, resolveSource func(string) (*service, error), serviceSrc []byte) ([]byte, bool, error) {
	existing, err := existingServiceMethods(dir)
	if err != nil {
		return nil, false, err
	}
	docs := handlerDocs(svc)
	aliases, err := mockAliases(svc, apiPath, resolveSource)
	if err != nil {
		return nil, false, err
	}
	var stubs []handlerStub
	needed := map[string]bool{}
	for _, f := range svc.features {
		name, kind, ok := handlerMethodName(f)
		if !ok || existing[name] {
			continue
		}
		stub, err := buildHandlerStub(svc, f, name, kind, docs[name], aliases, resolveSource)
		if err != nil {
			return nil, false, err
		}
		stubs = append(stubs, stub)
		for _, p := range stub.imports {
			needed[p] = true
		}
	}
	if len(stubs) == 0 {
		return serviceSrc, false, nil
	}
	var buf bytes.Buffer
	err = clientTemplate.ExecuteTemplate(&buf, "service_handler.txt", struct{ Stubs []handlerStub }{stubs})
	if err != nil {
		return nil, false, err
	}
	out, err := appendGoDecls(serviceSrc, needed, buf.Bytes())
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// handlerMethodName returns the *Service method name a feature implements and its kind, or ok=false for a
// feature with no hand-written handler (a plain metric or a callback-less config). It matches the naming
// handlerDocs uses so the two stay in lockstep.
func handlerMethodName(f feature) (name, kind string, ok bool) {
	switch f.kind {
	case "Function", "Web", "Task", "Workflow", "InboundEvent", "Ticker":
		return f.name, f.kind, true
	case "Metric":
		if attrBool(f.attrs, "Observable") {
			return "OnObserve" + f.name, f.kind, true
		}
	case "Config":
		if attrBool(f.attrs, "Callback") {
			return "OnChanged" + f.name, f.kind, true
		}
	}
	return "", "", false
}

// buildHandlerStub renders one handler's signature (reusing the same field-qualification as the mock, so
// domain types are written svcapi.T in the service package) and computes the imports it references.
func buildHandlerStub(svc *service, f feature, name, kind, godoc string, aliases map[string]string, resolveSource func(string) (*service, error)) (handlerStub, error) {
	imports := map[string]bool{}
	var params, results string
	switch kind {
	case "Web":
		params, results = "w http.ResponseWriter, r *http.Request", "err error"
		imports[impHTTP] = true
	case "Workflow":
		params, results = "ctx context.Context", "graph *workflow.Graph, err error"
		imports[impContext] = true
		imports[impWorkflow] = true
	case "InboundEvent":
		iv, _, _, err := inboundView(svc, f, resolveSource)
		if err != nil {
			return handlerStub{}, err
		}
		mv := standardMock(name, f.name, f.srcPkg, false, iv.inFields, iv.outFields)
		params, results = mv.Sig, mv.Results
		imports[impContext] = true
		addResolved(imports, aliases, params, results)
	case "Task":
		mv := standardMock(name, f.name, svc.apiPkg, true, svc.fieldsOf(f.in), svc.fieldsOf(f.out))
		params, results = mv.Sig, mv.Results
		imports[impContext] = true
		imports[impWorkflow] = true
		addResolved(imports, aliases, params, results)
	default: // Function, Ticker, and the OnObserve/OnChanged callbacks
		var in, out []fieldDef
		if kind == "Function" {
			in, out = svc.fieldsOf(f.in), svc.fieldsOf(f.out)
		}
		mv := standardMock(name, f.name, svc.apiPkg, false, in, out)
		params, results = mv.Sig, mv.Results
		imports[impContext] = true
		addResolved(imports, aliases, params, results)
	}
	godocBlock := ""
	if godoc != "" {
		godocBlock = "/*\n" + godoc + "\n*/\n"
	}
	var imps []string
	for p := range imports {
		imps = append(imps, p)
	}
	return handlerStub{
		Godoc: godocBlock, FuncName: name, Params: params, Results: results,
		Marker: f.name, Kind: kind, Name: f.name, imports: imps,
	}, nil
}

// existingServiceMethods returns the set of *Service method names defined across the microservice's
// hand-written non-test .go files, so a feature whose handler already exists (in any file) is not stubbed
// again.
func existingServiceMethods(dir string) (map[string]bool, error) {
	return scanFuncDecls(dir, false, true)
}

// scanFuncDecls collects top-level declaration names across the microservice's hand-written .go files.
// testFiles selects the `_test.go` files (true) or the non-test files (false); methods=true keeps
// `*Service` methods, methods=false keeps plain (receiverless) functions. Generated files are skipped, so
// the returned set is exactly what a human wrote - handlers and tests may be split across any number of
// files, and all of them are searched.
func scanFuncDecls(dir string, testFiles, methods bool) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") != testFiles {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if bytes.Contains(src, []byte("Code generated")) && bytes.Contains(src, []byte("DO NOT EDIT")) {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if methods {
				if isServiceReceiver(fn) {
					names[fn.Name.Name] = true
				}
			} else if fn.Recv == nil {
				names[fn.Name.Name] = true
			}
		}
	}
	return names, nil
}

// outputContent returns the pending content for path among outs, or nil if none is queued.
func outputContent(outs []output, path string) []byte {
	for _, o := range outs {
		if o.path == path {
			return o.content
		}
	}
	return nil
}

// replaceOrAppendOutput sets the content for path in outs, replacing an existing entry or appending one.
func replaceOrAppendOutput(outs []output, path string, content []byte) []output {
	for i := range outs {
		if outs[i].path == path {
			outs[i].content = content
			return outs
		}
	}
	return append(outs, output{path, content})
}
