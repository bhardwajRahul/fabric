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

// Package pkgresolver maps Go package import paths to on-disk directories.
//
// It is the shared workhorse for the framework's source-scanning generators
// (cmd/gentopology, cmd/gencreds) which need to walk the *api package of
// every service in a bundle. A naive implementation shells out to
// `go list -json -e <pkg>` once per call - ~150-300ms each - and a typical
// bundle has dozens of services × several *api imports, so the cost adds up
// to tens of seconds per run.
//
// Two optimizations cut that cost:
//
//   - In-module shortcut: for any path inside the resolver's own module
//     (computed from the nearest go.mod walking up from the start dir), the
//     directory is derived by string math against the module root, no
//     subprocess needed. Verified with os.Stat so a misnamed import or a
//     replace directive still falls through to go list.
//   - Cross-service memo: lookups are cached on the [Resolver], so the
//     second through Nth service importing the same *api package is free.
//
// DisableShortcut on [Resolver] forces every lookup through go list, even
// in-module ones. Reserved for tests that need to verify the fallback path
// or assert shortcut/fallback parity; do not flip it in production code.
package pkgresolver

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver maps Go package paths to on-disk directories. Construct with [New].
// Safe for sequential use; concurrent calls require external synchronization.
type Resolver struct {
	modulePath string // empty when go.mod was not found
	moduleRoot string
	cache      map[string]string

	// DisableShortcut forces every Dir call through `go list`, bypassing the
	// in-module string-math fast path. Test-only knob for verifying the
	// fallback or asserting shortcut/fallback parity.
	DisableShortcut bool
}

// New constructs a [Resolver] anchored at the nearest go.mod walking up from
// startDir. When no go.mod is found, the resolver still works - the in-module
// shortcut is disabled and every Dir call falls through to `go list`.
func New(startDir string) *Resolver {
	r := &Resolver{cache: map[string]string{}}
	dir := startDir
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					r.modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
					r.moduleRoot = dir
					return r
				}
			}
			return r
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return r
		}
		dir = parent
	}
}

// ModulePath returns the module path declared in the resolved go.mod, or "" if
// no go.mod was found.
func (r *Resolver) ModulePath() string {
	if r == nil {
		return ""
	}
	return r.modulePath
}

// ModuleRoot returns the on-disk directory of the resolved go.mod, or "" if no
// go.mod was found.
func (r *Resolver) ModuleRoot() string {
	if r == nil {
		return ""
	}
	return r.moduleRoot
}

// Dir returns the on-disk directory of pkgPath. Tries the in-module shortcut
// first (skipped when DisableShortcut is set), falls back to `go list` for
// paths outside the module. Results are memoized. A nil receiver disables both
// shortcut and cache - every call shells out to `go list` - so the receiver
// stays usable in code paths that may not own a resolver.
func (r *Resolver) Dir(pkgPath, fromDir string) (string, error) {
	if r == nil {
		return GoListDir(pkgPath, fromDir)
	}
	if d, ok := r.cache[pkgPath]; ok {
		return d, nil
	}
	if !r.DisableShortcut {
		if d := r.inModuleDir(pkgPath); d != "" {
			r.cache[pkgPath] = d
			return d, nil
		}
	}
	d, err := GoListDir(pkgPath, fromDir)
	if err != nil {
		return "", err
	}
	r.cache[pkgPath] = d
	return d, nil
}

// inModuleDir returns the on-disk directory of pkgPath when it lies inside the
// resolver's module. Empty result means "not in module" (the caller falls back
// to `go list`). os.Stat verifies the computed directory exists so a misnamed
// import or replace directive falls through instead of returning a stale path.
func (r *Resolver) inModuleDir(pkgPath string) string {
	if r.modulePath == "" {
		return ""
	}
	var rel string
	switch {
	case pkgPath == r.modulePath:
		rel = ""
	case strings.HasPrefix(pkgPath, r.modulePath+"/"):
		rel = pkgPath[len(r.modulePath)+1:]
	default:
		return ""
	}
	dir := filepath.Join(r.moduleRoot, rel)
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

// GoListDir runs `go list -json -e <pkgPath>` from fromDir and returns the
// resolved package directory. With the -e flag set, `go list` itself succeeds
// for missing packages (the returned Dir is empty), so a non-nil error here
// always represents a real toolchain failure (broken module, missing `go`,
// malformed go.mod) rather than a missing dependency.
func GoListDir(pkgPath, fromDir string) (string, error) {
	cmd := exec.Command("go", "list", "-json", "-e", pkgPath)
	if fromDir != "" {
		cmd.Dir = fromDir
	}
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("go list %s: %w (%s)", pkgPath, err, stderr)
	}
	var info struct{ Dir string }
	err = json.Unmarshal(out, &info)
	if err != nil {
		return "", fmt.Errorf("go list parse for %s: %w", pkgPath, err)
	}
	return info.Dir, nil
}
