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

// enrichFromServiceFiles walks service.go and intermediate.go to enrich the
// extracted bundle with information that isn't in intermediate.go's
// NewIntermediate body. Specifically:
//   - downstream services (api packages used via NewClient / NewMulticastClient)
//   - the SQL database flag (database/sql or sequel imports)
//   - the cloud flag (httpegressapi import)
//   - inbound event source aliases resolved to full package paths
//   - first-sentence godocs for tickers and inbound-event handlers (used as
//     descriptions when not provided elsewhere)
//
// Despite the file name, this function is a side-channel enricher that touches
// both files; the file is named for historical reasons (service.go is its
// primary input).
func enrichFromServiceFiles(path string, x *extracted, ownAPIPkg string) error {
	mainFile, _, err := parseFile(path)
	if err != nil {
		return err
	}
	intermediatePath := filepath.Join(filepath.Dir(path), "intermediate.go")
	var intFile *ast.File
	if _, err := os.Stat(intermediatePath); err == nil {
		intFile, _, _ = parseFile(intermediatePath)
	}

	// Collect *api alias → import path so inbound-event entries (whose
	// `Package` field is initially the alias from intermediate.go) can
	// be resolved to the actual import path. Other side-effects of
	// import scanning (SQL/cloud detection, downstream-package
	// discovery) moved to cmd/gentopology when the JIT cutover dropped
	// those fields from the manifest.
	aliasToPath := map[string]string{}
	for _, f := range []*ast.File{mainFile, intFile} {
		if f == nil {
			continue
		}
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
	collectServiceGodocs := func(f *ast.File) {
		if f == nil {
			return
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			recv := receiverTypeName(fn.Recv.List[0].Type)
			if recv != "Service" {
				continue
			}
			if fn.Doc == nil {
				continue
			}
			godocs[fn.Name.Name] = firstSentence(strings.TrimSpace(fn.Doc.Text()))
		}
	}
	collectServiceGodocs(mainFile)
	collectServiceGodocs(intFile)

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

