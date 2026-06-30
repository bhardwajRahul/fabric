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
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestGeninit_FreshScaffold(t *testing.T) {
	root := t.TempDir()
	g := &generator{root: root}
	testarossa.NoError(t, g.run())

	for _, rel := range []string{
		"main/main.go", "main/env.yaml", "CLAUDE.md", "config.yaml",
		"config.local.yaml", "env.yaml", "env.local.yaml", ".gitignore",
		".vscode/launch.json", ".claude/settings.json",
	} {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		testarossa.NoError(t, err, "expected %s", rel)
	}

	main := readFile(t, root, "main/main.go")
	testarossa.Contains(t, main, "foreman.NewService()")

	gitignore := readFile(t, root, ".gitignore")
	testarossa.Contains(t, gitignore, "*.local.*")
}

func TestGeninit_KeyIsValidEd25519(t *testing.T) {
	root := t.TempDir()
	g := &generator{root: root}
	testarossa.NoError(t, g.run())

	der := extractKey(t, readFile(t, root, "config.local.yaml"))
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	testarossa.NoError(t, err)
	_, ok := parsed.(ed25519.PrivateKey)
	testarossa.True(t, ok, "key is not Ed25519")
}

func TestGeninit_Idempotent(t *testing.T) {
	root := t.TempDir()
	g := &generator{root: root}
	testarossa.NoError(t, g.run())
	keyBefore := extractKey(t, readFile(t, root, "config.local.yaml"))

	// A second run must not overwrite files nor mint a new key.
	testarossa.NoError(t, g.run())
	keyAfter := extractKey(t, readFile(t, root, "config.local.yaml"))
	testarossa.Equal(t, keyBefore, keyAfter)
}

func TestGeninit_MergesExistingSettings(t *testing.T) {
	existing := `{
  "permissions": { "allow": ["Bash(npm:*)", "Bash(go vet:*)"] },
  "env": { "FOO": "bar" }
}`
	merged, changed, err := mergeSettings(existing, template("settings.json.txt"))
	testarossa.NoError(t, err)
	testarossa.True(t, changed)
	testarossa.Contains(t, merged, `"FOO": "bar"`)                    // preserved
	testarossa.Contains(t, merged, `"Bash(npm:*)"`)                   // preserved
	testarossa.Contains(t, merged, `"Bash(go build:*)"`)              // added
	testarossa.Equal(t, 1, strings.Count(merged, `"Bash(go vet:*)"`)) // not duplicated
}

func TestGeninit_PreservesUserMain(t *testing.T) {
	root := t.TempDir()
	custom := "package main\n\nfunc main() { /* mine */ }\n"
	testarossa.NoError(t, os.MkdirAll(filepath.Join(root, "main"), 0755))
	testarossa.NoError(t, os.WriteFile(filepath.Join(root, "main/main.go"), []byte(custom), 0644))

	g := &generator{root: root}
	testarossa.NoError(t, g.run())
	testarossa.Equal(t, custom, readFile(t, root, "main/main.go"))
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	testarossa.NoError(t, err)
	return string(b)
}

func extractKey(t *testing.T, configLocal string) []byte {
	t.Helper()
	for line := range strings.SplitSeq(configLocal, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "PrivateKey:")
		if ok {
			der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rest))
			testarossa.NoError(t, err)
			return der
		}
	}
	t.Fatal("no PrivateKey in config.local.yaml")
	return nil
}
