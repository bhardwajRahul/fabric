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

// Per-service AST scan. Cheaper than cmd/gencreds's scan because the
// topology only needs to know:
//   - which other services are dependencies (any *api import)
//   - which services we hook events from (any NewHook call)
//   - whether the service touches SQL (database/sql, sequel, or dwarf engine imports)
//   - whether the service runs a Python venv (pyvenv import)
//   - whether the service uses HTTP egress + which external host(s)
//
// No per-call route resolution, no Def lookup, no rule construction.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/microbus-io/fabric/cmd/internal/pkgresolver"
)

// scanned is a service's source-derived facts ready for the mermaid
// renderer.
type scanned struct {
	service
	Deps   []string // hostnames of services this one depends on (via *api client)
	Events []string // hostnames of services this one hooks events from
	DB     string   // "SQL" if database/sql, sequel, or the dwarf engine is imported, else ""
	Py     string   // "Py" if pyvenv is imported, else ""
	Cloud  string   // external host (if exactly one), "various" (if >1 or none extractable), or "" if no egress
}

// scanAll scans every service in the bundle and resolves cross-package
// hostnames against the *api packages each service imports. Bundle-internal
// hostnames are tracked so cloud detection can filter them out.
func scanAll(services []service, bundleDir string) ([]scanned, error) {
	bundleHosts := map[string]bool{}
	for _, s := range services {
		bundleHosts[s.Hostname] = true
	}
	resolver := pkgresolver.New(bundleDir)
	out := make([]scanned, 0, len(services))
	for _, s := range services {
		sc, err := scanService(s, bundleDir, bundleHosts, resolver)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", s.Hostname, err)
		}
		out = append(out, sc)
	}
	return out, nil
}

// scanService walks one service's source and returns its facts.
func scanService(s service, bundleDir string, bundleHosts map[string]bool, resolver *pkgresolver.Resolver) (scanned, error) {
	listDir := bundleDir
	if listDir == "" {
		listDir = s.Dir
	}
	out := scanned{service: s}

	// Collect all imports across non-test .go files.
	imports, err := collectImports(s.Dir)
	if err != nil {
		return out, err
	}

	// SQL: detected by import only. The dwarf engine is a SQL-backed
	// orchestrator (it requires a DSN), so any service embedding it is
	// SQL-backed even though it imports sequel only transitively.
	for _, imp := range imports {
		if imp == "database/sql" || imp == "github.com/microbus-io/sequel" || strings.HasPrefix(imp, "github.com/microbus-io/sequel/") ||
			imp == "github.com/microbus-io/dwarf/engine" {
			out.DB = "SQL"
			break
		}
	}

	// Python: detected by pyvenv import.
	for _, imp := range imports {
		if imp == "github.com/microbus-io/pyvenv" || strings.HasPrefix(imp, "github.com/microbus-io/pyvenv/") {
			out.Py = "Py"
			break
		}
	}

	// Service deps: every *api-suffixed import that isn't this service's own.
	// "Own" is identified by reading the import's last path segment and
	// checking for an own-api directory of that name in the service.
	ownAPIAlias, _ := findOwnAPIName(s.Dir)
	apiImports := map[string]bool{}
	hasEgress := false
	for _, imp := range imports {
		base := lastSegment(imp)
		if !strings.HasSuffix(base, "api") || base == "api" {
			continue
		}
		if base == "httpegressapi" {
			hasEgress = true
		}
		if base == ownAPIAlias {
			continue
		}
		apiImports[imp] = true
	}

	// Distinguish *api imports used as clients (`NewClient` /
	// `NewMulticastClient` / `NewMulticastTrigger`) from those used
	// only as event sinks (`NewHook`). A hook-only import doesn't
	// produce a dep edge - it produces an event edge instead, which
	// already conveys the dependency.
	clientAliases, hookAliases, err := scanAPIUsage(s.Dir, ownAPIAlias)
	if err != nil {
		return out, err
	}
	depHosts := map[string]bool{}
	eventHosts := map[string]bool{}
	for imp := range apiImports {
		dir, err := resolver.Dir(imp, listDir)
		if err != nil {
			return out, fmt.Errorf("resolve %s: %w", imp, err)
		}
		if dir == "" {
			continue
		}
		hostname, err := readHostname(filepath.Join(dir, "endpoints.go"))
		if err != nil || hostname == "" {
			continue
		}
		// Filter out hostnames that aren't in the bundle. A dep on a
		// service the operator hasn't bundled is dead at runtime; the
		// topology shouldn't surface it.
		if !bundleHosts[hostname] {
			continue
		}
		alias := lastSegment(imp)
		if clientAliases[alias] {
			depHosts[hostname] = true
		}
		if hookAliases[alias] {
			eventHosts[hostname] = true
		}
	}
	out.Deps = sortedKeys(depHosts)
	out.Events = sortedKeys(eventHosts)

	// Cloud: scan for URL string literals targeting hosts outside the bundle.
	if hasEgress {
		out.Cloud = detectCloud(s.Dir, bundleHosts)
	}

	return out, nil
}

