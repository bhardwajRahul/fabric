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

// Source-driven AST scan: walks a service's directory and produces an
// aclInput populated entirely from code (no manifest reads). Detects
// every call pattern that produces a NATS PUB or SUB rule:
//
//   - Typed-client method calls (`pkgapi.NewClient(svc).Method(...)`),
//     including ForHost variants and variable-bound clients.
//   - Raw svc.Request / svc.Publish with `pub.X(...)` options (literal
//     URL, variable URL, slice composite, append-built).
//   - Inbound event hooks (`pkgapi.NewHook(svc).OnX(...)`).
//   - Outbound event triggers (declared in *api/client.go as methods
//     on the MulticastTrigger receiver).
//   - Own-route subscriptions (`svc.Subscribe(...)` in intermediate.go).
//
// Ported from cmd/genmanifest's parsing logic. Two copies coexist by
// design during the JIT transition; the parity test pins them to the
// same output.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microbus-io/fabric/cmd/internal/pkgresolver"
)

// scanService walks the microservice rooted at serviceDir and returns
// an aclInput populated from source. fromDir is the working directory
// for `go list` resolution of cross-package deps; pass "" for the
// process's CWD. resolver may be nil; when non-nil it is shared across
// services in one run to memoize lookups and to short-circuit in-module
// paths past `go list`.
func scanService(serviceDir, fromDir string, resolver *pkgresolver.Resolver) (aclInput, error) {
	if fromDir == "" {
		fromDir = serviceDir
	}

	// Locate the service's own *api package (sibling directory ending in
	// "api"). Its endpoints.go has the Hostname constant and the Def
	// literals.
	ownAPIDir, ownAPIAlias, err := findOwnAPIDir(serviceDir)
	if err != nil {
		return aclInput{}, err
	}
	ownDefs, ownHost, err := parseAPIEndpoints(filepath.Join(ownAPIDir, "endpoints.go"))
	if err != nil {
		return aclInput{}, fmt.Errorf("parse own *api/endpoints.go: %w", err)
	}
	if ownHost == "" {
		return aclInput{}, fmt.Errorf("own *api package has no Hostname constant")
	}
	in := aclInput{Self: ownHost}

	// Outbound events: Defs whose names appear as MulticastTrigger
	// methods in *api/client.go.
	outboundNames, err := outboundEventNames(filepath.Join(ownAPIDir, "client.go"))
	if err != nil {
		return aclInput{}, err
	}
	outNamesSorted := make([]string, 0, len(outboundNames))
	for n := range outboundNames {
		outNamesSorted = append(outNamesSorted, n)
	}
	sort.Strings(outNamesSorted)
	for _, n := range outNamesSorted {
		d, ok := ownDefs[n]
		if !ok {
			continue
		}
		in.OutboundEvents = append(in.OutboundEvents, aclEventDef{
			Route:  d.Route,
			Method: d.Method,
		})
	}

	// Own subscribed routes: parse intermediate.go for svc.Subscribe(...)
	// calls. We just need each call's (Method, Route); skip the
	// kind/signature fields genmanifest cares about.
	intermediatePath := filepath.Join(serviceDir, "intermediate.go")
	ownRoutes, hookCalls, err := parseIntermediateForACL(intermediatePath, ownDefs, ownAPIAlias)
	if err != nil {
		return aclInput{}, err
	}
	in.OwnRoutes = ownRoutes

	// Inbound events: each Hook call resolves to a foreign *api package +
	// event Def name. Look up the foreign package via the importing file's
	// alias map; read its Hostname + Defs.
	for _, h := range hookCalls {
		ep, err := resolveHook(h, intermediatePath, fromDir, resolver)
		if err != nil || ep.Hostname == "" {
			continue
		}
		in.InboundEvents = append(in.InboundEvents, ep)
	}
	sort.Slice(in.InboundEvents, func(i, j int) bool {
		if in.InboundEvents[i].Hostname != in.InboundEvents[j].Hostname {
			return in.InboundEvents[i].Hostname < in.InboundEvents[j].Hostname
		}
		if in.InboundEvents[i].Route != in.InboundEvents[j].Route {
			return in.InboundEvents[i].Route < in.InboundEvents[j].Route
		}
		return in.InboundEvents[i].Method < in.InboundEvents[j].Method
	})

	// Typed-client downstream calls.
	clientCalls, err := scanClientCalls(serviceDir)
	if err != nil {
		return aclInput{}, err
	}
	rawReqs, err := scanRawRequests(serviceDir)
	if err != nil {
		return aclInput{}, err
	}
	in.Downstream, err = resolveDownstream(clientCalls, rawReqs, serviceDir, fromDir, resolver)
	if err != nil {
		return aclInput{}, err
	}
	return in, nil
}

