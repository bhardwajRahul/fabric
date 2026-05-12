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

// Typed-client method-call detection + raw svc.Request detection +
// downstream resolution. Ported from cmd/genmanifest's parse_calls.go.

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microbus-io/fabric/cmd/internal/pkgresolver"
)

// clientCall is one invocation of an endpoint method on a typed
// downstream client value.
type clientCall struct {
	alias            string // *api package alias
	method           string // method name
	hostnameOverride string // "" no ForHost, "*" ForHost(varExpr), literal otherwise
}

// scanClientCalls walks every non-test .go file at the service root and
// returns each detected typed-client method call.
func scanClientCalls(dir string) ([]clientCall, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var calls []clientCall
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "mock.go" {
			continue
		}
		fileCalls, err := scanFileClientCalls(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		calls = append(calls, fileCalls...)
	}
	return calls, nil
}

// scanFileClientCalls parses one .go file and returns its typed-client
// method calls.
func scanFileClientCalls(path string) ([]clientCall, error) {
	f, err := parseFile(path)
	if err != nil {
		return nil, err
	}
	aliasToPath := importAliases(f)
	if len(aliasToPath) == 0 {
		return nil, nil
	}
	var calls []clientCall
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		bindings := collectClientBindings(fn, aliasToPath)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if c, ok := classifyClientCall(call, aliasToPath, bindings); ok {
				calls = append(calls, c)
			}
			return true
		})
	}
	return calls, nil
}

// clientBinding records that a local var holds a typed client of a known
// *api package.
type clientBinding struct {
	alias string
}

// collectClientBindings tracks every identifier in a function (parameters,
// named results, local var decls, := assignments) that holds a typed
// downstream client.
func collectClientBindings(fn *ast.FuncDecl, aliasToPath map[string]string) map[string]clientBinding {
	bindings := map[string]clientBinding{}
	addTyped := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, field := range fl.List {
			alias := typedClientAlias(field.Type, aliasToPath)
			if alias == "" {
				continue
			}
			for _, n := range field.Names {
				if n.Name == "_" {
					continue
				}
				bindings[n.Name] = clientBinding{alias: alias}
			}
		}
	}
	if fn.Type != nil {
		addTyped(fn.Type.Params)
		addTyped(fn.Type.Results)
	}
	if fn.Body != nil {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Lhs) == 0 || len(s.Rhs) != len(s.Lhs) {
					return true
				}
				for i, lhs := range s.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || id.Name == "_" {
						continue
					}
					if alias := newClientAlias(s.Rhs[i], aliasToPath); alias != "" {
						bindings[id.Name] = clientBinding{alias: alias}
					}
				}
			case *ast.DeclStmt:
				gen, ok := s.Decl.(*ast.GenDecl)
				if !ok {
					return true
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					if vs.Type != nil {
						alias := typedClientAlias(vs.Type, aliasToPath)
						if alias != "" {
							for _, n := range vs.Names {
								if n.Name == "_" {
									continue
								}
								bindings[n.Name] = clientBinding{alias: alias}
							}
						}
					}
					for i, n := range vs.Names {
						if i >= len(vs.Values) || n.Name == "_" {
							continue
						}
						if alias := newClientAlias(vs.Values[i], aliasToPath); alias != "" {
							bindings[n.Name] = clientBinding{alias: alias}
						}
					}
				}
			}
			return true
		})
	}
	return bindings
}

// typedClientAlias returns the *api alias if expr denotes
// `<alias>.Client` or `<alias>.MulticastClient`. Pointer indirection is
// transparent.
func typedClientAlias(expr ast.Expr, aliasToPath map[string]string) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if sel.Sel.Name != "Client" && sel.Sel.Name != "MulticastClient" {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if _, known := aliasToPath[id.Name]; !known {
		return ""
	}
	return id.Name
}

// newClientAlias inspects an expression and returns the *api alias if it
// (after stripping chain helpers) is `<alias>.NewClient(...)` /
// `<alias>.NewMulticastClient(...)`.
func newClientAlias(e ast.Expr, aliasToPath map[string]string) string {
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		switch sel.Sel.Name {
		case "NewClient", "NewMulticastClient":
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return ""
			}
			if _, known := aliasToPath[id.Name]; !known {
				return ""
			}
			return id.Name
		case "ForHost", "WithOptions":
			e = sel.X
			continue
		default:
			return ""
		}
	}
}

