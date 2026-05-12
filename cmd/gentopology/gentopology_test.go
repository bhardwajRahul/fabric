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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microbus-io/testarossa"
)

// repoPath returns an absolute path under the repository root, robust
// against the test runner's CWD.
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
				t.Fatalf("path does not exist: %s", abs)
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

// TestRun_RealBundle exercises the full pipeline against the framework's
// own main.go bundle, asserting key properties of the resulting mermaid.
// Doesn't pin byte equality - operators tweaking service names or the
// bundle composition would otherwise have to update a brittle golden.
func TestRun_RealBundle(t *testing.T) {
	// No parallel - scans the full main bundle.
	assert := testarossa.For(t)
	bundle := repoPath(t, "main/main.go")
	out := filepath.Join(t.TempDir(), "topology.mmd")
	if err := run(config{bundle: bundle, out: out}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(data)

	// Structural shape: header, classDefs, class statements at the end.
	wantSubstr := []string{
		"graph LR",
		"classDef core",
		"classDef svc",
		"classDef danger fill:#f15922",
		"classDef ext",
		"class ",
	}
	for _, want := range wantSubstr {
		assert.Contains(got, want, "output missing %q", want)
	}

	// Spot-check known relationships in the bundle. bearer.token.core
	// and access.token.core expose :666/mint, so edges into them carry
	// the danger label and they're classified as danger nodes.
	checks := map[string]string{
		"foreman has SQL":             "foreman.core --- foreman.core.db[(SQL)]",
		"yellowpages has SQL":         "yellowpages.example.db[(SQL)]",
		"events flow source → sink":   " -..-> eventsink.example",
		"hello calls calculator":      "hello.example[Hello<br>hello.example] ---> calculator.example",
		"login calls bearer.token":    "login.example[Login<br>login.example] --->|danger| bearer.token.core",
		"creditflow calls foreman":    "creditflow.example[CreditFlow<br>creditflow.example] ---> foreman.core",
		"some service has cloud":      ".cloud@{shape: cloud, label:",
		"bearer.token.core is danger": "bearer.token.core danger",
		"embedder has Py":             "embedder.example.py[[Py]]",
	}
	for desc, substr := range checks {
		assert.Contains(got, substr, "%s: missing %q", desc, substr)
	}
}

// TestRun_CheckMode_Clean asserts --check returns nil when the existing
// file matches what the scan would produce.
func TestRun_CheckMode_Clean(t *testing.T) {
	// No parallel - scans the full main bundle twice.
	assert := testarossa.For(t)
	bundle := repoPath(t, "main/main.go")
	out := filepath.Join(t.TempDir(), "topology.mmd")
	// First run: write the file.
	if err := run(config{bundle: bundle, out: out}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Second run with --check: should be a no-op.
	assert.NoError(run(config{bundle: bundle, out: out, check: true}), "--check on fresh output flagged a diff")
}

// TestDetectCloud_IgnoresCommentURLs verifies that URLs appearing only
// in comments (the Apache license URL in the copyright header is the
// canonical case) do not get reported as outbound dependencies. This
// regression is what made httpegress and examples/browser show
// "www.apache.org" as their cloud target despite never calling Apache.
func TestDetectCloud_IgnoresCommentURLs(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	src := `/*
Copyright (c) 2026 Example.

Licensed under the Apache License, Version 2.0 (the "License");
http://www.apache.org/licenses/LICENSE-2.0
*/

// See also https://docs.example.org/v2 for usage notes.
package svc

func A() {}
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectCloud(dir, nil)
	assert.False(got == "www.apache.org" || got == "docs.example.org", "comment URL leaked into cloud detection: %q", got)
	assert.Equal("various", got, "got cloud %q, want %q (no statically-extractable URL outside comments)", got, "various")
}

// TestDetectCloud_DetectsRealURLDespiteCommentURL verifies that a URL
// in a string literal is still picked up when the same file also
// contains an Apache license URL in its copyright header. This is the
// pairing case for TestDetectCloud_IgnoresCommentURLs - the fix must
// strip comments without disturbing real URL literals.
func TestDetectCloud_DetectsRealURLDespiteCommentURL(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	src := `/*
Copyright (c) 2026 Example.
http://www.apache.org/licenses/LICENSE-2.0
*/

package svc

const Endpoint = "https://api.example.com/v1/widgets"
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectCloud(dir, nil)
	assert.Equal("api.example.com", got, "cloud detection")
}

// TestRun_CheckMode_Dirty asserts --check returns errCheckDiff when the
// existing file is out of date.
func TestRun_CheckMode_Dirty(t *testing.T) {
	// No parallel - scans the full main bundle.
	bundle := repoPath(t, "main/main.go")
	out := filepath.Join(t.TempDir(), "topology.mmd")
	if err := os.WriteFile(out, []byte("stale content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(config{bundle: bundle, out: out, check: true})
	if err == nil {
		t.Fatal("expected --check to flag drift, got nil")
	}
}
