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
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// definePkg is the import path of the declarative vocabulary package.
const definePkg = "github.com/microbus-io/fabric/define"

// upgradeV1_41_0 migrates one microservice from the endpoints.go model to the definition.go model: it
// synthesizes <x>api/definition.go from the microservice's manifest.yaml (the feature spec), its
// <x>api/endpoints.go (In/Out and domain type declarations, Hostname, and Def method/route), and its
// intermediate.go (the Version counter and any subscription options the manifest does not record), then
// deletes endpoints.go. It does not regenerate the rest of the boilerplate; the upgrade skill runs
// genservice as a separate step afterward.
func upgradeV1_41_0(serviceDir string) error {
	apiDir, err := findAPIDir(serviceDir)
	if err != nil {
		return err
	}
	endpointsPath := filepath.Join(apiDir, "endpoints.go")
	definitionPath := filepath.Join(apiDir, "definition.go")

	// Idempotent: a microservice already migrated (definition.go present, endpoints.go gone) is a no-op.
	if fileExists(definitionPath) && !fileExists(endpointsPath) {
		return nil
	}
	if !fileExists(endpointsPath) {
		return fmt.Errorf("no endpoints.go under %s; nothing to migrate", apiDir)
	}

	man, err := readManifest(filepath.Join(serviceDir, "manifest.yaml"))
	if err != nil {
		return err
	}
	ep, err := parseEndpoints(endpointsPath)
	if err != nil {
		return err
	}
	version := scanVersion(filepath.Join(serviceDir, "intermediate.go"))
	subs := scanSubscribeOpts(filepath.Join(serviceDir, "intermediate.go"))

	src, err := synthesizeDefinition(man, ep, version, subs)
	if err != nil {
		return err
	}
	err = os.WriteFile(definitionPath, src, 0o644)
	if err != nil {
		return err
	}
	err = os.Remove(endpointsPath)
	if err != nil {
		return err
	}
	fmt.Printf("genupgrade: %s: wrote definition.go, removed endpoints.go\n", apiDir)
	return nil
}

// findAPIDir returns the api subdirectory of serviceDir: the one holding endpoints.go (not yet
// migrated) or definition.go (already migrated, so the run can no-op). Recognizing both lets a re-run on
// a migrated microservice return cleanly instead of failing to find the package.
func findAPIDir(serviceDir string) (string, error) {
	entries, err := os.ReadDir(serviceDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cand := filepath.Join(serviceDir, e.Name())
		if fileExists(filepath.Join(cand, "endpoints.go")) || fileExists(filepath.Join(cand, "definition.go")) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no api subdirectory with an endpoints.go or definition.go found under %s", serviceDir)
}

// ============================== manifest.yaml ==============================

// manifest is the ordered, decoded view of a microservice's manifest.yaml. Sections keep declaration
// order (recovered from the YAML node tree) so the synthesized definition.go is stable across runs.
type manifest struct {
	name        string
	hostname    string
	description string

	configs   []namedNode
	metrics   []namedNode
	outbound  []namedNode
	functions []namedNode
	webs      []namedNode
	inbound   []namedNode
	tasks     []namedNode
	workflows []namedNode
	tickers   []namedNode
}

// namedNode is one keyed entry of a manifest section: the feature name and its still-encoded fields.
type namedNode struct {
	name string
	node *yaml.Node
}

func readManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	root := &doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	m := &manifest{}
	g := childMapping(root, "general")
	if g != nil {
		var gen struct {
			Name        string `yaml:"name"`
			Hostname    string `yaml:"hostname"`
			Description string `yaml:"description"`
		}
		_ = g.Decode(&gen)
		m.name, m.hostname, m.description = gen.Name, gen.Hostname, gen.Description
	}
	m.configs = mappingEntries(childMapping(root, "configs"))
	m.metrics = mappingEntries(childMapping(root, "metrics"))
	m.outbound = mappingEntries(childMapping(root, "outboundEvents"))
	m.functions = mappingEntries(childMapping(root, "functions"))
	m.webs = mappingEntries(childMapping(root, "webs"))
	m.inbound = mappingEntries(childMapping(root, "inboundEvents"))
	m.tasks = mappingEntries(childMapping(root, "tasks"))
	m.workflows = mappingEntries(childMapping(root, "workflows"))
	m.tickers = mappingEntries(childMapping(root, "tickers"))
	return m, nil
}

