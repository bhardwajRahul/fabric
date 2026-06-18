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

// Command genservice generates a microservice's boilerplate from its api package definition.go.
// Given an api package directory it generates client.go; given a service directory (with an <x>api
// subdirectory) it generates client.go, intermediate.go, mock.go, mock_test.go, and manifest.yaml.
//
// Usage: genservice [-check] <dir> [<dir>...]. With -check, nothing is written; the command exits 2 if
// any output is out of date (a CI guard), 1 on error, 0 when everything is current.
package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// output is one file the generator produces: its path and gofmt'd/serialized content.
type output struct {
	path    string
	content []byte
}

func main() {
	check := false
	var dirs []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-check", "--check":
			check = true
		default:
			dirs = append(dirs, arg)
		}
	}
	if len(dirs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: genservice [-check] <dir> [<dir>...]")
		os.Exit(2)
	}

	stale := false
	for _, dir := range dirs {
		outs, err := emitAll(dir, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", dir, err)
			os.Exit(1)
		}
		for _, o := range outs {
			if check {
				existing, _ := os.ReadFile(o.path)
				if !bytes.Equal(existing, o.content) {
					fmt.Fprintf(os.Stderr, "genservice: %s is out of date\n", o.path)
					stale = true
				}
				continue
			}
			err = os.WriteFile(o.path, o.content, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", o.path, err)
				os.Exit(1)
			}
			fmt.Printf("genservice: wrote %s\n", o.path)
		}
	}
	if check && stale {
		os.Exit(2)
	}
}

// emitAll detects whether dir is an api package (has definition.go) or a service directory (has an
// <x>api subdirectory with definition.go) and returns the files to generate, without writing them.
// now is the RFC 3339 timestamp used for a fresh or content-changed manifest.
func emitAll(dir, now string) ([]output, error) {
	if fileExists(filepath.Join(dir, "definition.go")) {
		return emitAllClient(dir)
	}
	apiDir, err := findAPISubdir(dir)
	if err != nil {
		return nil, err
	}
	return emitAllService(dir, apiDir, now)
}

// emitAllClient produces the api package's client.go.
func emitAllClient(apiDir string) ([]output, error) {
	svc, err := parseService(apiDir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(apiDir, "client.go")
	src, err := emitClient(svc, existingHeader(path))
	if err != nil {
		return nil, err
	}
	return []output{{path, src}}, nil
}

// emitAllService produces the api package's client.go plus the service's intermediate.go, mock.go,
// mock_test.go, and manifest.yaml.
func emitAllService(dir, apiDir, now string) ([]output, error) {
	svc, err := parseService(apiDir)
	if err != nil {
		return nil, err
	}
	modPath, modDir, err := findModule(dir)
	if err != nil {
		return nil, err
	}
	apiPath := importPathOf(modPath, modDir, apiDir)
	resourcesPath := importPathOf(modPath, modDir, dir) + "/resources"
	pkgPath := importPathOf(modPath, modDir, dir)
	pkg, err := packageName(dir)
	if err != nil {
		return nil, err
	}
	resolveSource := func(importPath string) (*service, error) {
		srcDir, ok := inModuleDir(modPath, modDir, importPath)
		if !ok {
			return nil, fmt.Errorf("source package %s is outside module %s", importPath, modPath)
		}
		return parseService(srcDir)
	}

	clientPath := filepath.Join(apiDir, "client.go")
	clientSrc, err := emitClient(svc, existingHeader(clientPath))
	if err != nil {
		return nil, err
	}
	interPath := filepath.Join(dir, "intermediate.go")
	interSrc, err := emitIntermediate(svc, pkg, apiPath, resourcesPath, existingHeader(interPath), resolveSource)
	if err != nil {
		return nil, err
	}
	mockPath := filepath.Join(dir, "mock.go")
	mockSrc, err := emitMock(svc, pkg, apiPath, existingHeader(mockPath), resolveSource)
	if err != nil {
		return nil, err
	}
	mockTestPath := filepath.Join(dir, "mock_test.go")
	mockTestSrc, err := emitMockTest(svc, pkg, apiPath, existingHeader(mockTestPath), resolveSource)
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(dir, "manifest.yaml")
	existingManifest, _ := os.ReadFile(manifestPath)
	manifestSrc, err := emitManifest(svc, pkgPath, now, existingManifest, resolveSource)
	if err != nil {
		return nil, err
	}
	return []output{
		{clientPath, clientSrc},
		{interPath, interSrc},
		{mockPath, mockSrc},
		{mockTestPath, mockTestSrc},
		{manifestPath, manifestSrc},
	}, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// findAPISubdir returns the first subdirectory of dir that contains a definition.go.
func findAPISubdir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cand := filepath.Join(dir, e.Name())
		if fileExists(filepath.Join(cand, "definition.go")) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no api subdirectory with a definition.go found under %s", dir)
}

// findModule walks up from dir to the nearest go.mod and returns its module path and directory.
func findModule(dir string) (modulePath, moduleDir string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for d := abs; ; {
		data, readErr := os.ReadFile(filepath.Join(d, "go.mod"))
		if readErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				rest, found := strings.CutPrefix(strings.TrimSpace(line), "module ")
				if found {
					return strings.TrimSpace(rest), d, nil
				}
			}
			return "", "", fmt.Errorf("no module line in %s/go.mod", d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", "", fmt.Errorf("go.mod not found above %s", dir)
		}
		d = parent
	}
}

// importPathOf computes the import path of targetDir within the module.
func importPathOf(modulePath, moduleDir, targetDir string) string {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return modulePath
	}
	rel, err := filepath.Rel(moduleDir, abs)
	if err != nil || rel == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(rel)
}

// inModuleDir maps an in-module import path to its on-disk directory via string math against the
// module root. Returns false for paths outside the module (which would need `go list` to resolve).
func inModuleDir(modulePath, moduleDir, importPath string) (string, bool) {
	if importPath == modulePath {
		return moduleDir, true
	}
	rest, found := strings.CutPrefix(importPath, modulePath+"/")
	if !found {
		return "", false
	}
	return filepath.Join(moduleDir, filepath.FromSlash(rest)), true
}

// packageName parses the Go package name declared in dir.
func packageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.PackageClauseOnly)
		if err != nil {
			return "", err
		}
		return f.Name.Name, nil
	}
	return "", fmt.Errorf("no Go files in %s", dir)
}
