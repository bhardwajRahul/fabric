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
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

// updateGoldens, when set, rewrites the committed manifest.yaml goldens
// in cmd/genmanifest/testdata/ from the live extractor output instead of
// comparing against them. Use after an intentional change to the
// extractor or emitter:
//
//	go test ./cmd/genmanifest/ -update
//
// Then review the diff in `git status` and commit the regenerated files.
var updateGoldens = flag.Bool("update", false, "rewrite committed manifest.yaml goldens from extractor output")

// TestExtract_Foreman runs the extractor against the real foreman package and
// asserts the salient fields it pulls out. The test is intentionally narrow:
// it exercises the extraction logic against a service that has the full
// cross-section of features (description, functions, web, outbound event,
// configs, metrics with observable, downstream).
func TestExtract_Foreman(t *testing.T) {
	assert := testarossa.For(t)
	dir := repoPath(t, "coreservices/foreman")
	x, err := extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assert.Equal("foreman.core", x.hostname, "hostname")
	assert.Contains(x.description, "agentic workflow execution", "description missing keyword")
	// Must have extracted multiple functions and one web.
	assert.True(len(x.functions) >= 10, "expected >=10 functions, got %d", len(x.functions))
	assert.True(len(x.webs) == 1 && x.webs[0].Name == "HistoryMermaid", "expected one web (HistoryMermaid), got %+v", x.webs)
	// Outbound event detected.
	assert.True(len(x.outboundEvents) == 1 && x.outboundEvents[0].Name == "OnFlowStopped", "expected OnFlowStopped outbound event, got %+v", x.outboundEvents)
	// Configs include the secret one and the callback one.
	var foundSecret, foundCallback bool
	for _, c := range x.configs {
		if c.Name == "SQLDataSourceName" && c.Secret {
			foundSecret = true
		}
		if c.Name == "NumShards" && c.Callback {
			foundCallback = true
		}
	}
	assert.True(foundSecret, "expected SQLDataSourceName secret=true")
	assert.True(foundCallback, "expected NumShards callback=true")
	// StepsQueueDepth metric is observable.
	var foundObs bool
	for _, m := range x.metrics {
		if m.Name == "StepsQueueDepth" && m.Observable {
			foundObs = true
		}
	}
	assert.True(foundObs, "expected StepsQueueDepth observable=true")
}

// TestExtract_CreditFlow exercises the task and workflow extraction paths,
// plus the one-downstream case (foreman).
func TestExtract_CreditFlow(t *testing.T) {
	assert := testarossa.For(t)
	dir := repoPath(t, "examples/creditflow")
	x, err := extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assert.Equal("creditflow.example", x.hostname, "hostname")
	assert.True(len(x.tasks) >= 5, "expected >=5 tasks, got %d", len(x.tasks))
	assert.Equal(2, len(x.workflows), "workflows count")
	// SSN field rendered without the camelCase glitch.
	for _, w := range x.workflows {
		if w.Name == "IdentityVerification" {
			assert.Contains(w.Signature, "ssn string", "expected lowercase ssn in workflow signature")
		}
	}
	// sub.TimeBudget on a task is extracted as a compact duration.
	var sawPhone bool
	for _, ts := range x.tasks {
		if ts.Name == "VerifyPhoneNumber" {
			sawPhone = true
			assert.Equal("1s", ts.TimeBudget, "VerifyPhoneNumber time budget")
		}
	}
	assert.True(sawPhone, "expected a VerifyPhoneNumber task")
}