// childMapping returns the mapping node stored under key in a mapping node, or nil.
func childMapping(parent *yaml.Node, key string) *yaml.Node {
	if parent == nil {
		return nil
	}
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	return nil
}

// mappingEntries returns the key/value pairs of a mapping node in declaration order.
func mappingEntries(m *yaml.Node) []namedNode {
	if m == nil {
		return nil
	}
	var out []namedNode
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, namedNode{name: m.Content[i].Value, node: m.Content[i+1]})
	}
	return out
}

// Decoded shapes for the per-feature field maps.

type mfEndpoint struct {
	Description string `yaml:"description"`
	Method     string `yaml:"method"`
	Route      string `yaml:"route"`
	Package    string `yaml:"package"` // inbound only
}

type mfConfig struct {
	Description string `yaml:"description"`
	Signature   string `yaml:"signature"`
	Validation  string `yaml:"validation"`
	Default     string `yaml:"default"`
	Secret      bool   `yaml:"secret"`
	Callback    bool   `yaml:"callback"`
}

type mfMetric struct {
	Description string    `yaml:"description"`
	Signature   string    `yaml:"signature"`
	Kind        string    `yaml:"kind"`
	Buckets     []float64 `yaml:"buckets"`
	OtelName    string    `yaml:"otelName"`
	Observable  bool      `yaml:"observable"`
}

type mfTicker struct {
	Description string `yaml:"description"`
	Interval    string `yaml:"interval"`
}

func decodeEndpoint(n *yaml.Node) mfEndpoint { var v mfEndpoint; _ = n.Decode(&v); return v }
func decodeConfig(n *yaml.Node) mfConfig     { var v mfConfig; _ = n.Decode(&v); return v }
func decodeMetric(n *yaml.Node) mfMetric     { var v mfMetric; _ = n.Decode(&v); return v }
func decodeTicker(n *yaml.Node) mfTicker     { var v mfTicker; _ = n.Decode(&v); return v }

// ============================== endpoints.go ==============================

// endpoints is the data lifted from a microservice's <x>api/endpoints.go. Method and route are NOT
// taken from here: the route is in the manifest, and the method is the manifest's (functions, webs,
// outbound events) or the kind constant (POST for tasks, GET for workflows - a foreman invariant).
type endpoints struct {
	header   string            // leading license comment block, verbatim
	pkg      string            // api package name
	hostname string            // value of the Hostname const
	imports  map[string]string // alias -> import path
	typeText map[string]string // type name -> verbatim source (single-spec type decls, excluding Def)
	extras   []string          // verbatim source of grouped/other carried type decls
}

func parseEndpoints(path string) (*endpoints, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	e := &endpoints{
		pkg:      f.Name.Name,
		imports:  importMap(f),
		typeText: map[string]string{},
	}
	s := string(data)
	i := strings.Index(s, "\npackage ")
	if i >= 0 {
		e.header = strings.TrimRight(s[:i], " \t\n")
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gen.Tok {
		case token.TYPE:
			if len(gen.Specs) == 1 {
				ts := gen.Specs[0].(*ast.TypeSpec)
				if ts.Name.Name == "Def" {
					continue // the routing struct is replaced by the define package
				}
				e.typeText[ts.Name.Name] = sliceDecl(data, fset, gen)
			} else {
				e.extras = append(e.extras, sliceDecl(data, fset, gen))
			}
		case token.CONST:
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for idx, n := range vs.Names {
					if n.Name == "Hostname" && idx < len(vs.Values) {
						bl, ok := vs.Values[idx].(*ast.BasicLit)
						if ok {
							e.hostname = strings.Trim(bl.Value, "`\"")
						}
					}
				}
			}
		}
	}
	return e, nil
}

// sliceDecl returns the verbatim source of a declaration, including its doc comment.
func sliceDecl(data []byte, fset *token.FileSet, gen *ast.GenDecl) string {
	start := gen.Pos()
	if gen.Doc != nil {
		start = gen.Doc.Pos()
	}
	a := fset.Position(start).Offset
	b := fset.Position(gen.End()).Offset
	return string(data[a:b])
}