// findOwnAPIDir returns the absolute path of the service's own *api
// subdirectory and its alias (the bare directory name, e.g. "kitchenapi").
// Convention: a service in dir/foo has its api package in dir/foo/fooapi/
// or any subdir whose name ends in "api". Entries are inspected in
// lexical order so the pick is deterministic when more than one
// *api-suffixed directory exists.
func findOwnAPIDir(serviceDir string) (apiDir, alias string, err error) {
	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return "", "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, "api") || name == "api" {
			continue
		}
		// Validate it has an endpoints.go.
		ep := filepath.Join(serviceDir, name, "endpoints.go")
		if _, err := os.Stat(ep); err != nil {
			continue
		}
		return filepath.Join(serviceDir, name), name, nil
	}
	return "", "", fmt.Errorf("no *api/endpoints.go subdirectory in %s", serviceDir)
}

// def is a (Method, Route) pair extracted from a Def literal.
type def struct {
	Method string
	Route  string
}

// parseAPIEndpoints reads endpoints.go and returns name→def + Hostname.
func parseAPIEndpoints(path string) (defs map[string]def, hostname string, err error) {
	f, err := parseFile(path)
	if err != nil {
		return nil, "", err
	}
	defs = map[string]def{}
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
				if id, ok := cl.Type.(*ast.Ident); !ok || id.Name != "Def" {
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

// outboundEventNames reads *api/client.go and returns the set of method
// names declared on the MulticastTrigger receiver. Each such name is a
// candidate outbound event whose Def lives in *api/endpoints.go.
//
// Builder helpers like ForHost / WithOptions return another
// MulticastTrigger to support method chaining. Genuine event triggers
// return a request/response sequence (typically iter.Seq[...]). The
// classification keys off that distinction rather than a hand-maintained
// allowlist of helper names, so a future framework helper added to
// MulticastTrigger does not silently become a phantom outbound event.
// The caller intersects the returned set with the Defs in
// endpoints.go, so any false positive without a matching Def is harmless.
func outboundEventNames(path string) (map[string]bool, error) {
	f, err := parseFile(path)
	if err != nil {
		// client.go missing is acceptable - not all services define one.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if recvTypeName(fn.Recv.List[0].Type) != "MulticastTrigger" {
			continue
		}
		// A method that returns (only) MulticastTrigger is a builder
		// helper, not an event trigger. Single-return shape is the
		// simplest reliable signal; event triggers return iter.Seq
		// or similar non-builder types.
		if returnsTriggerOnly(fn.Type) {
			continue
		}
		out[fn.Name.Name] = true
	}
	return out, nil
}

// returnsTriggerOnly reports whether fn's results are exactly
// MulticastTrigger (or *MulticastTrigger). Used to filter out builder
// helpers from the outbound-event candidate set.
func returnsTriggerOnly(fn *ast.FuncType) bool {
	if fn.Results == nil || len(fn.Results.List) != 1 {
		return false
	}
	field := fn.Results.List[0]
	if len(field.Names) > 1 {
		return false
	}
	return recvTypeName(field.Type) == "MulticastTrigger"
}

// hookCall captures one detected NewHook(...).OnX(...) chain.
type hookCall struct {
	apiAlias  string // alias of the foreign *api package
	eventName string // method name (the Def variable)
}

// parseIntermediateForACL walks intermediate.go and extracts (a) the
// (Method, Route) of every svc.Subscribe(...) call and (b) every hook
// subscription (NewHook(...).OnX(...)). Own-package hook calls (self-
// hooks) are filtered out - they aren't inbound events.
func parseIntermediateForACL(path string, ownDefs map[string]def, ownAPIAlias string) (own []aclOwnRoute, hooks []hookCall, err error) {
	f, err := parseFile(path)
	if err != nil {
		return nil, nil, err
	}
	// Build alias→importPath map for this file.
	aliasToPath := importAliases(f)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Subscribe":
				if r, ok := parseSubscribeAt(call, ownDefs, ownAPIAlias); ok {
					own = append(own, r)
				}
			default:
				// Possible hook chain: <something>.OnX(...) where the chain
				// roots in <pkg>.NewHook(...).
				if alias := findHookAlias(sel.X, ownAPIAlias); alias != "" {
					if _, known := aliasToPath[alias]; known {
						hooks = append(hooks, hookCall{apiAlias: alias, eventName: sel.Sel.Name})
					}
				}
			}
			return true
		})
	}
	return own, hooks, nil
}

