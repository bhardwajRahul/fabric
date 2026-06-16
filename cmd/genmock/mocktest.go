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
	"go/format"
	"sort"
	"strings"
)

// emitMockTest produces the gofmt'd mock_test.go source for g. If existing has
// a leading comment block above the package clause it is preserved verbatim,
// the same way emit does for mock.go.
func emitMockTest(g *generated, existing []byte) ([]byte, error) {
	var sb strings.Builder

	if h := reuseHeader(existing); h != "" {
		sb.WriteString(h)
		sb.WriteString("\n\n")
	}
	sb.WriteString(genmockMarker)
	sb.WriteString("\n\n")
	sb.WriteString("package ")
	sb.WriteString(g.pkgName)
	sb.WriteString("\n\n")

	ordered := orderMethods(g.methods)
	imports := collectTestImports(g, ordered)
	writeTestImports(&sb, imports, g.apiPath)

	fmt.Fprintf(&sb, "func Test%s_Mock(t *testing.T) {\n", pascalCase(g.pkgName))
	sb.WriteString("\tt.Parallel()\n")
	sb.WriteString("\tctx := t.Context()\n\n")
	sb.WriteString("\tmock := NewMock()\n")
	sb.WriteString("\tmock.SetDeployment(connector.TESTING)\n\n")

	writeOnStartupSubtest(&sb)
	writeOnShutdownSubtest(&sb)

	for _, m := range ordered {
		writeMethodSubtest(&sb, g, m)
	}

	sb.WriteString("}\n")

	src := []byte(sb.String())
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\nsource:\n%s", err, src)
	}
	return out, nil
}

// collectTestImports computes the set of imports referenced by the generated
// mock_test.go file. It does not reuse g.imports because mock.go and
// mock_test.go have largely disjoint dependency sets.
func collectTestImports(g *generated, methods []method) map[string]bool {
	imports := map[string]bool{
		"testing": true,
		"github.com/microbus-io/fabric/connector": true,
		"github.com/microbus-io/testarossa":       true,
	}

	intMap := importMapFromGenerator(g)

	needsWorkflow := false
	for _, m := range methods {
		if m.isWeb {
			imports["net/http"] = true
			imports["github.com/microbus-io/fabric/httpx"] = true
		}
		if m.isGraph || m.resultsHas.flow {
			needsWorkflow = true
		}
		for _, p := range m.params {
			addSelectorImports(imports, intMap, p.typ)
		}
		for _, r := range m.results {
			addSelectorImports(imports, intMap, r.typ)
		}
	}
	if needsWorkflow {
		imports["github.com/microbus-io/dwarf/workflow"] = true
	}

	// A workflow-graph mock's typed handler signature qualifies its In/Out
	// fields with apiPkg only when the field type isn't a builtin. Pull in the
	// api package import iff at least one In/Out field has a non-builtin type.
	if g.apiPath != "" && g.apiPkg != "" {
		for _, m := range methods {
			if !m.isGraph {
				continue
			}
			pair, ok := g.graphPairs[m.name]
			if !ok {
				continue
			}
			if hasNonBuiltin(pair.in) || hasNonBuiltin(pair.out) {
				imports[g.apiPath] = true
				break
			}
		}
	}
	return imports
}

// hasNonBuiltin reports whether any field's rendered type expression contains
// a bare identifier that isn't a Go builtin or keyword - i.e. a name that the
// generator will qualify with the api package alias.
func hasNonBuiltin(fields []structField) bool {
	for _, f := range fields {
		i := 0
		for i < len(f.typ) {
			c := f.typ[i]
			if !isIdentStart(c) {
				i++
				continue
			}
			j := i
			for j < len(f.typ) && isIdentCont(f.typ[j]) {
				j++
			}
			ident := f.typ[i:j]
			precededByDot := i > 0 && f.typ[i-1] == '.'
			followedByDot := j < len(f.typ) && f.typ[j] == '.'
			if !precededByDot && !followedByDot && !isBuiltinOrKeyword(ident) {
				return true
			}
			i = j
		}
	}
	return false
}

