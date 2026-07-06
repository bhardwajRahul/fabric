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
	"testing"

	"github.com/microbus-io/testarossa"
)

// writeDefinition writes a minimal api-package definition.go into a temp dir and returns the dir.
func writeDefinition(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	src := "package svcapi\n\nimport \"github.com/microbus-io/fabric/define\"\n\n" +
		"const Hostname = \"my.service\"\n\n" + body + "\n"
	err := os.WriteFile(filepath.Join(dir, "definition.go"), []byte(src), 0o644)
	testarossa.For(t).NoError(err)
	return dir
}

// TestParseServiceRejectsBacktickConfigDefault pins that a config whose Default value contains a
// backtick is rejected, because the value is embedded into a Go raw string literal in the generated
// code where a backtick would close the literal early and yield uncompilable Go.
func TestParseServiceRejectsBacktickConfigDefault(t *testing.T) {
	assert := testarossa.For(t)
	// A double-quoted Go string may hold a literal backtick.
	body := "var MyConfig = define.Config{ Default: \"wei" + "`" + "rd\" }"
	_, err := parseService(writeDefinition(t, body))
	if assert.Error(err) {
		assert.Contains(err.Error(), "MyConfig")
		assert.Contains(err.Error(), "backtick")
	}
}

// TestParseServiceAcceptsCleanConfigDefault pins that an ordinary config default parses.
func TestParseServiceAcceptsCleanConfigDefault(t *testing.T) {
	assert := testarossa.For(t)
	body := "var MyConfig = define.Config{ Default: \"normal\" }"
	_, err := parseService(writeDefinition(t, body))
	assert.NoError(err)
}

// TestParseServiceRejectsBacktickGodoc pins the sibling rule for a feature's godoc.
func TestParseServiceRejectsBacktickGodoc(t *testing.T) {
	assert := testarossa.For(t)
	body := "// MyConfig holds a wei" + "`" + "rd value.\nvar MyConfig = define.Config{ Default: \"x\" }"
	_, err := parseService(writeDefinition(t, body))
	if assert.Error(err) {
		assert.Contains(err.Error(), "MyConfig")
		assert.Contains(err.Error(), "backtick")
	}
}

// TestParseServiceRejectsNonLiteralRequiredClaims pins that a RequiredClaims factored into a const is
// rejected with an actionable error rather than silently producing an ungated endpoint.
func TestParseServiceRejectsNonLiteralRequiredClaims(t *testing.T) {
	assert := testarossa.For(t)
	body := "const gate = \"roles.customer\"\n" +
		"var Secret = define.Function{ Host: Hostname, Method: \"GET\", Route: \"/secret\", RequiredClaims: gate }"
	_, err := parseService(writeDefinition(t, body))
	if assert.Error(err) {
		assert.Contains(err.Error(), "Secret")
		assert.Contains(err.Error(), "RequiredClaims")
		assert.Contains(err.Error(), "string literal")
	}
}

// TestParseServiceAcceptsLiteralRequiredClaims pins that the inlined literal form parses cleanly.
func TestParseServiceAcceptsLiteralRequiredClaims(t *testing.T) {
	assert := testarossa.For(t)
	body := "var Secret = define.Function{ Host: Hostname, Method: \"GET\", Route: \"/secret\", RequiredClaims: \"roles.customer\" }"
	_, err := parseService(writeDefinition(t, body))
	assert.NoError(err)
}
