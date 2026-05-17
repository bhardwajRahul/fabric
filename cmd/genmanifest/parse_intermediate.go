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
	"sort"
	"strings"
)

// parseIntermediate is the main extraction pass over the microservice's
// intermediate.go. It populates x with everything that file declares -
// description, subscriptions (functions/webs/tasks/workflows), configs,
// metrics, tickers, inbound event hooks, callbacks, and observable metrics.
//
// The api package alias is taken to be `apiPkg`; references to `<apiPkg>.X.Method`
// and `<apiPkg>.X.Route` are resolved via `defs`. Subscriptions whose route
// or method comes from a different alias are still emitted but the literal
// values are taken as-is.
func parseIntermediate(path string, x *extracted, defs map[string]def, apiPkg string, inOuts map[string]inOutPair) error {
	f, _, err := parseFile(path)
	if err != nil {
		return err
	}

	// Collect ToDo interface method comments so we can pick up MARKER names and
	// reconstruct task signatures.
	todoMethods := collectToDoMethods(f)

	// Collect intermediate getter signatures for configs (SQLDataSourceName, Workers, …).
	configSigs := collectConfigSigs(f)

	// Collect recorder method signatures (IncrementXxx, RecordXxx) for metric labels.
	metricSigs := collectMetricSigs(f)

	// Walk the body of NewIntermediate.
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		switch fn.Name.Name {
		case "NewIntermediate":
			walkNewIntermediate(fn, x, defs, apiPkg, inOuts, todoMethods, metricSigs, configSigs)
		case "doOnObserveMetrics":
			collectObservableMetrics(fn, x)
		case "doOnConfigChanged":
			collectConfigCallbacks(fn, x)
		}
	}

	// Strip api-package qualifiers from extracted signatures (e.g. `[]foremanapi.FlowStep` → `[]FlowStep`).
	stripAPIPackageQualifiers(x, apiPkg)
	return nil
}

// stripAPIPackageQualifiers removes `<apiPkg>.` prefixes from rendered Go type
// expressions in all extracted signatures. The manifest convention is to drop
// the package qualifier for types defined in the service's own api package.
func stripAPIPackageQualifiers(x *extracted, apiPkg string) {
	prefix := apiPkg + "."
	clean := func(s string) string {
		// Replace `apiPkg.X` with `X` wherever it appears.
		return strings.ReplaceAll(s, prefix, "")
	}
	for i := range x.functions {
		x.functions[i].Signature = clean(x.functions[i].Signature)
	}
	for i := range x.tasks {
		x.tasks[i].Signature = clean(x.tasks[i].Signature)
	}
	for i := range x.workflows {
		x.workflows[i].Signature = clean(x.workflows[i].Signature)
	}
	for i := range x.outboundEvents {
		x.outboundEvents[i].Signature = clean(x.outboundEvents[i].Signature)
	}
	for i := range x.inboundEvents {
		x.inboundEvents[i].Signature = clean(x.inboundEvents[i].Signature)
	}
}

// collectConfigSigs scans the intermediate for getter functions like
// `func (svc *Intermediate) Workers() (workers int)` and returns a map of
// config name → manifest-style signature (e.g. "Workers() (workers int)").
func collectConfigSigs(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		// Only single-result config getters; skip Set* and Increment*/Record*.
		name := fn.Name.Name
		if strings.HasPrefix(name, "Set") || strings.HasPrefix(name, "Increment") || strings.HasPrefix(name, "Record") || strings.HasPrefix(name, "do") || strings.HasPrefix(name, "On") {
			continue
		}
		if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
			continue
		}
		// Single named return.
		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			continue
		}
		res := fn.Type.Results.List[0]
		if len(res.Names) != 1 {
			continue
		}
		retName := res.Names[0].Name
		retType := exprString(res.Type)
		out[name] = name + "() (" + retName + " " + retType + ")"
	}
	return out
}

