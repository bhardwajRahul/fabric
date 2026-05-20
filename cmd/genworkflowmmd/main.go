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

// genworkflowmmd regenerates Mermaid diagrams for every workflow graph
// endpoint exposed by a microservice. It AST-scans the microservice directory
// for methods on *Service of the form
//
//	func (svc *Service) Foo(ctx context.Context) (graph *workflow.Graph, err error)
//
// generates a throwaway main program under <microservice>/tmp/ that imports
// the microservice package, instantiates the service, invokes each graph
// builder, and writes graph.Mermaid() to <microservice>/<METHOD>.mmd, runs
// it with `go run`, and deletes the tmp directory.
//
// Mechanizes the housekeeping skill's "Visualize Workflows" step so it can be
// invoked unattended from a script or CI.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	var (
		path    = flag.String("path", ".", "directory containing the microservice source files")
		keepTmp = flag.Bool("keep-tmp", false, "do not delete the generated tmp/ directory after running (for debugging)")
	)
	flag.Parse()

	err := run(*path, *keepTmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genworkflowmmd:", err)
		os.Exit(1)
	}
}

func run(path string, keepTmp bool) error {
	dir, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve --path: %w", err)
	}

	workflows, err := scanWorkflows(dir)
	if err != nil {
		return fmt.Errorf("scan workflows: %w", err)
	}
	if len(workflows) == 0 {
		fmt.Println("no workflow graph endpoints found in", dir)
		return nil
	}

	pkgName, err := packageName(dir)
	if err != nil {
		return fmt.Errorf("read package name: %w", err)
	}

	importPath, err := importPathOf(dir)
	if err != nil {
		return fmt.Errorf("resolve import path: %w", err)
	}

	tmpDir := filepath.Join(dir, "tmp")
	err = os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}
	if !keepTmp {
		defer os.RemoveAll(tmpDir)
	}

	src, err := generateMain(importPath, pkgName, dir, workflows)
	if err != nil {
		return fmt.Errorf("generate main.go: %w", err)
	}
	mainGo := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainGo, src, 0o644)
	if err != nil {
		return fmt.Errorf("write tmp/main.go: %w", err)
	}

	cmd := exec.Command("go", "run", mainGo)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("go run %s: %w", mainGo, err)
	}
	return nil
}