// importMapFromGenerator rebuilds the alias -> path map from g.imports plus the
// own api package. It is the inverse of the slice-of-paths map: callers index by
// the selector prefix (e.g. "time", "bearertokenapi") that appears in a type
// expression.
func importMapFromGenerator(g *generated) map[string]string {
	m := map[string]string{}
	for path := range g.imports {
		m[lastPathSegment(path)] = path
	}
	if g.apiPath != "" && g.apiPkg != "" {
		m[g.apiPkg] = g.apiPath
	}
	return m
}

// addSelectorImports adds to imports the paths for every Pkg.Selector reference
// in a Go type expression. Bare identifiers (builtins, locally-declared types)
// produce no entry.
func addSelectorImports(imports map[string]bool, intMap map[string]string, typ string) {
	for _, sel := range selectorsIn(typ) {
		if path, ok := intMap[sel]; ok {
			imports[path] = true
		}
	}
}

func writeTestImports(sb *strings.Builder, imports map[string]bool, ownAPI string) {
	var std, thirdParty []string
	for path := range imports {
		if path == ownAPI {
			continue
		}
		if isStdLib(path) {
			std = append(std, path)
		} else {
			thirdParty = append(thirdParty, path)
		}
	}
	sort.Strings(std)
	sort.Strings(thirdParty)

	sb.WriteString("import (\n")
	for _, p := range std {
		fmt.Fprintf(sb, "\t%q\n", p)
	}
	if len(std) > 0 && len(thirdParty) > 0 {
		sb.WriteString("\n")
	}
	for _, p := range thirdParty {
		fmt.Fprintf(sb, "\t%q\n", p)
	}
	if ownAPI != "" && imports[ownAPI] {
		if len(std) > 0 || len(thirdParty) > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(sb, "\t%q\n", ownAPI)
	}
	sb.WriteString(")\n\n")
}

func writeOnStartupSubtest(sb *strings.Builder) {
	sb.WriteString("\tt.Run(\"on_startup\", func(t *testing.T) {\n")
	sb.WriteString("\t\tassert := testarossa.For(t)\n")
	sb.WriteString("\t\terr := mock.OnStartup(ctx)\n")
	sb.WriteString("\t\tassert.NoError(err)\n")
	sb.WriteString("\t})\n\n")
}

func writeOnShutdownSubtest(sb *strings.Builder) {
	sb.WriteString("\tt.Run(\"on_shutdown\", func(t *testing.T) {\n")
	sb.WriteString("\t\tassert := testarossa.For(t)\n")
	sb.WriteString("\t\terr := mock.OnShutdown(ctx)\n")
	sb.WriteString("\t\tassert.NoError(err)\n")
	sb.WriteString("\t})\n\n")
}

func writeMethodSubtest(sb *strings.Builder, g *generated, m method) {
	fmt.Fprintf(sb, "\tt.Run(%q, func(t *testing.T) { // MARKER: %s\n", snakeCase(m.name), m.marker)
	sb.WriteString("\t\tassert := testarossa.For(t)\n\n")

	switch {
	case m.isGraph:
		writeGraphSubtestBody(sb, g, m)
	case m.isWeb:
		writeWebSubtestBody(sb, m)
	default:
		writeStandardSubtestBody(sb, m)
	}

	sb.WriteString("\t})\n\n")
}

