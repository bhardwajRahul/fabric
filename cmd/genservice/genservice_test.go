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
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/microbus-io/testarossa"
)

// updateGoldens rewrites the committed generated files from genservice output. Run after an intentional
// generator change:
//
//	go test ./cmd/genservice/ -update
var updateGoldens = flag.Bool("update", false, "rewrite committed generated files from genservice output")

// goldenNow is the timestamp passed to emitAll in tests. It never appears in an up-to-date golden:
// emitManifest preserves the committed modifiedAt whenever content is unchanged, so this value only
// surfaces if a manifest actually drifts.
const goldenNow = "2099-01-01T00:00:00Z"

// goldenFixtures are the api-only and full-service fixtures whose generated files are committed and
// serve as goldens. srcapi precedes svcapi so the latter's inbound-event source resolves.
var goldenFixtures = []string{
	"testdata/pressuretest/srcapi",
	"testdata/pressuretest/svcapi",
	"testdata/svc",
}

// TestGoldens regenerates each fixture and asserts byte equality against its committed output. A
// generator change that alters output fails here with a localized diff; refresh with -update.
func TestGoldens(t *testing.T) {
	for _, dir := range goldenFixtures {
		t.Run(dir, func(t *testing.T) {
			assert := testarossa.For(t)
			outs, err := emitAll(dir, goldenNow)
			if err != nil {
				t.Fatalf("emitAll: %v", err)
			}
			for _, o := range outs {
				want, readErr := os.ReadFile(o.path)
				if *updateGoldens {
					if readErr != nil || !bytes.Equal(want, o.content) {
						if err := os.WriteFile(o.path, o.content, 0o644); err != nil {
							t.Fatalf("update %s: %v", o.path, err)
						}
						t.Logf("updated %s", o.path)
					}
					continue
				}
				if readErr != nil {
					t.Fatalf("read golden %s: %v", o.path, readErr)
				}
				assert.True(bytes.Equal(want, o.content),
					"%s drift (run `go test ./cmd/genservice/ -update`):\n--- want (committed) ---\n%s\n--- got (regenerated) ---\n%s",
					o.path, want, o.content)
			}
		})
	}
}

// TestManifestModifiedAtStable asserts the manifest's modifiedAt is preserved when content is
// unchanged, regardless of the supplied timestamp - the property that makes regeneration idempotent
// and -check stable.
func TestManifestModifiedAtStable(t *testing.T) {
	assert := testarossa.For(t)
	outs, err := emitAll("testdata/svc", "2099-12-31T23:59:59Z")
	if err != nil {
		t.Fatalf("emitAll: %v", err)
	}
	for _, o := range outs {
		if filepath.Base(o.path) != "manifest.yaml" {
			continue
		}
		committed, err := os.ReadFile(o.path)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(bytes.Equal(committed, o.content), "manifest modifiedAt is not stable across regeneration")
	}
}
