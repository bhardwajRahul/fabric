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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

// updateGoldens, when set, rewrites the committed nats.acl goldens in
// cmd/genmanifest/testdata/{kitchen,weird}/ from the live scan +
// buildACLRules output instead of comparing against them. Use after an
// intentional change to the AST scanner or rule constructor:
//
//	go test ./cmd/gencreds/ -update -run TestScan_FixtureGoldens
//
// Then review the diff in `git status` and commit the regenerated files.
var updateGoldens = flag.Bool("update", false, "rewrite committed nats.acl goldens from scan output")

// TestScan_FixtureGoldens runs the source-driven scan + buildACLRules
// against the kitchen and weird fixtures and asserts the resulting
// subjects match the committed `nats.acl` goldens. The goldens live
// under cmd/genmanifest/testdata/{kitchen,weird}/ alongside the
// fixtures themselves.
//
// Failure here means either:
//   - A detection regression in scan.go / scan_calls.go, or
//   - A subject-construction regression in aclbuild.go / aclencode.go.
//
// If the change is intentional, run with `-update` to refresh the
// goldens, then review and commit the diff.
func TestScan_FixtureGoldens(t *testing.T) {
	if !*updateGoldens {
		t.Parallel()
	}
	for _, name := range []string{"kitchen", "weird"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if !*updateGoldens {
				t.Parallel()
			}
			assert := testarossa.For(t)
			dir := repoPath(t, "cmd/genmanifest/testdata/"+name)
			in, err := scanService(dir, dir)
			if err != nil {
				t.Fatalf("scanService: %v", err)
			}
			rules, _, err := buildACLRules(in)
			if err != nil {
				t.Fatalf("buildACLRules: %v", err)
			}
			rules = subsumptionDedup(rules)
			gotPub, gotSub := sortedSubjects(rules)
			goldenPath := filepath.Join(dir, "nats.acl")
			if *updateGoldens {
				if err := writeACLGolden(goldenPath, name+".fixture", gotPub, gotSub); err != nil {
					t.Fatalf("update golden: %v", err)
				}
				t.Logf("%s: golden updated", name)
				return
			}
			wantPub, wantSub := readACLSubjects(t, goldenPath)
			assert.True(equalStrings(gotPub, wantPub), "PUB mismatch (run `go test -update` to refresh)\nextra in source: %v\nmissing from source: %v",
				diffSets(gotPub, wantPub), diffSets(wantPub, gotPub))
			assert.True(equalStrings(gotSub, wantSub), "SUB mismatch (run `go test -update` to refresh)\nextra in source: %v\nmissing from source: %v",
				diffSets(gotSub, wantSub), diffSets(wantSub, gotSub))
		})
	}
}

// writeACLGolden writes a nats.acl in the canonical golden shape: a
// header comment block, blank line, sorted PUB rules, blank line, sorted
// SUB rules. Subjects are passed in already sorted.
func writeACLGolden(path, hostname string, pub, sub []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# nats.acl for %s\n", hostname)
	b.WriteString("# Test golden for cmd/gencreds. Pinned by code review; regenerate via\n")
	b.WriteString("# `go test ./cmd/gencreds/ -update -run TestScan_FixtureGoldens`.\n")
	b.WriteString("\n")
	for _, s := range pub {
		fmt.Fprintf(&b, "PUB %s\n", s)
	}
	if len(pub) > 0 && len(sub) > 0 {
		b.WriteString("\n")
	}
	for _, s := range sub {
		fmt.Fprintf(&b, "SUB %s\n", s)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// TestScan_AllRealServices is a smoke test: every service the framework
// ships should at minimum be scannable and produce a valid rule set
// (non-empty, within budget). Stronger validation lives in the
// end-to-end test (real broker) and the fixture-golden test (above).
func TestScan_AllRealServices(t *testing.T) {
	t.Parallel()
	for _, dir := range allServiceDirs(t) {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			assert := testarossa.For(t)
			in, err := scanService(dir, dir)
			if err != nil {
				t.Fatalf("scanService: %v", err)
			}
			rules, _, err := buildACLRules(in)
			if err != nil {
				t.Fatalf("buildACLRules: %v", err)
			}
			rules = subsumptionDedup(rules)
			if len(rules) == 0 {
				t.Fatalf("empty rule set for %s", dir)
			}
			size := aclBudgetSize(rules)
			assert.True(size <= aclBytesBudget, "rule set %d bytes exceeds %d-byte budget (top: %s)",
				size, aclBytesBudget, topPatterns(rules, 3))
		})
	}
}

// allServiceDirs returns absolute paths to every service directory in
// the repo's coreservices/ and examples/ trees. Skips testdata.
func allServiceDirs(t *testing.T) []string {
	t.Helper()
	var dirs []string
	for _, sub := range []string{"coreservices", "examples"} {
		root := repoPath(t, sub)
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", sub, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			d := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(d, "manifest.yaml")); err == nil {
				dirs = append(dirs, d)
			}
		}
	}
	if len(dirs) == 0 {
		t.Fatal("no service dirs with manifest.yaml found")
	}
	return dirs
}

func sortedSubjects(rules []aclRule) (pub, sub []string) {
	for _, r := range rules {
		switch r.Verb {
		case "PUB":
			pub = append(pub, r.Subject)
		case "SUB":
			sub = append(sub, r.Subject)
		}
	}
	sort.Strings(pub)
	sort.Strings(sub)
	return pub, sub
}

// readACLSubjects parses a committed nats.acl golden and returns the PUB
// and SUB subjects in lex order.
func readACLSubjects(t *testing.T, path string) (pub, sub []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		verb, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch verb {
		case "PUB":
			pub = append(pub, strings.TrimSpace(rest))
		case "SUB":
			sub = append(sub, strings.TrimSpace(rest))
		}
	}
	sort.Strings(pub)
	sort.Strings(sub)
	return pub, sub
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffSets returns elements in a not present in b.
func diffSets(a, b []string) []string {
	bs := map[string]bool{}
	for _, s := range b {
		bs[s] = true
	}
	var out []string
	for _, s := range a {
		if !bs[s] {
			out = append(out, s)
		}
	}
	return out
}