// TestEmit_RoundTrip verifies that running the tool twice produces the same
// output (modulo modifiedAt). The test writes once, then snapshots, mutates
// modifiedAt to a known value, and writes again with that known value to
// confirm idempotence.
func TestEmit_RoundTrip(t *testing.T) {
	assert := testarossa.For(t)
	dir := mkFixtureCopy(t, repoPath(t, "examples/creditflow"))
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := run(dir, false, fixed); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := run(dir, false, fixed); err != nil {
		t.Fatalf("run #2: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(string(first), string(second), "non-idempotent output:\n--- first ---\n%s\n--- second ---\n%s", first, second)
}

// TestCheckMode_NoDiff confirms --check returns nil after a fresh write.
func TestCheckMode_NoDiff(t *testing.T) {
	dir := mkFixtureCopy(t, repoPath(t, "examples/creditflow"))
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := run(dir, false, fixed); err != nil {
		t.Fatalf("run write: %v", err)
	}
	if err := run(dir, true, fixed); err != nil {
		t.Fatalf("--check after fresh write: %v, want nil", err)
	}
}

// TestModifiedAt_StableWhenContentUnchanged: regenerating a manifest whose
// content hasn't changed must not bump modifiedAt, even if the wall-clock now
// has advanced. This is what makes --check usable in practice - without it,
// the timestamp churn alone produces spurious diffs.
func TestModifiedAt_StableWhenContentUnchanged(t *testing.T) {
	assert := testarossa.For(t)
	// Copy creditflow's source files (intermediate.go, service.go, *api/) but
	// *delete* the manifest so the first run writes fresh with t0.
	dir := mkFixtureCopy(t, repoPath(t, "examples/creditflow"))
	if err := os.Remove(filepath.Join(dir, "manifest.yaml")); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // months later
	if err := run(dir, false, t0); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(first, []byte(`modifiedAt: "2026-01-01T00:00:00Z"`)) {
		t.Fatalf("expected initial timestamp in first write:\n%s", first)
	}
	if err := run(dir, false, t1); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.True(bytes.Equal(first, second), "regen with unchanged content modified the file:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	// And --check with the later wall-clock must report no diff.
	assert.NoError(run(dir, true, t1), "--check with advanced clock but unchanged content")
}

// TestCheckMode_Diff confirms --check returns errCheckDiff when the manifest
// is mutated.
func TestCheckMode_Diff(t *testing.T) {
	assert := testarossa.For(t)
	dir := mkFixtureCopy(t, repoPath(t, "examples/creditflow"))
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := run(dir, false, fixed); err != nil {
		t.Fatalf("run write: %v", err)
	}
	mp := filepath.Join(dir, "manifest.yaml")
	body, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	mutated := append(body, []byte("# trailing\n")...)
	if err := os.WriteFile(mp, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	err = run(dir, true, fixed)
	assert.True(errors.Is(err, errCheckDiff), "--check after mutation: %v, want errCheckDiff", err)
}

// TestPreservesGeneralName confirms operator-curated fields under general are
// kept across regeneration.
func TestPreservesGeneralName(t *testing.T) {
	assert := testarossa.For(t)
	dir := mkFixtureCopy(t, repoPath(t, "examples/creditflow"))
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := run(dir, false, fixed); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"name: CreditFlow",
		"frameworkVersion: 1.28.0",
	} {
		assert.Contains(string(body), want, "regenerated manifest missing %q\n%s", want, body)
	}
}

// TestLowerFirst tests the acronym-aware lowerFirst helper.
func TestLowerFirst(t *testing.T) {
	assert := testarossa.For(t)
	cases := map[string]string{
		"":         "",
		"ID":       "id",
		"SSN":      "ssn",
		"URL":      "url",
		"URLPath":  "urlPath",
		"FlowKey":  "flowKey",
		"flowKey":  "flowKey",
		"X":        "x",
		"OAuth":    "oAuth",
		"HTTPHost": "httpHost",
	}
	for in, want := range cases {
		assert.Equal(want, lowerFirst(in), "lowerFirst(%q)", in)
	}
}

// TestExtract_InboundEvent_BareHook covers the simple `xxxapi.NewHook(svc).OnY(svc.OnY)`
// shape (no chain helpers between NewHook and OnY).
func TestExtract_InboundEvent_BareHook(t *testing.T) {
	assert := testarossa.For(t)
	dir := repoPath(t, "examples/eventsink")
	x, err := extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	want := map[string]bool{"OnAllowRegister": false, "OnRegistered": false}
	for _, ev := range x.inboundEvents {
		if _, ok := want[ev.Name]; ok {
			want[ev.Name] = true
		}
	}
	for name, found := range want {
		assert.True(found, "inbound event %q not extracted; got %+v", name, x.inboundEvents)
	}
}

// TestExtract_InboundEvent_ForHostChain covers the documented foreman-subscriber
// shape `xxxapi.NewHook(svc).ForHost(...).OnY(svc.OnY)` - the receiver of OnY
// is a ForHost call, not NewHook directly. Walking the chain is the fix for
// the bug where ForHost-scoped hooks were silently dropped.
func TestExtract_InboundEvent_ForHostChain(t *testing.T) {
	dir := mkFixtureDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "sinkapi"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal *api/endpoints.go so findAPIDir succeeds.
	endpoints := []byte(`package sinkapi

const Hostname = "sink.test"
type Def struct {
	Method string
	Route  string
}
`)
	if err := os.WriteFile(filepath.Join(dir, "sinkapi", "endpoints.go"), endpoints, 0o644); err != nil {
		t.Fatal(err)
	}
	// Minimal client.go (no MulticastTrigger godocs).
	client := []byte(`package sinkapi

type MulticastTrigger struct{}
type Hook struct{}
type Client struct{}
type MulticastClient struct{}
`)
	if err := os.WriteFile(filepath.Join(dir, "sinkapi", "client.go"), client, 0o644); err != nil {
		t.Fatal(err)
	}
	intermediate := []byte(`package sink

import (
	"context"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

type Intermediate struct{}

type ToDo interface {
	OnFlowStopped(ctx context.Context, flowKey string, status string, snapshot map[string]any) (err error)
}

func NewIntermediate(svc *Intermediate, impl ToDo) *Intermediate {
	foremanapi.NewHook(svc).ForHost("sink.test").OnFlowStopped(impl.OnFlowStopped)
	return svc
}
`)
	if err := os.WriteFile(filepath.Join(dir, "intermediate.go"), intermediate, 0o644); err != nil {
		t.Fatal(err)
	}
	service := []byte(`package sink

import "github.com/microbus-io/fabric/coreservices/foreman/foremanapi"

var _ = foremanapi.Hostname

type Service struct{}
`)
	if err := os.WriteFile(filepath.Join(dir, "service.go"), service, 0o644); err != nil {
		t.Fatal(err)
	}

	x, err := extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	assert := testarossa.For(t)
	var found bool
	for _, ev := range x.inboundEvents {
		if ev.Name == "OnFlowStopped" {
			found = true
			assert.Contains(ev.Package, "foreman", "expected source resolved to foreman package, got %q", ev.Package)
		}
	}
	assert.True(found, "ForHost-chained inbound hook was dropped:\n%+v", x.inboundEvents)
}

// TestExtract_InboundEventDependency confirms that an inbound event hook
// surfaces its source *api package on the manifest. The hostname/route/
// method fields are intentionally not asserted - they're not stored in
// the manifest under JIT.
func TestExtract_InboundEventDependency(t *testing.T) {
	assert := testarossa.For(t)
	x, err := extract(repoPath(t, "examples/eventsink"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var onAllow *InboundEvent
	for i := range x.inboundEvents {
		if x.inboundEvents[i].Name == "OnAllowRegister" {
			onAllow = &x.inboundEvents[i]
		}
	}
	if onAllow == nil {
		t.Fatalf("OnAllowRegister not extracted: %+v", x.inboundEvents)
	}
	assert.True(onAllow.Package != "", "package is empty; expected resolved source package")
}

// TestKitchenFixture_ManifestGolden re-extracts the kitchen fixture's manifest
// and asserts byte equality (modulo modifiedAt) against the committed golden.
// The kitchen fixture intentionally exercises every detected call pattern, so
// this test fails any time the static analyzer's behavior shifts on:
// chain/var/parameter binding, ForHost literal vs varExpr, helper expansion,
// raw svc.Request inline/slice/append, self-call with and without ForHost,
// inbound-event hook resolution.
func TestKitchenFixture_ManifestGolden(t *testing.T) {
	assertManifestGolden(t, "kitchen")
}

// TestWeirdFixture_ManifestGolden re-extracts the weird fixture (the kitchen's
// downstream) and pins its manifest. This catches regressions in Def-side
// detection: outbound event recognition, route-shape preservation, package
// path resolution.
func TestWeirdFixture_ManifestGolden(t *testing.T) {
	assertManifestGolden(t, "weird")
}

// assertManifestGolden runs genmanifest against testdata/<name> and compares
// against the committed manifest.yaml. The committed file IS the golden.
//
// Run with `go test -update` to overwrite the golden with the freshly
// extracted output instead of comparing.
func assertManifestGolden(t *testing.T, name string) {
	t.Helper()
	assert := testarossa.For(t)
	dir := repoPath(t, filepath.Join("cmd/genmanifest/testdata", name))
	goldenPath := filepath.Join(dir, "manifest.yaml")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	// Run with the existing modifiedAt baked in so a content-equal regen
	// produces identical bytes (the same trick TestModifiedAt uses).
	fixed := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	tmp := mkFixtureCopy(t, dir)
	if err := run(tmp, false, fixed); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if *updateGoldens {
		if bytes.Equal(want, got) {
			return
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		t.Logf("%s: golden updated", name)
		return
	}
	assert.True(bytes.Equal(want, got), "%s manifest drift (run `go test -update` to refresh):\n--- want (committed golden) ---\n%s\n--- got (re-extracted) ---\n%s",
		name, want, got)
}

// TestEmit_NoHeaderWhenAbsent: a fresh manifest (no existing file) gets no
// synthesized license header, but it always starts with the generated marker
// so the file is never mistaken for hand-authored. Operators add a license
// header once and the tool preserves it above the marker.
func TestEmit_NoHeaderWhenAbsent(t *testing.T) {
	assert := testarossa.For(t)
	m := &Manifest{General: General{Hostname: "x.svc"}}
	out := string(emit(m, nil))
	assert.True(strings.HasPrefix(out, genmanifestMarker+"\n\ngeneral:"),
		"fresh manifest should start with the generated marker then general:, got:\n%s", out)
}

// TestEmit_PreservesExistingHeader: an existing file's leading comment block
// is preserved verbatim across regeneration.
func TestEmit_PreservesExistingHeader(t *testing.T) {
	assert := testarossa.For(t)
	existing := []byte(`# Custom header line 1
# Custom header line 2

general:
  hostname: x.svc
`)
	m := &Manifest{General: General{Hostname: "x.svc"}}
	out := string(emit(m, existing))
	assert.True(strings.HasPrefix(out, "# Custom header line 1\n# Custom header line 2\n"), "existing header lost on regen, got:\n%s", out)
}

// TestEmit_BlockScalar verifies that multi-line description values are emitted
// using YAML's literal block scalar form.
func TestEmit_BlockScalar(t *testing.T) {
	assert := testarossa.For(t)
	m := &Manifest{
		General: General{
			Hostname:    "x.svc",
			Description: "Line 1.\nLine 2.",
		},
	}
	out := string(emit(m, nil))
	assert.Contains(out, "description: |-", "expected block scalar marker, got:\n%s", out)
	assert.True(strings.Contains(out, "Line 1.") && strings.Contains(out, "Line 2."), "expected both lines preserved, got:\n%s", out)
}

// TestExtract_MissingAPIDir surfaces a clear error when a service has no
// *api/ subdirectory.
func TestExtract_MissingAPIDir(t *testing.T) {
	assert := testarossa.For(t)
	dir := mkFixtureDir(t)
	_, err := extract(dir)
	assert.Error(err, "expected error when *api/ is absent")
}

// TestUnquote_DecodesEscapes confirms that escape sequences inside double-quoted
// Go string literals are decoded into their runtime values, while raw strings
// pass through verbatim.
func TestUnquote_DecodesEscapes(t *testing.T) {
	assert := testarossa.For(t)
	cases := map[string]string{
		`"plain"`:   "plain",
		`"a\nb"`:    "a\nb",
		`"\t"`:      "\t",
		`"a\"b"`:    `a"b`,
		"`raw\\nb`": `raw\nb`, // backticks: literal backslash+n
	}
	for in, want := range cases {
		assert.Equal(want, unquote(in), "unquote(%q)", in)
	}
}

// TestNeedsQuoting covers the YAML emitter's quoting decision matrix.
func TestNeedsQuoting(t *testing.T) {
	tests := map[string]bool{
		"":              true,
		"hello":         false,
		"true":          true,
		"FALSE":         true,
		" leading":      true,
		"trailing ":     true,
		"has: colon":    true,
		"with #":        true,
		"plain word":    false,
		"int [1,]":      false,
		"dur (0s,24h]":  false,
		"//other.host":  false,
		":417/route":    false,
		"a b c":         false,
		"@reserved":     true,
		"1.28.0":        false,
		"trailing-col:": true,
	}
	assert := testarossa.For(t)
	for in, want := range tests {
		assert.Equal(want, needsQuoting(in), "needsQuoting(%q)", in)
	}
}

// repoPath returns the absolute path of a directory inside the framework
// module. The module root is found by walking up from this source file
// (via runtime.Caller) to the nearest go.mod - independent of test CWD,
// so it works under standard `go test`, sandboxed runners (bazel, nix
// checkPhase), and any depth of package nesting.
func repoPath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			abs := filepath.Join(dir, rel)
			if _, err := os.Stat(abs); err != nil {
				t.Fatalf("repo path %s does not exist: %v", abs, err)
			}
			return abs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", file)
		}
		dir = parent
	}
}

// mkFixtureDir creates a temp dir under testdata/ so `go list` resolves
// against the framework's go.mod.
func mkFixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("testdata", "fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// mkFixtureCopy clones a microservice directory into a temp dir under
// testdata/ (so `go list` resolves through the framework's go.mod). Only the
// files genmanifest reads are copied.
func mkFixtureCopy(t *testing.T, srcDir string) string {
	t.Helper()
	dst := mkFixtureDir(t)

	// Copy intermediate.go, service.go, manifest.yaml at the top.
	for _, name := range []string{"intermediate.go", "service.go", "manifest.yaml"} {
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			if os.IsNotExist(err) && name == "manifest.yaml" {
				continue
			}
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Copy *api directory.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), "api") || e.Name() == "api" {
			continue
		}
		srcAPI := filepath.Join(srcDir, e.Name())
		dstAPI := filepath.Join(dst, e.Name())
		if err := os.MkdirAll(dstAPI, 0o755); err != nil {
			t.Fatal(err)
		}
		apiEntries, err := os.ReadDir(srcAPI)
		if err != nil {
			t.Fatal(err)
		}
		for _, ae := range apiEntries {
			if ae.IsDir() || !strings.HasSuffix(ae.Name(), ".go") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(srcAPI, ae.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dstAPI, ae.Name()), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dst
}