// importMap maps each import's alias (or default package name) to its path.
func importMap(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := lastSegment(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		out[alias] = path
	}
	return out
}

func lastSegment(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// ============================== intermediate.go ==============================

var versionRE = regexp.MustCompile(`(?m)^\s*Version\s*=\s*([0-9]+)`)

// scanVersion extracts the api Version counter from intermediate.go, defaulting to 1.
func scanVersion(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 1
	}
	m := versionRE.FindSubmatch(data)
	if m == nil {
		return 1
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil || n == 0 {
		return 1
	}
	return n
}

// subOpts are the subscription options the manifest does not always record (it omits all three for
// workflows). They are recovered straight from the sub.* options on each svc.Subscribe call.
type subOpts struct {
	claims        string
	timeBudget    string // verbatim Go duration expression, e.g. "5 * time.Second"
	loadBalancing string // "none" | custom queue | ""
}

// scanSubscribeOpts maps each subscription name to its options, parsed from intermediate.go.
func scanSubscribeOpts(path string) map[string]subOpts {
	out := map[string]subOpts{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Subscribe" || len(call.Args) < 1 {
			return true
		}
		bl, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		name := strings.Trim(bl.Value, "`\"")
		var o subOpts
		for _, a := range call.Args[1:] {
			c, ok := a.(*ast.CallExpr)
			if !ok {
				continue
			}
			s, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			switch s.Sel.Name {
			case "RequiredClaims":
				o.claims = stringArg(c)
			case "TimeBudget":
				if len(c.Args) > 0 {
					o.timeBudget = exprText(fset, c.Args[0])
				}
			case "NoQueue":
				o.loadBalancing = "none"
			case "Queue":
				o.loadBalancing = stringArg(c)
			}
		}
		out[name] = o
		return true
	})
	return out
}

func stringArg(c *ast.CallExpr) string {
	if len(c.Args) == 0 {
		return ""
	}
	bl, ok := c.Args[0].(*ast.BasicLit)
	if !ok {
		return ""
	}
	return strings.Trim(bl.Value, "`\"")
}

func exprText(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	err := printer.Fprint(&b, fset, e)
	if err != nil {
		return ""
	}
	return b.String()
}

// ============================== synthesis ==============================