// todoMethod captures a member of the ToDo interface - its declared go signature
// plus its MARKER name (which may differ from the method name, as with
// OnChangedNumShards → MARKER: NumShards).
type todoMethod struct {
	methodName string
	marker     string
	params     []structField // ordered (name, type) excluding ctx and flow
	results    []structField // ordered, excluding the trailing err return
}

// collectToDoMethods scans the ToDo interface and returns a map keyed by method
// name with the parsed signature.
func collectToDoMethods(f *ast.File) map[string]todoMethod {
	out := map[string]todoMethod{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "ToDo" {
				continue
			}
			it, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range it.Methods.List {
				if len(m.Names) == 0 {
					continue
				}
				name := m.Names[0].Name
				ft, ok := m.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				method := todoMethod{
					methodName: name,
					marker:     extractMarker(m.Comment),
					params:     fieldsToList(ft.Params, true),
					results:    fieldsToList(ft.Results, false),
				}
				out[name] = method
			}
		}
	}
	return out
}

// fieldsToList converts an ast.FieldList into structField pairs. If skipCtxFlow
// is true, parameters typed as `context.Context` and `*workflow.Flow` are
// omitted (they are framework-injected). The trailing `err error` return is
// always omitted from result lists.
func fieldsToList(fl *ast.FieldList, skipCtxFlow bool) []structField {
	if fl == nil {
		return nil
	}
	var out []structField
	for _, field := range fl.List {
		typ := exprString(field.Type)
		if skipCtxFlow {
			if typ == "context.Context" {
				continue
			}
			if typ == "*workflow.Flow" {
				continue
			}
			// Web handler signature is (w http.ResponseWriter, r *http.Request).
			if typ == "http.ResponseWriter" || typ == "*http.Request" {
				continue
			}
		}
		if len(field.Names) == 0 {
			out = append(out, structField{Name: "", Type: typ})
			continue
		}
		for _, n := range field.Names {
			// Trim trailing err in result lists when it's named "err" of type "error".
			if !skipCtxFlow && n.Name == "err" && typ == "error" {
				continue
			}
			out = append(out, structField{Name: n.Name, Type: typ})
		}
	}
	return out
}

// extractMarker pulls "MARKER: Foo" out of a comment group. Returns "" if not present.
func extractMarker(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	for _, c := range g.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		text = strings.TrimSuffix(text, "*/")
		idx := strings.Index(text, "MARKER:")
		if idx < 0 {
			continue
		}
		marker := strings.TrimSpace(text[idx+len("MARKER:"):])
		// Trim trailing comment bits if any.
		if i := strings.IndexAny(marker, " \t"); i >= 0 {
			marker = marker[:i]
		}
		return marker
	}
	return ""
}

// metricSig is the parsed signature of a metric recorder method (Increment/Record).
type metricSig struct {
	labels []string // label names (e.g. ["workflowName", "status"])
	value  string   // "int", "float64", etc., or "" if not parseable
}

// collectMetricSigs scans Increment*/Record* methods on *Intermediate and
// captures the value type plus the trailing label parameter names.
func collectMetricSigs(f *ast.File) map[string]metricSig {
	out := map[string]metricSig{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		name := fn.Name.Name
		var metricName string
		switch {
		case strings.HasPrefix(name, "Increment"):
			metricName = name[len("Increment"):]
		case strings.HasPrefix(name, "Record"):
			metricName = name[len("Record"):]
		default:
			continue
		}
		// Re-extract from the marker comment (more reliable: matches the manifest name).
		if m := extractMarker(fn.Doc); m != "" {
			metricName = m
		}
		sig := metricSig{}
		if fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				typ := exprString(field.Type)
				if typ == "context.Context" {
					continue
				}
				for _, n := range field.Names {
					if n.Name == "value" {
						sig.value = typ
						continue
					}
					if typ == "string" {
						sig.labels = append(sig.labels, n.Name)
					}
				}
			}
		}
		out[metricName] = sig
	}
	return out
}

