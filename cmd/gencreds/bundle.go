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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microbus-io/fabric/cmd/schema"
)

// service is a single service in a bundle, with everything gencreds needs to
// sign one JWT.
type service struct {
	Hostname string // canonical hostname, e.g. "kitchen.fixture"
	Dir      string // absolute path to the service's source directory
}

// resolveBundle returns the bundle's services. Either --bundle (a path to a
// main.go that calls app.Add) or --manifests (a comma-separated list of
// service directories) is honored, in that order.
func resolveBundle(bundle, manifests string) ([]service, error) {
	if manifests != "" {
		return resolveManifestList(manifests)
	}
	return resolveMainGo(bundle)
}

// resolveManifestList parses a comma-separated list of service directories.
// Each directory must contain manifest.yaml and nats.acl.
func resolveManifestList(list string) ([]service, error) {
	var services []service
	for _, dir := range strings.Split(list, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		s, err := serviceFromDir(dir)
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Hostname < services[j].Hostname })
	return services, nil
}

// resolveMainGo parses a main.go file and locates each service added via
// app.Add(...). For each call argument of the shape `pkg.NewService()`, the
// imported package is resolved to its on-disk directory via `go list`, and
// that directory's manifest.yaml + nats.acl are read.
//
// Init wrapping is supported: `pkg.NewService().Init(...)` resolves to `pkg`
// the same as the bare form. The argument expression's leading selector chain
// must root in a single import alias.
func resolveMainGo(mainGo string) ([]service, error) {
	abs, err := filepath.Abs(mainGo)
	if err != nil {
		return nil, fmt.Errorf("abs %s: %w", mainGo, err)
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", abs, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", abs, err)
	}

	imports := importMap(f)
	pkgPaths := collectAppAddPackages(fset, f, imports)
	if len(pkgPaths) == 0 {
		return nil, fmt.Errorf("no app.Add(...) services found in %s", abs)
	}

	mainDir := filepath.Dir(abs)
	dirs, err := goListDirs(mainDir, pkgPaths)
	if err != nil {
		return nil, err
	}

	var services []service
	seen := map[string]bool{}
	for _, dir := range dirs {
		s, err := serviceFromDir(dir)
		if err != nil {
			return nil, err
		}
		if seen[s.Hostname] {
			continue // duplicate replicas (e.g. messaging x3 in main.go)
		}
		seen[s.Hostname] = true
		services = append(services, s)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Hostname < services[j].Hostname })
	return services, nil
}

// importMap returns alias → import path for every import in the file. Aliased
// imports use the alias; unaliased imports use the trailing path segment as
// the alias. Dot and underscore imports are skipped.
func importMap(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			alias = path[strings.LastIndex(path, "/")+1:]
		}
		if alias == "_" || alias == "." {
			continue
		}
		out[alias] = path
	}
	return out
}

// collectAppAddPackages walks the file looking for `app.Add(...)` calls (or
// `something.Add(...)` chains where the callee's name is `Add`) and returns
// the import paths of each argument's leading selector chain. The receiver of
// `.Add` is matched on method name only because callers may have stored the
// application in a renamed variable.
//
// Argument shapes that don't resolve to a known import alias (variadic
// spreads, range-loop adds, factory expressions, var-bound services) are
// reported on stderr so the operator notices a service that won't be signed.
func collectAppAddPackages(fset *token.FileSet, f *ast.File, imports map[string]string) []string {
	pkgs := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Add" {
			return true
		}
		for _, arg := range call.Args {
			if pkg := pkgFromExpr(arg, imports); pkg != "" {
				pkgs[pkg] = true
				continue
			}
			pos := fset.Position(arg.Pos())
			fmt.Fprintf(os.Stderr,
				"gencreds: warning: %s:%d: cannot resolve app.Add argument to a package; service will not be signed\n",
				pos.Filename, pos.Line)
		}
		return true
	})
	out := make([]string, 0, len(pkgs))
	for k := range pkgs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pkgFromExpr unwraps method-chain receivers to find the rightmost call whose
// callee is a SelectorExpr rooted at an imported alias. Handles
// `pkg.NewService()`, `pkg.NewService().Init(...)`, and similar shapes.
func pkgFromExpr(e ast.Expr, imports map[string]string) string {
	for {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			if path, ok := imports[id.Name]; ok {
				return path
			}
			return ""
		}
		// Walk further down the chain (e.g. .Init wrapping).
		e = sel.X
	}
}

// goListDirs returns the on-disk directory of each given package path, in the
// same order. Uses `go list -f '{{.Dir}}'` so the local module's vendor or
// replace directives are honored.
func goListDirs(workDir string, pkgPaths []string) ([]string, error) {
	if len(pkgPaths) == 0 {
		return nil, nil
	}
	args := append([]string{"list", "-f", "{{.Dir}}"}, pkgPaths...)
	cmd := exec.Command("go", args...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return nil, fmt.Errorf("go list: %w: %s", err, stderr)
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs, nil
}

// serviceFromDir reads the manifest from the given directory (just for the
// hostname) and returns the service descriptor. ACL rules are derived from
// source at sign time, not read from a file.
func serviceFromDir(dir string) (service, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	m, err := schema.ReadManifest(manifestPath)
	if err != nil {
		return service{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return service{}, err
	}
	return service{
		Hostname: m.General.Hostname,
		Dir:      abs,
	}, nil
}