// synthesizeDefinition renders and gofmts definition.go from the gathered inputs.
func synthesizeDefinition(man *manifest, ep *endpoints, version int, subs map[string]subOpts) ([]byte, error) {
	consumed := map[string]bool{}
	var body strings.Builder
	imports := newImportSet(ep.imports)
	imports.add(definePkg)

	hostname := ep.hostname
	if hostname == "" {
		hostname = man.hostname
	}

	// Configs.
	for _, e := range man.configs {
		c := decodeConfig(e.node)
		emitConfig(&body, e.name, c, imports)
	}
	// Metrics.
	for _, e := range man.metrics {
		mt := decodeMetric(e.node)
		emitMetric(&body, e.name, mt)
	}
	// Outbound events, functions, tasks, workflows: all carry In/Out. Webs do not. Tasks are always
	// POST and workflows always GET (a foreman invariant); the manifest omits their method, so those
	// kind constants are the default when the manifest has none.
	for _, e := range man.outbound {
		emitRoutable(&body, "OutboundEvent", e.name, "", decodeEndpoint(e.node), ep, subs, imports, consumed, true)
	}
	for _, e := range man.functions {
		emitRoutable(&body, "Function", e.name, "", decodeEndpoint(e.node), ep, subs, imports, consumed, true)
	}
	for _, e := range man.webs {
		emitRoutable(&body, "Web", e.name, "", decodeEndpoint(e.node), ep, subs, imports, consumed, false)
	}
	for _, e := range man.inbound {
		emitInbound(&body, e.name, decodeEndpoint(e.node), imports)
	}
	for _, e := range man.tasks {
		emitRoutable(&body, "Task", e.name, "POST", decodeEndpoint(e.node), ep, subs, imports, consumed, true)
	}
	for _, e := range man.workflows {
		emitRoutable(&body, "Workflow", e.name, "GET", decodeEndpoint(e.node), ep, subs, imports, consumed, true)
	}
	// Tickers.
	for _, e := range man.tickers {
		t := decodeTicker(e.node)
		emitTicker(&body, e.name, t, imports)
	}

	// Any type declarations from endpoints.go not consumed as an In/Out pair (domain types that lived
	// there) carry over verbatim at the end.
	for _, name := range sortedKeys(ep.typeText) {
		if consumed[name] {
			continue
		}
		imports.scan(ep.typeText[name])
		body.WriteString(ep.typeText[name])
		body.WriteString("\n\n")
	}
	for _, x := range ep.extras {
		imports.scan(x)
		body.WriteString(x)
		body.WriteString("\n\n")
	}

	var head strings.Builder
	if ep.header != "" {
		head.WriteString(ep.header)
		head.WriteString("\n\n")
	}
	head.WriteString("package ")
	head.WriteString(ep.pkg)
	head.WriteString("\n\n")
	head.WriteString(imports.block())
	head.WriteString("\n")
	fmt.Fprintf(&head, "// Hostname is the default hostname of the microservice.\nconst Hostname = %q\n\n", hostname)
	fmt.Fprintf(&head, "// Name is the decorative PascalCase name of the microservice.\nconst Name = %q\n\n", man.name)
	fmt.Fprintf(&head, "// Version is the major version of the microservice's public API.\nconst Version = %d\n\n", version)
	fmt.Fprintf(&head, "// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.\nconst Description = %s\n\n", rawString(man.description))

	full := head.String() + body.String()
	out, err := format.Source([]byte(full))
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\n%s", err, full)
	}
	return out, nil
}

// emitRoutable writes a function/web/task/workflow/outbound var, followed by its In/Out type
// declarations (lifted verbatim from endpoints.go) for the kinds that carry them.
func emitRoutable(b *strings.Builder, kind, name, defaultMethod string, mf mfEndpoint, ep *endpoints, subs map[string]subOpts, imports *importSet, consumed map[string]bool, hasInOut bool) {
	method := mf.Method
	if method == "" {
		method = defaultMethod
	}
	route := mf.Route
	o := subs[name]

	b.WriteString(godoc(mf.Description, name))
	fmt.Fprintf(b, "var %s = define.%s{\n", name, kind)
	fmt.Fprintf(b, "\tHost: Hostname, Method: %q, Route: %q,\n", method, route)
	if o.claims != "" {
		fmt.Fprintf(b, "\tRequiredClaims: %q,\n", o.claims)
	}
	if o.timeBudget != "" {
		fmt.Fprintf(b, "\tTimeBudget: %s,\n", o.timeBudget)
		imports.add("time")
	}
	switch {
	case o.loadBalancing == "none":
		b.WriteString("\tLoadBalancing: define.None,\n")
	case o.loadBalancing != "" && o.loadBalancing != "default":
		fmt.Fprintf(b, "\tLoadBalancing: %q,\n", o.loadBalancing)
	}
	if hasInOut {
		fmt.Fprintf(b, "\tIn: %sIn{}, Out: %sOut{},\n", name, name)
	}
	b.WriteString("}\n\n")

	if hasInOut {
		emitInOut(b, name+"In", ep, imports, consumed)
		emitInOut(b, name+"Out", ep, imports, consumed)
	}
}

// emitInOut writes the named In/Out struct verbatim and marks it consumed.
func emitInOut(b *strings.Builder, typeName string, ep *endpoints, imports *importSet, consumed map[string]bool) {
	text, ok := ep.typeText[typeName]
	if !ok {
		return
	}
	imports.scan(text)
	b.WriteString(text)
	b.WriteString("\n\n")
	consumed[typeName] = true
}

