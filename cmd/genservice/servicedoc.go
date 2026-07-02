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
	"sort"
	"strings"
)

// serviceDocKinds are the feature kinds whose handler is a *Service method named exactly after the
// feature, so the method's godoc is the feature's description in definition.go verbatim. Observable
// metrics and callback configs also have hand-written *Service methods (OnObserveXxx, OnChangedXxx), but
// those are named after the feature with a fixed prefix; handlerDocs synthesizes their first line so the
// godoc opens with the method name and appends the feature's description as a second paragraph.
var serviceDocKinds = map[string]bool{
	"Function": true, "Web": true, "Task": true, "Workflow": true,
	"InboundEvent": true, "Ticker": true,
}

// emitServiceDocs rewrites the godoc of each *Service handler method in the service directory's
// hand-written .go files so it matches the description of the feature it implements in definition.go.
// A handler with no matching feature is left untouched, as are generated and test files. Only files
// whose bytes actually change are returned, so an up-to-date directory produces no output.
func emitServiceDocs(dir string, svc *service) ([]output, error) {
	docs := handlerDocs(svc)
	if len(docs) == 0 {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var outs []output
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		orig, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if bytes.Contains(orig, []byte("Code generated")) && bytes.Contains(orig, []byte("DO NOT EDIT")) {
			continue
		}
		updated, err := syncHandlerDocs(orig, docs)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(orig, updated) {
			outs = append(outs, output{filepath.Join(dir, e.Name()), updated})
		}
	}
	return outs, nil
}

// handlerDocs maps each hand-written *Service handler method name to the godoc body it should carry. For
// the kinds named after their feature the body is the feature's description verbatim; for an observable
// metric's OnObserveXxx and a callback config's OnChangedXxx it is a genservice-fixed first line (so the
// godoc opens with the method name) followed by the feature's description as a second paragraph.
func handlerDocs(svc *service) map[string]string {
	docs := map[string]string{}
	for _, ft := range svc.features {
		switch {
		case serviceDocKinds[ft.kind]:
			if ft.doc != "" {
				docs[ft.name] = ft.doc
			}
		case ft.kind == "Metric" && attrBool(ft.attrs, "Observable"):
			docs["OnObserve"+ft.name] = withFixedFirstLine(
				"OnObserve"+ft.name+" emits the observed value of the "+ft.name+" metric.", ft.doc)
		case ft.kind == "Config" && attrBool(ft.attrs, "Callback"):
			docs["OnChanged"+ft.name] = withFixedFirstLine(
				"OnChanged"+ft.name+" is called when the "+ft.name+" config property changes.", ft.doc)
		}
	}
	return docs
}

// withFixedFirstLine joins a genservice-authored first line with the feature's description as a separate
// paragraph, or returns the first line alone when the feature has no description.
func withFixedFirstLine(firstLine, doc string) string {
	if doc == "" {
		return firstLine
	}
	return firstLine + "\n\n" + doc
}

// syncHandlerDocs replaces the godoc of every top-level *Service method whose name is a key in docs with
// a /* */ block comment carrying that description. It returns the rewritten source. Edits are byte
// splices against the original text, so code outside the targeted doc comments is preserved exactly;
// only the located comment regions change.
func syncHandlerDocs(src []byte, docs map[string]string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isServiceReceiver(fn) {
			continue
		}
		doc, ok := docs[fn.Name.Name]
		if !ok {
			continue
		}
		block := "/*\n" + doc + "\n*/"
		if fn.Doc != nil {
			// Replace the existing comment group in place; the newline that separates it from the
			// func keyword lies beyond Doc.End() and is preserved.
			edits = append(edits, edit{
				start: fset.Position(fn.Doc.Pos()).Offset,
				end:   fset.Position(fn.Doc.End()).Offset,
				text:  block,
			})
			continue
		}
		// No existing godoc: insert the block comment immediately before the func keyword.
		at := fset.Position(fn.Pos()).Offset
		edits = append(edits, edit{start: at, end: at, text: block + "\n"})
	}
	if len(edits) == 0 {
		return src, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := src
	for _, e := range edits {
		spliced := make([]byte, 0, len(out)-(e.end-e.start)+len(e.text))
		spliced = append(spliced, out[:e.start]...)
		spliced = append(spliced, e.text...)
		spliced = append(spliced, out[e.end:]...)
		out = spliced
	}
	// Re-parse to guarantee the splice produced valid Go rather than silently writing a broken file.
	_, err = parser.ParseFile(token.NewFileSet(), "", out, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// isServiceReceiver reports whether fn is a method on *Service or Service.
func isServiceReceiver(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return false
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	return ok && id.Name == "Service"
}