// walkNewIntermediate inspects the body of NewIntermediate(impl ToDo) and pulls
// out the description, subscriptions, configs, metrics, tickers, and inbound hooks.
func walkNewIntermediate(fn *ast.FuncDecl, x *extracted, defs map[string]def, apiPkg string, inOuts map[string]inOutPair, todoMethods map[string]todoMethod, metricSigs map[string]metricSig, configSigs map[string]string) {
	if fn.Body == nil {
		return
	}
	for _, stmt := range fn.Body.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		// Method calls on svc: svc.SetDescription, svc.Subscribe, svc.DefineConfig, etc.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		// Only consider receivers named `svc` to avoid unrelated calls.
		switch sel.Sel.Name {
		case "SetDescription":
			if s := stringArg(call, 0); s != "" {
				x.description = s
			}
		case "Subscribe":
			handleSubscribe(call, x, defs, apiPkg, inOuts, todoMethods)
		case "DefineConfig":
			handleDefineConfig(call, x, configSigs)
		case "DescribeCounter":
			handleDescribe(call, x, "counter", metricSigs)
		case "DescribeGauge":
			handleDescribe(call, x, "gauge", metricSigs)
		case "DescribeHistogram":
			handleDescribe(call, x, "histogram", metricSigs)
		case "StartTicker":
			handleStartTicker(call, x, todoMethods)
		}
		// Hook subscriptions look like `eventsourceapi.NewHook(svc).OnX(svc.OnX)` -
		// they are call chains. Detect them separately.
		handleHookCall(call, x, todoMethods, inOuts, apiPkg)
	}
}

// handleSubscribe parses one svc.Subscribe(...) call and appends to the
// appropriate endpoint slice.
func handleSubscribe(call *ast.CallExpr, x *extracted, defs map[string]def, apiPkg string, inOuts map[string]inOutPair, todoMethods map[string]todoMethod) {
	if len(call.Args) < 2 {
		return
	}
	name := unquoteOrEmpty(call.Args[0])
	if name == "" {
		return
	}
	ep := Endpoint{Name: name}
	kind := "" // function | web | task | workflow

	for _, arg := range call.Args[2:] {
		c, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		switch s.Sel.Name {
		case "At":
			if len(c.Args) >= 2 {
				ep.Method = resolveDef(c.Args[0], defs, apiPkg, "Method")
				ep.Route = resolveDef(c.Args[1], defs, apiPkg, "Route")
			}
		case "Description":
			ep.Description = stringArg(c, 0)
		case "RequiredClaims":
			ep.RequiredClaims = stringArg(c, 0)
		case "TimeBudget":
			if len(c.Args) >= 1 {
				ep.TimeBudget = renderDurationExpr(c.Args[0])
			}
		case "NoQueue":
			ep.LoadBalancing = "none"
		case "Queue":
			ep.LoadBalancing = stringArg(c, 0)
		case "Function":
			kind = "function"
		case "Web":
			kind = "web"
		case "Task":
			kind = "task"
		case "Workflow":
			kind = "workflow"
		}
	}

	switch kind {
	case "function":
		ep.Signature = signatureFromTodo(name, todoMethods)
		x.functions = append(x.functions, ep)
	case "web":
		// Web endpoints use the bare description+method+route shape (no signature).
		x.webs = append(x.webs, ep)
	case "task":
		ep.Signature = signatureFromTodo(name, todoMethods)
		x.tasks = append(x.tasks, ep)
	case "workflow":
		ep.Signature = signatureFromInOutWorkflow(name, inOuts)
		x.workflows = append(x.workflows, ep)
	}
}