func emitConfig(b *strings.Builder, name string, c mfConfig, imports *importSet) {
	valType := configValueType(c.Signature)
	imports.addType(valType)
	b.WriteString(godoc(c.Description, name))
	fmt.Fprintf(b, "var %s = define.Config{\n", name)
	fmt.Fprintf(b, "\tValue: %s,\n", valueCarrier(valType))
	if c.Default != "" {
		fmt.Fprintf(b, "\tDefault: %q,\n", c.Default)
	}
	if c.Validation != "" {
		fmt.Fprintf(b, "\tValidation: %q,\n", c.Validation)
	}
	if c.Secret {
		b.WriteString("\tSecret: true,\n")
	}
	if c.Callback {
		b.WriteString("\tCallback: true,\n")
	}
	b.WriteString("}\n\n")
}

func emitMetric(b *strings.Builder, name string, m mfMetric) {
	valType, labels := metricValueAndLabels(m.Signature)
	b.WriteString(godoc(m.Description, name))
	fmt.Fprintf(b, "var %s = define.Metric{\n", name)
	fmt.Fprintf(b, "\tKind: define.%s, Value: %s,", metricKindConst(m.Kind), valueCarrier(valType))
	if len(labels) > 0 {
		b.WriteString(" Labels: []string{")
		for i, l := range labels {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "%q", l)
		}
		b.WriteString("},")
	}
	b.WriteString("\n")
	if len(m.Buckets) > 0 {
		b.WriteString("\tBuckets: []float64{")
		for i, v := range m.Buckets {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		}
		b.WriteString("},\n")
	}
	fmt.Fprintf(b, "\tOTelName: %q,", m.OtelName)
	if m.Observable {
		b.WriteString(" Observable: true,")
	}
	b.WriteString("\n}\n\n")
}

func emitTicker(b *strings.Builder, name string, t mfTicker, imports *importSet) {
	b.WriteString(godoc(t.Description, name))
	fmt.Fprintf(b, "var %s = define.Ticker{\n", name)
	fmt.Fprintf(b, "\tInterval: %s,\n", durationExpr(t.Interval))
	b.WriteString("}\n\n")
	imports.add("time")
}

func emitInbound(b *strings.Builder, name string, e mfEndpoint, imports *importSet) {
	// The manifest records the source microservice's package path; the OutboundEvent var lives in that
	// microservice's api package, which by convention is <service>/<service>api.
	apiPath := e.Package + "/" + lastSegment(e.Package) + "api"
	alias := lastSegment(apiPath)
	imports.add(apiPath)
	b.WriteString(godoc(e.Description, name))
	fmt.Fprintf(b, "var %s = define.InboundEvent{\n", name)
	fmt.Fprintf(b, "\tSource: %s.%s,\n", alias, name)
	b.WriteString("}\n\n")
}

// ============================== small helpers ==============================

