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

// workflowFn names a workflow graph builder method on *Service.
type workflowFn struct {
	Method     string // PascalCase method name, e.g. "FanOut"
	OutputFile string // <ALLCAPS>.mmd, e.g. "FANOUT.mmd"
}

// scanWorkflows returns every method on *Service in dir whose signature matches
//
//	func (svc *Service) Foo(ctx context.Context) (graph *workflow.Graph, err error)
//
// The match is purely syntactic; alias imports of the workflow package are not
// followed. This matches the microservice authoring convention enforced by the
// add-workflow skill.
func scanWorkflows(dir string) ([]workflowFn, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var workflows []workflowFn
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			if !isServiceReceiver(fn.Recv) {
				continue
			}
			if !isWorkflowGraphSignature(fn.Type) {
				continue
			}
			workflows = append(workflows, workflowFn{
				Method:     fn.Name.Name,
				OutputFile: strings.ToUpper(fn.Name.Name) + ".mmd",
			})
		}
	}
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Method < workflows[j].Method
	})
	return workflows, nil
}

// packageName returns the package declared in the first non-test .go file in dir.
func packageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return f.Name.Name, nil
	}
	return "", fmt.Errorf("no .go files in %s", dir)
}

// importPathOf walks upward from dir to find go.mod, parses the module path,
// and returns <module>/<rel-from-modroot-to-dir>.
func importPathOf(dir string) (string, error) {
	cur := dir
	for {
		mod := filepath.Join(cur, "go.mod")
		data, err := os.ReadFile(mod)
		if err == nil {
			modPath, err := moduleDirective(data)
			if err != nil {
				return "", fmt.Errorf("%s: %w", mod, err)
			}
			rel, err := filepath.Rel(cur, dir)
			if err != nil {
				return "", err
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return modPath, nil
			}
			return modPath + "/" + rel, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no go.mod found at or above %s", dir)
		}
		cur = parent
	}
}

// moduleDirective extracts the path from the first `module <path>` line.
func moduleDirective(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			path = strings.Trim(path, `"`)
			return path, nil
		}
	}
	return "", fmt.Errorf("no module directive")
}

// isServiceReceiver reports whether the receiver is `(... *Service)`.
func isServiceReceiver(recv *ast.FieldList) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}
	star, ok := recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := star.X.(*ast.Ident)
	return ok && id.Name == "Service"
}

// isWorkflowGraphSignature reports whether the function takes exactly
// (context.Context) and returns (*workflow.Graph, error).
func isWorkflowGraphSignature(ft *ast.FuncType) bool {
	params := flattenFieldList(ft.Params)
	if len(params) != 1 || !isSelectorType(params[0], "context", "Context") {
		return false
	}
	results := flattenFieldList(ft.Results)
	if len(results) != 2 {
		return false
	}
	if !isPointerToSelector(results[0], "workflow", "Graph") {
		return false
	}
	id, ok := results[1].(*ast.Ident)
	return ok && id.Name == "error"
}

// flattenFieldList returns one type per parameter, expanding `(a, b int)` style
// groups into two entries with the same type expression.
func flattenFieldList(fl *ast.FieldList) []ast.Expr {
	if fl == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

func isSelectorType(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && sel.Sel.Name == name
}

func isPointerToSelector(e ast.Expr, pkg, name string) bool {
	star, ok := e.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isSelectorType(star.X, pkg, name)
}