// signatureFromTodo builds a signature from the ToDo interface method matching
// `name`. It strips ctx/flow/err returns. If no match is found, returns "".
func signatureFromTodo(name string, todoMethods map[string]todoMethod) string {
	m, ok := todoMethods[name]
	if !ok {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('(')
	for i, p := range m.params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Name)
		sb.WriteByte(' ')
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')
	if len(m.results) > 0 {
		sb.WriteString(" (")
		for i, r := range m.results {
			if i > 0 {
				sb.WriteString(", ")
			}
			if r.Name != "" {
				sb.WriteString(r.Name)
				sb.WriteByte(' ')
			}
			sb.WriteString(r.Type)
		}
		sb.WriteByte(')')
	}
	return sb.String()
}

// signatureFromInOutWorkflow builds a workflow signature using the In/Out struct
// fields (since the ToDo method returns *workflow.Graph rather than the inputs).
// Workflow signatures use Go field names rather than JSON tag names so that
// outputs sharing a name with an input keep their `Out` suffix - producing a
// valid Go function signature, consistent with how task signatures are rendered.
func signatureFromInOutWorkflow(name string, inOuts map[string]inOutPair) string {
	pair, ok := inOuts[name]
	if !ok {
		return name + "()"
	}
	return signatureFromInOutGoNames(name, pair.in, pair.out)
}

