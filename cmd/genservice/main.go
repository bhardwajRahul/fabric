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
// subdirectory) it generates both the api package's client.go and the service's intermediate.go.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: genservice <dir> [<dir>...]")
		os.Exit(2)
	}
	for _, dir := range os.Args[1:] {
		err := generate(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", dir, err)
			os.Exit(1)
		}
	}
}

// generate detects whether dir is an api package (has definition.go) or a service directory (has an
// <x>api subdirectory with definition.go) and generates the corresponding files.
func generate(dir string) error {
	if fileExists(filepath.Join(dir, "definition.go")) {
		return generateClient(dir)
	}
	apiDir, err := findAPISubdir(dir)
	if err != nil {
		return err
	}
	return generateService(dir, apiDir)
}

// generateClient writes the api package's client.go.
func generateClient(apiDir string) error {
	svc, err := parseService(apiDir)
	if err != nil {
		return err
	}
	path := filepath.Join(apiDir, "client.go")
	src, err := emitClient(svc, existingHeader(path))
	if err != nil {
		return err
	}
	err = os.WriteFile(path, src, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("genservice: wrote %s\n", path)
	return nil
}

// generateService writes the api package's client.go and the service's intermediate.go.
func generateService(dir, apiDir string) error {
	svc, err := parseService(apiDir)
	if err != nil {
		return err
	}
	clientPath := filepath.Join(apiDir, "client.go")
	clientSrc, err := emitClient(svc, existingHeader(clientPath))
	if err != nil {
		return err
	}
	err = os.WriteFile(clientPath, clientSrc, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("genservice: wrote %s\n", clientPath)

	modPath, modDir, err := findModule(dir)
	if err != nil {
		return err
	}
	apiPath := importPathOf(modPath, modDir, apiDir)
	resourcesPath := importPathOf(modPath, modDir, dir) + "/resources"
	pkg, err := packageName(dir)
	if err != nil {
		return err
	}
	resolveSource := func(importPath string) (*service, error) {
		srcDir, ok := inModuleDir(modPath, modDir, importPath)
		if !ok {
			return nil, fmt.Errorf("source package %s is outside module %s", importPath, modPath)
		}
		return parseService(srcDir)
	}
	interPath := filepath.Join(dir, "intermediate.go")
	interSrc, err := emitIntermediate(svc, pkg, apiPath, resourcesPath, existingHeader(interPath), resolveSource)
	if err != nil {
		return err
	}
	err = os.WriteFile(interPath, interSrc, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("genservice: wrote %s\n", interPath)

	mockPath := filepath.Join(dir, "mock.go")
	mockSrc, err := emitMock(svc, pkg, apiPath, existingHeader(mockPath), resolveSource)
	if err != nil {
		return err
	}
	err = os.WriteFile(mockPath, mockSrc, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("genservice: wrote %s\n", mockPath)

	mockTestPath := filepath.Join(dir, "mock_test.go")
	mockTestSrc, err := emitMockTest(svc, pkg, apiPath, existingHeader(mockTestPath), resolveSource)
	if err != nil {
		return err
	}
	err = os.WriteFile(mockTestPath, mockTestSrc, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("genservice: wrote %s\n", mockTestPath)
	return nil
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
