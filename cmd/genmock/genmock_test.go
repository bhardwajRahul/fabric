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
	"os"
	"path/filepath"
	"testing"
)

// TestRoundtrip_RealServices regenerates each microservice's mock.go and
// re-runs the generator. The second run must produce byte-identical output -
// any drift means the generator is non-idempotent.
func TestRoundtrip_RealServices(t *testing.T) {
	roots := []string{
		"../../coreservices",
		"../../examples",
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(dir, "intermediate.go")); err != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, "mock.go")); err != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, "mock_test.go")); err != nil {
				continue
			}
			t.Run(e.Name(), func(t *testing.T) {
				assertIdempotent(t, dir)
			})
		}
	}
}

func assertIdempotent(t *testing.T, dir string) {
	t.Helper()
	g, err := generate(dir)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	cases := []struct {
		name     string
		filename string
		emit     func(*generated, []byte) ([]byte, error)
	}{
		{"mock.go", "mock.go", emit},
		{"mock_test.go", "mock_test.go", emitMockTest},
	}
	for _, c := range cases {
		first, err := c.emit(g, nil)
		if err != nil {
			t.Fatalf("emit %s first: %v", c.name, err)
		}
		second, err := c.emit(g, first)
		if err != nil {
			t.Fatalf("emit %s second: %v", c.name, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("non-idempotent %s output for %s", c.name, dir)
		}
		committed, err := os.ReadFile(filepath.Join(dir, c.filename))
		if err != nil {
			t.Fatalf("read %s: %v", c.filename, err)
		}
		regenFromCommitted, err := c.emit(g, committed)
		if err != nil {
			t.Fatalf("emit %s from committed: %v", c.name, err)
		}
		if !bytes.Equal(regenFromCommitted, committed) {
			t.Logf("note: %s/%s would change on regeneration", dir, c.filename)
		}
	}
}

func TestQualifyTypes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"string", "string"},
		{"int", "int"},
		{"bool", "bool"},
		{"any", "any"},
		{"error", "error"},
		{"Foo", "pkgapi.Foo"},
		{"[]Foo", "[]pkgapi.Foo"},
		{"*Foo", "*pkgapi.Foo"},
		{"map[string]Foo", "map[string]pkgapi.Foo"},
		{"map[Foo]Bar", "map[pkgapi.Foo]pkgapi.Bar"},
		{"time.Time", "time.Time"},
		{"pkgapi.Bar", "pkgapi.Bar"},
		{"[]other.Thing", "[]other.Thing"},
	}
	for _, c := range cases {
		got := qualifyTypes(c.in, "pkgapi")
		if got != c.want {
			t.Errorf("qualifyTypes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKebabCase(t *testing.T) {
	cases := map[string]string{
		"ChatLoop":             "chat-loop",
		"IdentityVerification": "identity-verification",
		"X":                    "x",
		"ABC":                  "a-b-c",
		"OnFoo":                "on-foo",
	}
	for in, want := range cases {
		if got := kebabCase(in); got != want {
			t.Errorf("kebabCase(%q) = %q, want %q", in, got, want)
		}
	}
}