// classifyClientCall checks whether a call is an endpoint method
// invocation on a typed downstream client. Returns the (alias, method,
// hostnameOverride) triple.
func classifyClientCall(call *ast.CallExpr, aliasToPath map[string]string, bindings map[string]clientBinding) (clientCall, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return clientCall{}, false
	}
	method := sel.Sel.Name
	switch method {
	case "ForHost", "WithOptions", "NewClient", "NewMulticastClient", "NewHook", "NewMulticastTrigger":
		return clientCall{}, false
	}
	override := ""
	expr := sel.X
	for {
		switch x := expr.(type) {
		case *ast.CallExpr:
			inner, ok := x.Fun.(*ast.SelectorExpr)
			if !ok {
				return clientCall{}, false
			}
			switch inner.Sel.Name {
			case "NewClient", "NewMulticastClient":
				id, ok := inner.X.(*ast.Ident)
				if !ok {
					return clientCall{}, false
				}
				if _, known := aliasToPath[id.Name]; !known {
					return clientCall{}, false
				}
				return clientCall{alias: id.Name, method: method, hostnameOverride: override}, true
			case "ForHost":
				if override == "" {
					override = forHostValue(x)
				}
				expr = inner.X
				continue
			case "WithOptions":
				expr = inner.X
				continue
			default:
				return clientCall{}, false
			}
		case *ast.Ident:
			b, ok := bindings[x.Name]
			if !ok {
				return clientCall{}, false
			}
			return clientCall{alias: b.alias, method: method, hostnameOverride: override}, true
		default:
			return clientCall{}, false
		}
	}
}

// forHostValue returns the literal string passed to ForHost, or "*" if
// the argument is a non-literal expression.
func forHostValue(call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return "*"
	}
	if bl, ok := call.Args[0].(*ast.BasicLit); ok {
		return unquote(bl.Value)
	}
	return "*"
}

// rawRequest is one svc.Request(...) / svc.Publish(...) invocation with
// pub.* options. Fields are "*" when not statically resolvable.
type rawRequest struct {
	hostname string
	port     string
	path     string
	method   string
}

// scanRawRequests walks every non-test .go file at the service root and
// returns one entry per svc.Request / svc.Publish call carrying pub.*
// options.
func scanRawRequests(dir string) ([]rawRequest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []rawRequest
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "mock.go" {
			continue
		}
		f, err := parseFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			optsBindings := collectPubOptionsBindings(fn.Body)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name != "Request" && sel.Sel.Name != "Publish" {
					return true
				}
				if rc, ok := classifyRawRequest(call, optsBindings); ok {
					out = append(out, rc)
				}
				return true
			})
		}
	}
	return out, nil
}

// collectPubOptionsBindings finds slice composite-literal assignments of
// the shape `opts := []pub.Option{...}` and `opts = append(opts, ...)`
// and returns var name → list of pub.X option calls.
func collectPubOptionsBindings(body *ast.BlockStmt) map[string][]*ast.CallExpr {
	out := map[string][]*ast.CallExpr{}
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
			return true
		}
		if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
			id, ok := assign.Lhs[0].(*ast.Ident)
			if ok && id.Name != "_" {
				if cl, ok := assign.Rhs[0].(*ast.CompositeLit); ok {
					if isPubOptionSlice(cl.Type) {
						for _, elt := range cl.Elts {
							if ce, ok := elt.(*ast.CallExpr); ok && isPubCall(ce) {
								out[id.Name] = append(out[id.Name], ce)
							}
						}
						return true
					}
				}
				if ce, ok := assign.Rhs[0].(*ast.CallExpr); ok {
					if fnIdent, ok := ce.Fun.(*ast.Ident); ok && fnIdent.Name == "append" {
						for _, a := range ce.Args[1:] {
							if pubCe, ok := a.(*ast.CallExpr); ok && isPubCall(pubCe) {
								out[id.Name] = append(out[id.Name], pubCe)
							}
						}
					}
				}
			}
		}
		return true
	})
	return out
}

func isPubOptionSlice(e ast.Expr) bool {
	arr, ok := e.(*ast.ArrayType)
	if !ok {
		return false
	}
	sel, ok := arr.Elt.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "pub" && sel.Sel.Name == "Option"
}

func isPubCall(ce *ast.CallExpr) bool {
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "pub"
}