// writeStandardSubtestBody emits a smoke test for a normal ToDo method. The
// handler signature mirrors the method's params and results; inputs are zero-
// valued via local var declarations, *workflow.Flow args are passed as nil, and
// the return values (except err) are discarded with blank identifiers.
func writeStandardSubtestBody(sb *strings.Builder, m method) {
	fmt.Fprintf(sb, "\t\tmock.Mock%s(func(%s) (%s) {\n", m.name, paramList(m.params), paramList(m.results))
	sb.WriteString("\t\t\treturn\n")
	sb.WriteString("\t\t})\n")

	for _, p := range varDecls(m.params) {
		fmt.Fprintf(sb, "\t\tvar %s %s\n", p.name, p.typ)
	}

	lhs := mockCallLHS(m.results)
	args := mockCallArgs(m.params)
	if lhs == "err" {
		fmt.Fprintf(sb, "\t\terr := mock.%s(%s)\n", m.name, args)
	} else {
		fmt.Fprintf(sb, "\t\t%s := mock.%s(%s)\n", lhs, m.name, args)
	}
	sb.WriteString("\t\tassert.NoError(err)\n")
}

func writeWebSubtestBody(sb *strings.Builder, m method) {
	fmt.Fprintf(sb, "\t\tmock.Mock%s(func(w http.ResponseWriter, r *http.Request) (err error) {\n", m.name)
	sb.WriteString("\t\t\treturn nil\n")
	sb.WriteString("\t\t})\n")
	sb.WriteString("\t\tw := httpx.NewResponseRecorder()\n")
	sb.WriteString("\t\tr := httpx.MustNewRequest(\"GET\", \"/\", nil)\n")
	fmt.Fprintf(sb, "\t\terr := mock.%s(w, r)\n", m.name)
	sb.WriteString("\t\tassert.NoError(err)\n")
}

func writeGraphSubtestBody(sb *strings.Builder, g *generated, m method) {
	pair, ok := g.graphPairs[m.name]
	if !ok {
		// Fall back to standard subtest pattern when there is no In/Out pair.
		writeStandardSubtestBody(sb, m)
		return
	}
	handlerSig := buildHandlerSig(pair, g.apiPkg)
	fmt.Fprintf(sb, "\t\tmock.Mock%s(%s {\n", m.name, handlerSig)
	sb.WriteString("\t\t\treturn\n")
	sb.WriteString("\t\t})\n")
	fmt.Fprintf(sb, "\t\tgraph, err := mock.%s(ctx)\n", m.name)
	sb.WriteString("\t\tif assert.NoError(err) {\n")
	sb.WriteString("\t\t\tassert.NotNil(graph)\n")
	sb.WriteString("\t\t}\n")
}

// varDecls returns the parameters that need a local var declaration before
// the mock call: everything except ctx and *workflow.Flow.
func varDecls(ps []param) []param {
	var out []param
	for _, p := range ps {
		if p.typ == "context.Context" || p.typ == "*workflow.Flow" {
			continue
		}
		if p.name == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// mockCallArgs renders the argument list for the mock.X(...) call: ctx for the
// context parameter, nil for *workflow.Flow parameters, and the param name
// otherwise.
func mockCallArgs(ps []param) string {
	var parts []string
	for _, p := range ps {
		switch p.typ {
		case "context.Context":
			parts = append(parts, "ctx")
		case "*workflow.Flow":
			parts = append(parts, "nil")
		default:
			parts = append(parts, p.name)
		}
	}
	return strings.Join(parts, ", ")
}

// mockCallLHS returns the LHS of the mock.X(...) assignment. Non-err results
// are discarded with blank identifiers; err is always the last name.
func mockCallLHS(results []param) string {
	var parts []string
	for _, r := range results {
		if r.name == "err" && r.typ == "error" {
			continue
		}
		parts = append(parts, "_")
	}
	parts = append(parts, "err")
	return strings.Join(parts, ", ")
}

// pascalCase capitalizes the first letter of s. The package directory name is
// already all-lowercase by convention, so this is the cheap "PascalCase of a
// package name" form. Vanity-cased identifiers like ChatGPTLLM are accepted as
// Chatgptllm in the generated test name.
func pascalCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - ('a' - 'A')
	}
	return string(r)
}

// snakeCase converts a PascalCase or camelCase identifier to snake_case.
func snakeCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				sb.WriteByte('_')
			}
			sb.WriteRune(r + ('a' - 'A'))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