// resolveDef extracts a string value from an argument that's either a literal
// or a reference like `apiPkg.Foo.Method`. Returns "" if neither shape matches.
func resolveDef(arg ast.Expr, defs map[string]def, apiPkg string, field string) string {
	if bl, ok := arg.(*ast.BasicLit); ok {
		return unquote(bl.Value)
	}
	sel, ok := arg.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if sel.Sel.Name != field {
		return ""
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := inner.X.(*ast.Ident)
	if !ok || pkg.Name != apiPkg {
		return ""
	}
	d, ok := defs[inner.Sel.Name]
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

// handleDefineConfig parses one svc.DefineConfig(...) call.
func handleDefineConfig(call *ast.CallExpr, x *extracted, configSigs map[string]string) {
	if len(call.Args) < 1 {
		return
	}
	name := unquoteOrEmpty(call.Args[0])
	if name == "" {
		return
	}
	c := Config{Name: name}
	if sig, ok := configSigs[name]; ok {
		c.Signature = sig
	}
	for _, arg := range call.Args[1:] {
		ce, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		s, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		switch s.Sel.Name {
		case "Description":
			c.Description = stringArg(ce, 0)
		case "DefaultValue":
			c.Default = stringArg(ce, 0)
		case "Validation":
			c.Validation = stringArg(ce, 0)
		case "Secret":
			c.Secret = true
		}
	}
	x.configs = append(x.configs, c)
}

// handleDescribe parses one DescribeCounter/Gauge/Histogram call. The name is
// the OpenTelemetry name; the local manifest name is recovered by matching the
// recorder method (Increment*/Record*) whose snake_case form is a substring of
// the otel name (e.g. `FlowsStarted` → `flows_started` ⊂ `microbus_flows_started_total`).
func handleDescribe(call *ast.CallExpr, x *extracted, kind string, metricSigs map[string]metricSig) {
	if len(call.Args) < 2 {
		return
	}
	otelName := unquoteOrEmpty(call.Args[0])
	desc := unquoteOrEmpty(call.Args[1])
	if otelName == "" {
		return
	}
	m := Metric{
		OtelName:    otelName,
		Description: desc,
		Kind:        kind,
	}
	if kind == "histogram" && len(call.Args) >= 3 {
		m.Buckets = parseBucketLiteral(call.Args[2])
	}
	// Iterate in sorted order so a name collision (multiple snake_case forms
	// matching the same otelName) is resolved deterministically.
	names := make([]string, 0, len(metricSigs))
	for name := range metricSigs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, localName := range names {
		if strings.Contains(otelName, pascalToSnake(localName)) {
			m.Name = localName
			m.Signature = buildMetricSignature(localName, metricSigs[localName])
			break
		}
	}
	x.metrics = append(x.metrics, m)
}

// pascalToSnake renders PascalCase as snake_case, treating runs of consecutive
// capitals as acronyms (e.g. `LLMTokens` → `llm_tokens`, `SQLDataSourceName`
// → `sql_data_source_name`). A break is inserted before a capital only when
// the previous rune was lowercase, or when the capital is followed by a
// lowercase (i.e. it starts the next word after an acronym run).
func pascalToSnake(s string) string {
	rs := []rune(s)
	var sb strings.Builder
	for i, r := range rs {
		isUpper := r >= 'A' && r <= 'Z'
		if i > 0 && isUpper {
			prev := rs[i-1]
			prevLower := prev >= 'a' && prev <= 'z'
			next := rune(0)
			if i+1 < len(rs) {
				next = rs[i+1]
			}
			nextLower := next >= 'a' && next <= 'z'
			if prevLower || nextLower {
				sb.WriteByte('_')
			}
		}
		if isUpper {
			sb.WriteRune(r + ('a' - 'A'))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// buildMetricSignature constructs a manifest-style metric signature
// (e.g. `FlowsStarted(value int, workflowName string)`).
func buildMetricSignature(name string, sig metricSig) string {
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteString("(value ")
	if sig.value == "" {
		sb.WriteString("int")
	} else {
		sb.WriteString(sig.value)
	}
	for _, l := range sig.labels {
		sb.WriteString(", ")
		sb.WriteString(l)
		sb.WriteString(" string")
	}
	sb.WriteByte(')')
	return sb.String()
}

// parseBucketLiteral handles the bucket slice argument to DescribeHistogram.
// We render each element as plain text (preserving int/float style as-written).
func parseBucketLiteral(e ast.Expr) []string {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var out []string
	for _, elt := range cl.Elts {
		out = append(out, exprToValue(elt))
	}
	return out
}

// exprToValue renders a numeric literal or named constant as text.
func exprToValue(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.BasicLit:
		return t.Value
	case *ast.UnaryExpr:
		return t.Op.String() + exprToValue(t.X)
	case *ast.BinaryExpr:
		return exprToValue(t.X) + " " + t.Op.String() + " " + exprToValue(t.Y)
	}
	return ""
}

// handleStartTicker parses a svc.StartTicker(name, interval, handler) call.
func handleStartTicker(call *ast.CallExpr, x *extracted, todoMethods map[string]todoMethod) {
	if len(call.Args) < 3 {
		return
	}
	name := unquoteOrEmpty(call.Args[0])
	if name == "" {
		return
	}
	t := Ticker{Name: name}
	t.Interval = renderDurationExpr(call.Args[1])
	// Description and signature come from the ToDo method matching `name`.
	if m, ok := todoMethods[name]; ok {
		_ = m
		t.Signature = name + "()"
	}
	x.tickers = append(x.tickers, t)
}

// renderDurationExpr renders a `time.Duration` literal expression as a string
// using the same compact form the manifest uses (e.g. `5m`, `24h`, `100ms`).
func renderDurationExpr(e ast.Expr) string {
	bin, ok := e.(*ast.BinaryExpr)
	if ok {
		left, lok := bin.X.(*ast.BasicLit)
		right, rok := bin.Y.(*ast.SelectorExpr)
		if lok && rok {
			if id, ok := right.X.(*ast.Ident); ok && id.Name == "time" {
				val := left.Value
				suffix := durationSuffix(right.Sel.Name)
				return val + suffix
			}
		}
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" {
			return "1" + durationSuffix(sel.Sel.Name)
		}
	}
	return exprString(e)
}

func durationSuffix(name string) string {
	switch name {
	case "Nanosecond":
		return "ns"
	case "Microsecond":
		return "us"
	case "Millisecond":
		return "ms"
	case "Second":
		return "s"
	case "Minute":
		return "m"
	case "Hour":
		return "h"
	}
	return ""
}

// handleHookCall detects hook subscription chains and emits inbound event
// entries. Both the bare form and the ForHost-scoped form are recognized:
//
//	xxxapi.NewHook(svc).OnY(svc.OnY)
//	xxxapi.NewHook(svc).ForHost("...").OnY(svc.OnY)
//	xxxapi.NewHook(svc).WithOptions(...).OnY(svc.OnY)
//
// The receiver chain may contain any number of `ForHost` / `WithOptions` /
// other framework helpers between `NewHook` and the `OnY` call. The source
// package is derived from the api alias; resolution to the full import path
// happens in parseService.
func handleHookCall(call *ast.CallExpr, x *extracted, todoMethods map[string]todoMethod, inOuts map[string]inOutPair, ownAPIPkg string) {
	// The outer call is OnY(svc.OnY). Walk down its receiver chain looking
	// for a NewHook(svc) call.
	outerSel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg := findHookPkg(outerSel.X, ownAPIPkg)
	if pkg == "" {
		return
	}
	eventName := outerSel.Sel.Name
	if !strings.HasPrefix(eventName, "On") {
		// Defensive: the framework hook surface only exposes On* methods, but
		// guard against ForHost/WithOptions being the outermost call (which
		// would mean the chain doesn't end in a subscription).
		return
	}
	ev := InboundEvent{
		Name:      eventName,
		Signature: buildOutboundSignature(eventName, inOuts),
		Package:   pkg, // alias placeholder; resolved to package path in parseService
	}
	if sig := signatureFromTodo(eventName, todoMethods); sig != "" {
		// The local ToDo signature includes ctx; signatureFromTodo strips it.
		ev.Signature = sig
	}
	x.inboundEvents = append(x.inboundEvents, ev)
}

// findHookPkg walks down the receiver chain of a hook subscription and returns
// the api-package alias of the originating `NewHook(...)` call, or "" if the
// chain doesn't contain one (or is rooted at our own api package, in which
// case it's a self-hook, not an inbound event).
func findHookPkg(expr ast.Expr, ownAPIPkg string) string {
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
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name == ownAPIPkg {
				return ""
			}
			return pkg.Name
		}
		// Otherwise this link is a chain helper (ForHost, WithOptions, ...);
		// keep walking.
		expr = sel.X
	}
}

// collectObservableMetrics looks at doOnObserveMetrics' body for OnObserveX
// calls and marks the corresponding metrics observable.
func collectObservableMetrics(fn *ast.FuncDecl, x *extracted) {
	if fn.Body == nil {
		return
	}
	observable := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if strings.HasPrefix(sel.Sel.Name, "OnObserve") {
			observable[sel.Sel.Name[len("OnObserve"):]] = true
		}
		return true
	})
	for i := range x.metrics {
		if observable[x.metrics[i].Name] {
			x.metrics[i].Observable = true
		}
	}
}

// collectConfigCallbacks examines doOnConfigChanged for `if changed("X")` blocks
// and marks those configs as having callbacks.
func collectConfigCallbacks(fn *ast.FuncDecl, x *extracted) {
	if fn.Body == nil {
		return
	}
	cb := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "changed" || len(call.Args) != 1 {
			return true
		}
		bl, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		cb[unquote(bl.Value)] = true
		return true
	})
	for i := range x.configs {
		if cb[x.configs[i].Name] {
			x.configs[i].Callback = true
		}
	}
}

// stringArg returns the i-th argument of a call as a Go string literal, or "".
func stringArg(call *ast.CallExpr, i int) string {
	if i >= len(call.Args) {
		return ""
	}
	return unquoteOrEmpty(call.Args[i])
}

// unquoteOrEmpty unwraps a string literal expression. Empty if not a string.
func unquoteOrEmpty(e ast.Expr) string {
	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return ""
	}
	return unquote(bl.Value)
}
