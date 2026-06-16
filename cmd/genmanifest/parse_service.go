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
	"os"
	"path/filepath"
	"strings"
)

// enrichFromSourceFiles walks every non-test .go file in the microservice
// directory to enrich the extracted bundle with information that isn't in
// intermediate.go's NewIntermediate body:
//   - inbound event source aliases resolved to full package paths
//   - first-sentence godocs for tickers and inbound-event handlers (used as
//     descriptions when not provided elsewhere)
//
// It deliberately does not assume handlers live only in service.go: service.go
// may be split into transitions.go, scheduling.go, etc., and a ticker or
// inbound-event handler (and the *api import that names an event source) may
// live in any of those files. intermediate.go is scanned like any other file.
// (SQL/cloud detection and downstream-package discovery, which this function
// used to do off service.go, moved to cmd/gentopology when the JIT cutover
// dropped those fields from the manifest.)
func enrichFromSourceFiles(dir string, x *extracted, ownAPIPkg string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, _, err := parseFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		files = append(files, f)
	}

	// Collect *api alias → import path so inbound-event entries (whose
	// `Package` field is initially the alias from intermediate.go) can
	// be resolved to the actual import path.
	aliasToPath := map[string]string{}
	for _, f := range files {
		for _, imp := range f.Imports {
			pathStr := unquote(imp.Path.Value)
			alias := lastSegment(pathStr)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if strings.HasSuffix(alias, "api") && alias != "api" && alias != ownAPIPkg {
				aliasToPath[alias] = pathStr
			}
		}
	}

	// Resolve inbound event source aliases into the parent package path.
	for i := range x.inboundEvents {
		alias := x.inboundEvents[i].Package
		if full, ok := aliasToPath[alias]; ok {
			x.inboundEvents[i].Package = parentPackage(full)
		}
	}

	// Collect first-sentence godocs for Service methods, used as descriptions
	// for tickers and inbound events (whose descriptions don't appear elsewhere).
	godocs := map[string]string{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) != "Service" {
				continue
			}
			if fn.Doc == nil {
				continue
			}
			godocs[fn.Name.Name] = firstSentence(strings.TrimSpace(fn.Doc.Text()))
		}
	}

	for i := range x.tickers {
		if x.tickers[i].Description == "" {
			x.tickers[i].Description = godocs[x.tickers[i].Name]
		}
	}
	for i := range x.inboundEvents {
		if x.inboundEvents[i].Description == "" {
			x.inboundEvents[i].Description = godocs[x.inboundEvents[i].Name]
		}
	}

	return nil
}

// firstSentence returns the first sentence of a godoc paragraph. The sentence
// terminator is a period followed by whitespace (or end of string). If no
// terminator is found, the whole string up to the first newline pair is used.
func firstSentence(s string) string {
	// Strip trailing whitespace.
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Cut at the first paragraph boundary (blank line).
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	// Find the first ". " or ".\n".
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && (s[i+1] == ' ' || s[i+1] == '\n') {
			return strings.TrimSpace(s[:i+1])
		}
	}
	return strings.TrimSpace(s)
}
