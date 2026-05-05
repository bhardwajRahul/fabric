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
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// readHostnameForAPI resolves the api package and returns the value of its
// `Hostname` constant. fromDir is used to scope `go list` to the local module.
//
// Returns an error if the package can't be resolved. Empty hostname is also an
// error since a manifest without a downstream hostname is malformed.
func readHostnameForAPI(apiPkg string, fromDir string) (string, error) {
	hostname, _, err := readAPIPackage(apiPkg, fromDir)
	if err != nil {
		return "", err
	}
	return hostname, nil
}

// readAPIPackage resolves the api package and returns its `Hostname` constant
// plus the parsed Def map (var name → Method/Route). Used both to resolve a
// downstream's hostname and to look up the route/method an inbound event or a
// downstream method invocation pins down.
func readAPIPackage(apiPkg string, fromDir string) (hostname string, defs map[string]def, err error) {
	dir, err := resolvePackageDir(apiPkg, fromDir)
	if err != nil {
		return "", nil, err
	}
	if dir == "" {
		return "", nil, fmt.Errorf("could not resolve %s", apiPkg)
	}
	endpointsPath := filepath.Join(dir, "endpoints.go")
	defs, hostname, err = parseEndpoints(endpointsPath)
	if err != nil {
		return "", nil, err
	}
	if hostname == "" {
		return "", nil, fmt.Errorf("no Hostname constant in %s", endpointsPath)
	}
	return hostname, defs, nil
}

// resolvePackageDir uses `go list -json -e` to find the on-disk directory of a
// Go package. Empty result means the package isn't resolvable.
func resolvePackageDir(pkgPath string, fromDir string) (string, error) {
	cmd := exec.Command("go", "list", "-json", "-e", pkgPath)
	if fromDir != "" {
		cmd.Dir = fromDir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	var info struct {
		Dir string
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("parse `go list` output for %s: %w", pkgPath, err)
	}
	return info.Dir, nil
}

// lastSegment returns the final "/"-separated component of a package path.
func lastSegment(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}

// parentPackage returns the package path one level up. It's used to resolve a
// `*api` package back to its enclosing service package.
func parentPackage(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[:idx]
}