// classifyRawRequest reads the argument list of a svc.Request/.Publish
// call and resolves the (hostname, port, path, method) tuple. Inline
// pub.X(...) options are read directly; spread of a slice variable
// (`opts...`) folds in the options collected for that variable.
func classifyRawRequest(call *ast.CallExpr, optsBindings map[string][]*ast.CallExpr) (rawRequest, bool) {
	rc := rawRequest{hostname: "*", port: "*", path: "*", method: "*"}
	sawPub := false
	apply := func(ce *ast.CallExpr) {
		s, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		pkg, ok := s.X.(*ast.Ident)
		if !ok || pkg.Name != "pub" {
			return
		}
		sawPub = true
		switch s.Sel.Name {
		case "Method":
			if v := stringArg(ce, 0); v != "" {
				rc.method = strings.ToUpper(v)
			}
		case "URL":
			if v := stringArg(ce, 0); v != "" {
				if h, p, path, ok := parseInternalURL(v); ok {
					rc.hostname, rc.port, rc.path = h, p, path
				}
			}
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			rc.method = s.Sel.Name
			if v := stringArg(ce, 0); v != "" {
				if h, p, path, ok := parseInternalURL(v); ok {
					rc.hostname, rc.port, rc.path = h, p, path
				}
			}
		}
	}
	for _, arg := range call.Args {
		if ce, ok := arg.(*ast.CallExpr); ok {
			apply(ce)
			continue
		}
		if id, ok := arg.(*ast.Ident); ok {
			for _, ce := range optsBindings[id.Name] {
				apply(ce)
			}
		}
	}
	if !sawPub {
		return rawRequest{}, false
	}
	return rc, true
}

// stringArg returns the i-th argument of a call as a Go string literal,
// or "" if not a literal.
func stringArg(call *ast.CallExpr, i int) string {
	if i >= len(call.Args) {
		return ""
	}
	bl, ok := call.Args[i].(*ast.BasicLit)
	if !ok {
		return ""
	}
	return unquote(bl.Value)
}

// parseInternalURL extracts (host, port, path) from a Microbus URL.
func parseInternalURL(s string) (host, port, path string, ok bool) {
	rest := s
	switch {
	case strings.HasPrefix(rest, "https://"):
		rest = rest[len("https://"):]
		port = "443"
	case strings.HasPrefix(rest, "http://"):
		rest = rest[len("http://"):]
		port = "80"
	default:
		return "", "", "", false
	}
	pathStart := strings.IndexByte(rest, '/')
	authority := rest
	if pathStart >= 0 {
		authority = rest[:pathStart]
		path = rest[pathStart:]
	}
	if i := strings.IndexByte(authority, ':'); i >= 0 {
		host = authority[:i]
		port = authority[i+1:]
	} else {
		host = authority
	}
	if host == "" {
		return "", "", "", false
	}
	if path == "" {
		path = "/"
	}
	return host, port, path, true
}