// parseSubscribeAt extracts (Method, Route) from the sub.At(...) option of
// a svc.Subscribe(...) call. Returns ok=false if the call doesn't have an
// At option or the args can't be resolved.
func parseSubscribeAt(call *ast.CallExpr, ownDefs map[string]def, ownAPIAlias string) (aclOwnRoute, bool) {
	if len(call.Args) < 2 {
		return aclOwnRoute{}, false
	}
	for _, arg := range call.Args[2:] {
		c, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "At" || len(c.Args) < 2 {
			continue
		}
		method := resolveOwnDefField(c.Args[0], ownDefs, ownAPIAlias, "Method")
		route := resolveOwnDefField(c.Args[1], ownDefs, ownAPIAlias, "Route")
		if method == "" || route == "" {
			return aclOwnRoute{}, false
		}
		return aclOwnRoute{Method: method, Route: route}, true
	}
	return aclOwnRoute{}, false
}

// resolveOwnDefField resolves an expression like `xxxapi.MyEndpoint.Method`
// to the corresponding field of MyEndpoint's Def literal in ownDefs.
// `field` is "Method" or "Route". Returns "" on miss.
func resolveOwnDefField(e ast.Expr, ownDefs map[string]def, ownAPIAlias, field string) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != field {
		return ""
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := inner.X.(*ast.Ident)
	if !ok || pkg.Name != ownAPIAlias {
		return ""
	}
	d, ok := ownDefs[inner.Sel.Name]
	if !ok {
		return ""
	}
	switch field {
	case "Method":
		return d.Method
	case "Route":
		return d.Route
	}
	return ""
}

// findHookAlias walks down the receiver chain of a hook subscription
// (`<pkg>.NewHook(svc)` possibly with .ForHost/.WithOptions wrapping) and
// returns the *api package alias if found. Returns "" if not a hook chain
// or if it's a self-hook on the own *api package.
func findHookAlias(expr ast.Expr, ownAPIAlias string) string {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		if sel.Sel.Name == "NewHook" {
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name == ownAPIAlias {
				return ""
			}
			return id.Name
		}
		expr = sel.X
	}
}

// resolveHook turns a detected hookCall into a populated aclInboundEvent
// by reading the foreign *api package's Hostname + Def lookup.
func resolveHook(h hookCall, fromFile, fromDir string, resolver *pkgresolver.Resolver) (aclInboundEvent, error) {
	pkgPath, err := aliasImportPath(fromFile, h.apiAlias)
	if err != nil || pkgPath == "" {
		return aclInboundEvent{}, err
	}
	dir, err := resolver.Dir(pkgPath, fromDir)
	if err != nil || dir == "" {
		return aclInboundEvent{}, err
	}
	defs, hostname, err := parseAPIEndpoints(filepath.Join(dir, "endpoints.go"))
	if err != nil {
		return aclInboundEvent{}, err
	}
	d, ok := defs[h.eventName]
	if !ok {
		return aclInboundEvent{}, nil
	}
	return aclInboundEvent{
		Hostname: hostname,
		Route:    d.Route,
		Method:   d.Method,
	}, nil
}

// importAliases returns alias→importPath for every import in f, scoped to
// *api packages only (anything else can't be a typed-client / hook source).
func importAliases(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		path := unquote(imp.Path.Value)
		alias := lastSegment(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if !strings.HasSuffix(alias, "api") || alias == "api" {
			continue
		}
		out[alias] = path
	}
	return out
}

// aliasImportPath looks up a single alias's import path in a file's
// import list.
func aliasImportPath(filePath, alias string) (string, error) {
	f, err := parseFile(filePath)
	if err != nil {
		return "", err
	}
	for a, p := range importAliases(f) {
		if a == alias {
			return p, nil
		}
	}
	return "", nil
}

// parseFile is a thin wrapper around go/parser that always includes
// comments.
func parseFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, parser.ParseComments)
}

// unquote strips surrounding quotes from a Go string literal token.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// lastSegment returns the final "/"-separated component of a package path.
func lastSegment(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// recvTypeName returns the receiver type's identifier, stripping pointer
// indirection. `func (t *T) F()` and `func (t T) F()` both yield "T".
func recvTypeName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}
