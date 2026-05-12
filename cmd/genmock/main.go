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

// genmock regenerates a microservice's mock.go from the ToDo interface declared
// in its intermediate.go. Each method becomes a (mockX field, MockX setter,
// X executor) triple. Methods whose signature returns *workflow.Graph are
// rendered with the workflow-graph mock pattern, which builds a synthetic
// single-task subgraph that delegates to the supplied handler.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// errCheckDiff is returned by run when --check finds a mismatch between the
// existing mock.go and the regenerated content. main translates it to exit
// code 2; tests assert on the sentinel directly.
var errCheckDiff = errors.New("regeneration would change mock.go")

func main() {
	var (
		path  = flag.String("path", ".", "directory containing the microservice source files")
		check = flag.Bool("check", false, "exit nonzero if regeneration would change the file")
	)
	flag.Parse()

	err := run(*path, *check)
	switch {
	case err == nil:
		return
	case errors.Is(err, errCheckDiff):
		fmt.Fprintln(os.Stderr, "genmock:", err)
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "genmock:", err)
		os.Exit(1)
	}
}

func run(dir string, check bool) error {
	gen, err := generate(dir)
	if err != nil {
		return err
	}

	type target struct {
		path     string
		existing []byte
		out      []byte
	}

	mockPath := filepath.Join(dir, "mock.go")
	mockTestPath := filepath.Join(dir, "mock_test.go")

	existingMock, _ := os.ReadFile(mockPath)
	outMock, err := emit(gen, existingMock)
	if err != nil {
		return err
	}

	existingMockTest, _ := os.ReadFile(mockTestPath)
	outMockTest, err := emitMockTest(gen, existingMockTest)
	if err != nil {
		return err
	}

	targets := []target{
		{mockPath, existingMock, outMock},
		{mockTestPath, existingMockTest, outMockTest},
	}

	for _, t := range targets {
		if bytes.Equal(t.existing, t.out) {
			continue
		}
		if check {
			return fmt.Errorf("%w: %s", errCheckDiff, t.path)
		}
		if err := os.WriteFile(t.path, t.out, 0o644); err != nil {
			return err
		}
	}
	return nil
}