// godoc renders a doc comment block for a declaration. A multi-line description becomes one // line
// each; an empty description falls back to a bare "<Name> ..." stub so the var is still documented.
func godoc(desc, name string) string {
	if desc == "" {
		return "// " + name + "\n"
	}
	var b strings.Builder
	for _, line := range strings.Split(desc, "\n") {
		b.WriteString("// ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// rawString renders s as a backtick raw string literal, falling back to a quoted literal when s
// contains a backtick.
func rawString(s string) string {
	if strings.Contains(s, "`") {
		return strconv.Quote(s)
	}
	return "`" + s + "`"
}

// configValueType extracts the getter's return type from a config signature. The return is named for
// the config, not "value" (e.g. "TimeBudget() (budget time.Duration)" -> "time.Duration",
// "MaxItems() (value int)" -> "int"). Empty when the signature is absent (a struct-valued config).
func configValueType(sig string) string {
	i := strings.LastIndex(sig, "(")
	if i < 0 {
		return ""
	}
	inner := strings.TrimSuffix(strings.TrimSpace(sig[i+1:]), ")")
	fields := strings.Fields(inner)
	switch len(fields) {
	case 0:
		return ""
	case 1:
		return fields[0]
	default:
		return fields[len(fields)-1] // "<name> <type>" -> type
	}
}

// metricValueAndLabels parses a recorder signature, e.g. "RequestsTotal(value int, status string)" ->
// "int", ["status"]. Defaults the value type to int.
func metricValueAndLabels(sig string) (string, []string) {
	const marker = "(value "
	i := strings.Index(sig, marker)
	if i < 0 {
		return "int", nil
	}
	rest := sig[i+len(marker):]
	j := strings.LastIndex(rest, ")")
	if j >= 0 {
		rest = rest[:j]
	}
	parts := strings.Split(rest, ",")
	valType := strings.TrimSpace(parts[0])
	if valType == "" {
		valType = "int"
	}
	var labels []string
	for _, p := range parts[1:] {
		fields := strings.Fields(strings.TrimSpace(p))
		if len(fields) > 0 {
			labels = append(labels, fields[0])
		}
	}
	return valType, labels
}

// valueCarrier renders the explicit type carrier for a config/metric value type, e.g. int -> "int(0)",
// string -> `string("")`, a struct type T -> "T{}".
func valueCarrier(typ string) string {
	switch typ {
	case "":
		return `string("")`
	case "string":
		return `string("")`
	case "bool":
		return "bool(false)"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "time.Duration":
		return typ + "(0)"
	default:
		return typ + "{}"
	}
}

func metricKindConst(kind string) string {
	switch kind {
	case "counter":
		return "Counter"
	case "gauge":
		return "Gauge"
	case "histogram":
		return "Histogram"
	}
	return "Counter"
}

var durationRE = regexp.MustCompile(`^([0-9]+)(ns|us|ms|s|m|h)$`)

// durationExpr renders a compact manifest duration as a Go expression, e.g. "30s" -> "30 * time.Second".
func durationExpr(s string) string {
	m := durationRE.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	unit := map[string]string{
		"ns": "Nanosecond", "us": "Microsecond", "ms": "Millisecond",
		"s": "Second", "m": "Minute", "h": "Hour",
	}[m[2]]
	return m[1] + " * time." + unit
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ============================== import set ==============================

// importSet accumulates the import paths definition.go needs. aliases is the alias->path map lifted
// from endpoints.go, used to resolve selectors found in carried type declarations.
type importSet struct {
	aliases map[string]string
	paths   map[string]bool
}

func newImportSet(aliases map[string]string) *importSet {
	return &importSet{aliases: aliases, paths: map[string]bool{}}
}

func (s *importSet) add(path string) {
	if path != "" {
		s.paths[path] = true
	}
}

// scan adds the import paths of any pkg.Selector references in a carried type declaration, resolved
// against the endpoints.go alias map. Unresolved selectors are left alone: a lifted struct always comes
// with its package already imported by endpoints.go.
func (s *importSet) scan(text string) {
	for _, alias := range selectors(text) {
		if path, ok := s.aliases[alias]; ok {
			s.paths[path] = true
		}
	}
}

// addType adds the import for a single type expression's package qualifier, e.g. "time.Duration" ->
// "time". Unlike scan, an unresolved qualifier is assumed to be a standard-library package (its import
// path equals the package name), since a config/metric value type comes from the getter/recorder rather
// than from endpoints.go and may reference a package endpoints.go did not import.
func (s *importSet) addType(typ string) {
	for _, alias := range selectors(typ) {
		if path, ok := s.aliases[alias]; ok {
			s.add(path)
		} else {
			s.add(alias)
		}
	}
}

// block renders the gofmt-able import block. The standard library and external groups are merged here;
// format.Source regroups and sorts them.
func (s *importSet) block() string {
	paths := make([]string, 0, len(s.paths))
	for p := range s.paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range paths {
		alias := ""
		for a, ap := range s.aliases {
			if ap == p && a != lastSegment(p) {
				alias = a + " "
				break
			}
		}
		fmt.Fprintf(&b, "\t%s%q\n", alias, p)
	}
	b.WriteString(")\n")
	return b.String()
}

// selectors returns the leading identifiers of pkg.Type selectors in a type expression text.
func selectors(text string) []string {
	var out []string
	seen := map[string]bool{}
	i := 0
	for i < len(text) {
		c := text[i]
		if !isIdentStart(c) {
			i++
			continue
		}
		j := i
		for j < len(text) && isIdentCont(text[j]) {
			j++
		}
		ident := text[i:j]
		if j < len(text) && text[j] == '.' && !seen[ident] {
			seen[ident] = true
			out = append(out, ident)
		}
		i = j + 1
	}
	return out
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