// resolveDownstream folds detected client calls and raw requests into
// aclDownstream entries. Each typed-client call resolves to an endpoint
// of its target's *api package; raw requests bucket by URL host. resolver
// may be nil; when non-nil it memoizes cross-service package lookups and
// short-circuits in-module paths past `go list`.
func resolveDownstream(calls []clientCall, raws []rawRequest, fromDir, listDir string, resolver *pkgresolver.Resolver) ([]aclDownstream, error) {
	// Group typed-client calls by alias.
	byAlias := map[string][]clientCall{}
	for _, c := range calls {
		byAlias[c.alias] = append(byAlias[c.alias], c)
	}

	// Resolve alias→importPath using all *api imports across the service's
	// files (project-wide map, since a service has many files importing
	// different api packages).
	aliasToPath, err := projectAliasToPath(fromDir)
	if err != nil {
		return nil, err
	}

	// Per-host bucket: hostname → set of (route, method, hostnameOverride).
	byHost := map[string]map[string]aclEndpoint{}

	// Typed-client calls: resolve each alias's calls into endpoints.
	for alias, aliasCalls := range byAlias {
		apiPath, ok := aliasToPath[alias]
		if !ok {
			continue
		}
		dir, err := resolver.Dir(apiPath, listDir)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", apiPath, err)
		}
		if dir == "" {
			continue
		}
		defs, hostname, err := parseAPIEndpoints(filepath.Join(dir, "endpoints.go"))
		if err != nil || hostname == "" {
			continue
		}
		helpers, err := buildClientHelperMap(dir, defs)
		if err != nil {
			continue
		}
		for _, c := range aliasCalls {
			methods := resolveClientMethod(c.method, defs, helpers)
			for _, m := range methods {
				d := defs[m]
				ep := aclEndpoint{
					Route:            d.Route,
					Method:           d.Method,
					HostnameOverride: c.hostnameOverride,
				}
				key := ep.Route + "\x00" + ep.Method + "\x00" + ep.HostnameOverride
				if byHost[hostname] == nil {
					byHost[hostname] = map[string]aclEndpoint{}
				}
				byHost[hostname][key] = ep
			}
		}
	}

	// Raw requests: each call already carries (hostname, port, path,
	// method); convert to a synthetic endpoint with route ":<port><path>".
	for _, r := range raws {
		var route string
		switch {
		case r.port == "*" && r.path == "*":
			route = "*"
		case r.port == "*":
			route = ":*" + r.path
		case r.path == "*":
			route = ":" + r.port
		default:
			route = ":" + r.port + r.path
		}
		ep := aclEndpoint{Route: route, Method: r.method}
		key := ep.Route + "\x00" + ep.Method + "\x00"
		if byHost[r.hostname] == nil {
			byHost[r.hostname] = map[string]aclEndpoint{}
		}
		byHost[r.hostname][key] = ep
	}

	// Materialize sorted output.
	out := make([]aclDownstream, 0, len(byHost))
	hosts := make([]string, 0, len(byHost))
	for h := range byHost {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	for _, h := range hosts {
		eps := byHost[h]
		entry := aclDownstream{Hostname: h}
		keys := make([]string, 0, len(eps))
		for k := range eps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			entry.Endpoints = append(entry.Endpoints, eps[k])
		}
		sortDownstreamEndpoints(entry.Endpoints)
		out = append(out, entry)
	}
	return out, nil
}

// projectAliasToPath gathers *api imports across all non-test .go files in
// dir and returns a project-wide alias→importPath map.
func projectAliasToPath(dir string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
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
		for a, p := range importAliases(f) {
			out[a] = p
		}
	}
	return out, nil
}

// buildClientHelperMap walks every non-test .go file in apiDir and finds
// methods on Client/MulticastClient that aren't themselves Defs (helpers
// like AwaitAndParse). Maps each helper to the set of receiver-method
// calls in its body - those are the real Defs the helper invokes.
func buildClientHelperMap(apiDir string, defs map[string]def) (map[string][]string, error) {
	helpers := map[string][]string{}
	for name := range defs {
		helpers[name] = []string{name}
	}
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		return helpers, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parseFile(filepath.Join(apiDir, name))
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil {
				continue
			}
			recv := recvTypeName(fn.Recv.List[0].Type)
			if recv != "Client" && recv != "MulticastClient" {
				continue
			}
			methodName := fn.Name.Name
			if _, isDef := defs[methodName]; isDef {
				continue
			}
			recvName := ""
			if len(fn.Recv.List[0].Names) > 0 {
				recvName = fn.Recv.List[0].Names[0].Name
			}
			if recvName == "" {
				continue
			}
			var invoked []string
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != recvName {
					return true
				}
				invoked = append(invoked, sel.Sel.Name)
				return true
			})
			helpers[methodName] = invoked
		}
	}
	return helpers, nil
}

// resolveClientMethod expands a method name into the set of Def names it
// resolves to. Direct Defs return themselves; helpers expand to their
// transitive Def invocations. Cycles are guarded.
func resolveClientMethod(method string, defs map[string]def, helpers map[string][]string) []string {
	visited := map[string]bool{}
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(m string) {
		if visited[m] {
			return
		}
		visited[m] = true
		if _, isDef := defs[m]; isDef {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
			return
		}
		for _, sub := range helpers[m] {
			walk(sub)
		}
	}
	walk(method)
	return out
}

// sortDownstreamEndpoints sorts endpoints deterministically.
func sortDownstreamEndpoints(eps []aclEndpoint) {
	sort.Slice(eps, func(i, j int) bool {
		if eps[i].Route != eps[j].Route {
			return eps[i].Route < eps[j].Route
		}
		if eps[i].Method != eps[j].Method {
			return eps[i].Method < eps[j].Method
		}
		return eps[i].HostnameOverride < eps[j].HostnameOverride
	})
}