// collectImports gathers every distinct import path across non-test .go
// files in dir (top-level only, no recursion).
func collectImports(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parseFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for _, imp := range f.Imports {
			seen[strings.Trim(imp.Path.Value, `"`)] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// findOwnAPIName finds the service's own *api subdirectory (sibling
// directory ending in "api") and returns the directory base name (the
// alias used in imports).
func findOwnAPIName(serviceDir string) (string, error) {
	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, "api") || name == "api" {
			continue
		}
		if _, err := os.Stat(filepath.Join(serviceDir, name, "endpoints.go")); err == nil {
			return name, nil
		}
	}
	return "", nil
}

// scanAPIUsage walks every non-test .go file in serviceDir for
// `<alias>.NewClient(...)`, `<alias>.NewMulticastClient(...)`,
// `<alias>.NewMulticastTrigger(...)`, and `<alias>.NewHook(...)` calls,
// returning two sets of *api aliases: those used as clients (any of
// the first three) and those used as hooks. Own-package aliases are
// filtered out. Used to decide whether a dep is a client dep (solid
// edge) or hook-only (dotted edge).
func scanAPIUsage(serviceDir, ownAPIAlias string) (clients, hooks map[string]bool, err error) {
	clients = map[string]bool{}
	hooks = map[string]bool{}
	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parseFile(filepath.Join(serviceDir, name))
		if err != nil {
			return nil, nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name == ownAPIAlias {
				return true
			}
			if !strings.HasSuffix(id.Name, "api") || id.Name == "api" {
				return true
			}
			switch sel.Sel.Name {
			case "NewClient", "NewMulticastClient", "NewMulticastTrigger":
				clients[id.Name] = true
			case "NewHook":
				hooks[id.Name] = true
			}
			return true
		})
	}
	return clients, hooks, nil
}

// urlLiteralRE matches the host portion of an http:// or https:// URL
// inside a Go string literal. Greedy [^/"`] up to the next path or
// quote.
var urlLiteralRE = regexp.MustCompile(`https?://([^/"\s\\` + "`" + `]+)`)

// detectCloud scans every non-test .go file in dir for URL string
// literals (`https://...`, `http://...`), extracts the host, filters out
// hosts that match other services in the same bundle, and returns:
//
//   - the single external host if exactly one is found
//   - "various" if multiple distinct external hosts are found
//   - "various" if no host is statically extractable (the egress import
//     is present but URLs are dynamic)
//
// Hostnames with ports are stripped to host-only; user-info, query
// strings, and fragments are likewise stripped.
//
// Comments are stripped before the URL scan so the Apache license URL
// in the copyright header (and any other doc-style URL references in
// godoc) does not leak into the result. The regex still runs against
// raw post-stripping bytes - that's deliberate, so URLs split across
// composite-literal initializers or string concat still match.
func detectCloud(dir string, bundleHosts map[string]bool) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "various"
	}
	external := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := readGoSourceStripped(filepath.Join(dir, name))
		if err != nil || len(data) == 0 {
			continue
		}
		for _, m := range urlLiteralRE.FindAllSubmatch(data, -1) {
			host := stripHost(string(m[1]))
			if host == "" {
				continue
			}
			if bundleHosts[host] {
				continue
			}
			external[host] = true
		}
	}
	switch len(external) {
	case 0:
		return "various"
	case 1:
		for h := range external {
			return h
		}
	}
	return "various"
}

// readGoSourceStripped reads a Go source file and returns its bytes
// with every comment range overwritten by spaces (newlines preserved).
// Length is unchanged, so byte offsets in the returned slice still
// correspond to positions in the on-disk file. Used by detectCloud so
// URLs appearing only in comments (most commonly the Apache license
// URL in the copyright header) do not get reported as outbound
// dependencies.
func readGoSourceStripped(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		// Fall back to the raw bytes - a parse error here would
		// otherwise mask a real outbound URL just because a sibling
		// file in the same package had a syntax issue.
		return data, nil
	}
	tf := fset.File(f.Pos())
	if tf == nil {
		return data, nil
	}
	out := append([]byte(nil), data...)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			start := tf.Offset(c.Pos())
			end := tf.Offset(c.End())
			for i := start; i < end && i < len(out); i++ {
				if out[i] != '\n' {
					out[i] = ' '
				}
			}
		}
	}
	return out, nil
}

// stripHost takes the authority portion of a URL and returns just the
// host (no user-info, no port). Returns "" on malformed input.
func stripHost(authority string) string {
	// Use net/url for the heavy lifting by reconstructing a parseable URL.
	u, err := url.Parse("https://" + authority)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// readHostname reads the Hostname constant from an *api/endpoints.go.
func readHostname(path string) (string, error) {
	f, err := parseFile(path)
	if err != nil {
		return "", err
	}
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
			for i, name := range vs.Names {
				if name.Name != "Hostname" || i >= len(vs.Values) {
					continue
				}
				if bl, ok := vs.Values[i].(*ast.BasicLit); ok {
					return strings.Trim(bl.Value, `"`), nil
				}
			}
		}
	}
	return "", nil
}

// parseFile is a thin wrapper around go/parser.
func parseFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, parser.ParseComments)
}

// lastSegment returns the final "/"-separated component of a package path.
func lastSegment(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// sortedKeys returns the keys of a string-keyed bool set in lex order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
