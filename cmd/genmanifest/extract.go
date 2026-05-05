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
	"sort"
	"strings"
)

// extracted holds everything the parser pulls out of source. It's a
// pre-merge bundle that the merge step combines with the existing manifest.
type extracted struct {
	hostname    string
	description string

	functions      []Endpoint
	webs           []Endpoint
	tasks          []Endpoint
	workflows      []Endpoint
	outboundEvents []Endpoint
	inboundEvents  []InboundEvent

	configs []Config
	metrics []Metric
	tickers []Ticker

	pkgPath string // service package path (best-effort, derived from go.mod dir)
}

// extract parses the microservice rooted at dir and returns a populated
// extracted struct. Any I/O or parse error is fatal.
func extract(dir string) (*extracted, error) {
	x := &extracted{}

	// Locate the *api/ subdirectory.
	apiDir, apiPkg, err := findAPIDir(dir)
	if err != nil {
		return nil, err
	}

	// Parse *api/endpoints.go for hostname + endpoint defs.
	endpointsPath := filepath.Join(apiDir, "endpoints.go")
	defs, hostname, err := parseEndpoints(endpointsPath)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", endpointsPath, err)
	}
	x.hostname = hostname

	// Parse *api/client.go for outbound event godocs (and their In/Out signatures).
	clientPath := filepath.Join(apiDir, "client.go")
	outboundDocs, err := parseOutboundEvents(clientPath)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", clientPath, err)
	}

	// Parse In/Out struct shapes for signature reconstruction.
	inOuts, err := parseInOutStructs(apiDir)
	if err != nil {
		return nil, fmt.Errorf("parse api structs: %w", err)
	}

	// Parse intermediate.go - the primary source of truth.
	intermediatePath := filepath.Join(dir, "intermediate.go")
	if err := parseIntermediate(intermediatePath, x, defs, apiPkg, inOuts); err != nil {
		return nil, fmt.Errorf("parse %s: %w", intermediatePath, err)
	}

	// Walk outbound events: every Def with a godoc in client.go's
	// MulticastTrigger receiver list is an outbound event. Iterate in sorted
	// name order so the emitted manifest is byte-stable across runs.
	defNames := make([]string, 0, len(defs))
	for name := range defs {
		defNames = append(defNames, name)
	}
	sort.Strings(defNames)
	for _, name := range defNames {
		doc, ok := outboundDocs[name]
		if !ok {
			continue
		}
		def := defs[name]
		x.outboundEvents = append(x.outboundEvents, Endpoint{
			Name:        name,
			Signature:   buildOutboundSignature(name, inOuts),
			Description: doc,
			Method:      def.Method,
			Route:       def.Route,
		})
	}

	// Walk service.go (and intermediate.go's imports) for downstreams, db/cloud
	// detection, hook source resolution, and ticker/inbound-event godocs.
	servicePath := filepath.Join(dir, "service.go")
	if err := enrichFromServiceFiles(servicePath, x, apiPkg); err != nil {
		return nil, fmt.Errorf("enrich from %s: %w", servicePath, err)
	}

	return x, nil
}

// findAPIDir locates the *api/ subdirectory and returns its absolute path and
// import path (last segment). The microservice directory is expected to contain
// exactly one such subdirectory.
func findAPIDir(dir string) (apiDir string, apiPkg string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, "api") && name != "api" {
			return filepath.Join(dir, name), name, nil
		}
	}
	return "", "", fmt.Errorf("no *api subdirectory found in %s", dir)
}

// signatureFromInOut reconstructs a function signature from the In/Out struct
// fields of an endpoint. inFields and outFields are in declaration order.
func signatureFromInOut(name string, inFields, outFields []structField) string {
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('(')
	for i, f := range inFields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteByte(' ')
		sb.WriteString(f.Type)
	}
	sb.WriteByte(')')
	if len(outFields) > 0 {
		sb.WriteString(" (")
		for i, f := range outFields {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(f.Name)
			sb.WriteByte(' ')
			sb.WriteString(f.Type)
		}
		sb.WriteByte(')')
	}
	return sb.String()
}

// buildOutboundSignature constructs an outbound event's signature using its
// In/Out struct fields.
func buildOutboundSignature(name string, inOuts map[string]inOutPair) string {
	pair, ok := inOuts[name]
	if !ok {
		return name + "()"
	}
	return signatureFromInOut(name, pair.in, pair.out)
}

// signatureFromInOutGoNames is like signatureFromInOut but uses each field's
// Go-derived name rather than its JSON tag. This preserves any `Out` suffix on
// output fields so the rendered signature is valid Go.
func signatureFromInOutGoNames(name string, inFields, outFields []structField) string {
	withGo := func(fs []structField) []structField {
		copies := make([]structField, len(fs))
		for i, f := range fs {
			n := f.GoName
			if n == "" {
				n = f.Name
			}
			copies[i] = structField{Name: n, Type: f.Type}
		}
		return copies
	}
	return signatureFromInOut(name, withGo(inFields), withGo(outFields))
}

// lowerFirst lowercases the leading run of capital letters in a name, matching
// the `strings.ToLower` step Go uses to convert struct field names like `SSN`
// or `URL` into JSON tags `ssn` / `url`. Mixed-case names (e.g. `FlowKey`) only
// have their first rune lowercased.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if !isUpperRune(r[0]) {
		return s
	}
	// Find the run of uppercase letters at the start.
	end := 1
	for end < len(r) && isUpperRune(r[end]) {
		end++
	}
	// If the run extends to end-of-string, lowercase the whole run (e.g. `URL` → `url`).
	// If the run is followed by a lowercase letter, the last uppercase is the
	// start of a new word and must stay uppercase (e.g. `URLPath` → `urlPath`).
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

// parseFile is a thin wrapper around go/parser that always includes comments.
func parseFile(path string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return f, fset, nil
}
