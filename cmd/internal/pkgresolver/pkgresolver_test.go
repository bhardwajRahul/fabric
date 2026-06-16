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

package pkgresolver

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/microbus-io/testarossa"
)

// repoPath returns an absolute path under the framework's module root,
// independent of the test runner's working directory.
func repoPath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			abs := filepath.Join(dir, rel)
			_, err := os.Stat(abs)
			if err != nil {
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

// TestNew_FindsModule asserts the constructor walks up to find go.mod and
// records the declared module path + root directory.
func TestNew_FindsModule(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	start := repoPath(t, "cmd/internal/pkgresolver")
	r := New(start)
	assert.Equal("github.com/microbus-io/fabric", r.ModulePath())
	assert.NotEqual("", r.ModuleRoot())

	// The root must actually contain the go.mod that declared the path.
	_, err := os.Stat(filepath.Join(r.ModuleRoot(), "go.mod"))
	assert.NoError(err)
}

// TestNew_NoGoMod asserts the resolver degrades gracefully when there is no
// go.mod above the start directory. The shortcut is disabled, every Dir call
// would fall through to go list.
func TestNew_NoGoMod(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r := New(t.TempDir())
	assert.Equal("", r.ModulePath())
	assert.Equal("", r.ModuleRoot())
}

// TestDir_InModule_MatchesGoList asserts the in-module shortcut produces the
// same directory go list does, for representative packages across the module.
// This is the parity check that justifies trusting the shortcut.
func TestDir_InModule_MatchesGoList(t *testing.T) {
	// No parallel - shells out to `go list` once per package with the shortcut
	// disabled, so it's the heaviest test in the package.
	assert := testarossa.For(t)

	start := repoPath(t, "cmd/internal/pkgresolver")
	pkgs := []string{
		"github.com/microbus-io/fabric",                                           // module root itself
		"github.com/microbus-io/fabric/sub",                                       // top-level package
		"github.com/microbus-io/fabric/connector",                                 // top-level package
		"github.com/microbus-io/fabric/dlru",                                      // top-level package
		"github.com/microbus-io/fabric/coreservices/configurator/configuratorapi", // deeply nested
	}
	for _, pkg := range pkgs {
		fast := New(start)
		fastDir, err := fast.Dir(pkg, start)
		assert.NoError(err, "shortcut: %s", pkg)

		slow := New(start)
		slow.DisableShortcut = true
		slowDir, err := slow.Dir(pkg, start)
		assert.NoError(err, "go list: %s", pkg)

		assert.Equal(slowDir, fastDir, "shortcut and go list disagree on %s", pkg)
	}
}

// TestDir_OutOfModule asserts paths outside the module fall through to go list
// even when the shortcut is enabled. We use a standard-library package that is
// guaranteed to exist on any toolchain.
func TestDir_OutOfModule(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	start := repoPath(t, "cmd/internal/pkgresolver")
	r := New(start)
	dir, err := r.Dir("encoding/json", start)
	assert.NoError(err)
	assert.NotEqual("", dir, "encoding/json should resolve via go list")
}

// TestDir_Cached asserts a second lookup of the same path returns the cached
// answer without re-running the resolution. We verify by populating the cache
// directly and observing the cached value comes back even though the path is
// nonsensical (no in-module shortcut, no go list call would succeed).
func TestDir_Cached(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r := New(repoPath(t, "cmd/internal/pkgresolver"))
	r.cache["fictional.example.com/nope"] = "/tmp/cached-answer"
	dir, err := r.Dir("fictional.example.com/nope", "")
	assert.NoError(err)
	assert.Equal("/tmp/cached-answer", dir)
}

// TestDir_NilReceiver asserts a nil [*Resolver] is usable - every call shells
// out to go list with no caching. This lets call sites that may not own a
// resolver still invoke .Dir safely.
func TestDir_NilReceiver(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var r *Resolver
	dir, err := r.Dir("github.com/microbus-io/fabric/sub", repoPath(t, "cmd/internal/pkgresolver"))
	assert.NoError(err)
	assert.NotEqual("", dir)
}

// TestDir_StaleInModulePath asserts the shortcut returns "" (rather than a
// non-existent directory) for an in-module-shaped path whose directory doesn't
// actually exist. The caller then falls through to go list, which returns the
// empty Dir field for an unknown package - matching the contract.
func TestDir_StaleInModulePath(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r := New(repoPath(t, "cmd/internal/pkgresolver"))
	// Path shaped like an in-module package but no such directory exists.
	dir, err := r.Dir("github.com/microbus-io/fabric/nosuch_package_anywhere", "")
	// go list returns empty Dir + no error for missing packages with the -e flag.
	assert.NoError(err)
	assert.Equal("", dir)
}
