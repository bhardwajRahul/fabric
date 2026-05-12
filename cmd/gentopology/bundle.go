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

// Bundle resolution: identical shape to cmd/gencreds - parse main.go for
// app.Add(...) calls, resolve each to a service directory via `go list`.
// Copied (not imported) per the framework's "no shared package between
// cmd/* tools" convention; the duplication is bounded and the two tools
// have different consumer profiles (gencreds signs creds, gentopology
// renders diagrams).

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

	"github.com/microbus-io/fabric/cmd/internal/schema"
)

// service is a single service in a bundle. Hostname comes from the
// service's manifest (the Hostname constant value, copied at extraction
// time); Name comes from manifest's general.name (operator-curated PascalCase
// or directory-name fallback); Dir is the absolute path used for AST scan.
// Danger is true when any of the service's exposed routes (webs, functions,
// tasks, workflows) is on the trust-root port :666.
type service struct {
	Hostname string
	Name     string
	Dir      string
	Danger   bool
}

// resolveBundle returns the bundle's services. --bundle takes precedence
// when both flags are given.
func resolveBundle(bundle, manifests string) ([]service, error) {
	if manifests != "" {
		return resolveManifestList(manifests)
	}
	return resolveMainGo(bundle)
}

// resolveManifestList parses a comma-separated list of service dirs.
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

// resolveMainGo parses main.go for app.Add(...) calls and resolves each to
// a service directory via `go list`. Init-wrapped forms
// (`pkg.NewService().Init(...)`) resolve to `pkg` like the bare form.
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
			continue // dedupe replicas (e.g. messaging × 3)
		}
		seen[s.Hostname] = true
		services = append(services, s)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Hostname < services[j].Hostname })
	return services, nil
}

// importMap returns alias → import path for every import in f.
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

// collectAppAddPackages returns the import paths of services added via
// app.Add(...) calls. Argument unwrapping handles `pkg.NewService()`,
// `pkg.NewService().Init(...)`, etc.
//
// Arguments that don't resolve to a known import alias are reported on
// stderr so the operator notices a service that will be excluded from
// the topology diagram.
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
				"gentopology: warning: %s:%d: cannot resolve app.Add argument to a package; service will be omitted\n",
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

// pkgFromExpr unwraps method-chain receivers to find the rightmost call
// whose callee is a SelectorExpr rooted at an imported alias.
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
		e = sel.X
	}
}

// goListDirs returns on-disk directories for the given package paths.
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

// serviceFromDir loads the manifest from the given directory for hostname
// and display-name. Display name falls back to the directory's lowercase
// base name when the manifest doesn't carry one.
func serviceFromDir(dir string) (service, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	abs, err := filepath.Abs(dir)
	if err != nil {
		return service{}, err
	}
	m, err := schema.ReadManifest(manifestPath)
	if err != nil {
		return service{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}
	name := m.General.Name
	if name == "" {
		name = strings.ToLower(filepath.Base(abs))
	}
	return service{
		Hostname: m.General.Hostname,
		Name:     name,
		Dir:      abs,
		Danger:   hasDangerRoute(m),
	}, nil
}

// hasDangerRoute reports whether any of the manifest's exposed routes
// is on the trust-root port :666. Outbound events and inbound-event
// subscriptions are excluded - those go on :417 by convention; the
// trust-root capability is about caller-reachable RPC/web/task/workflow
// endpoints.
func hasDangerRoute(m *schema.Manifest) bool {
	for _, r := range m.ExposedRoutes() {
		if isPort666(r.Route) {
			return true
		}
	}
	return false
}

// isPort666 reports whether a route string declares port :666. Matches
// the bare ":666" port-only form and the ":666/path" form. Routes
// without an explicit port use :443 by default and are not danger.
func isPort666(route string) bool {
	if !strings.HasPrefix(route, ":666") {
		return false
	}
	rest := route[len(":666"):]
	return rest == "" || rest[0] == '/'
}
