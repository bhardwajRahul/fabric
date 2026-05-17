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

// genmanifest regenerates manifest.yaml deterministically from a microservice's
// source code. It extracts service description, endpoints (functions, webs,
// tasks, workflows), configs, metrics, tickers, outbound events, inbound event
// hooks, and downstream dependencies from intermediate.go, *api/endpoints.go,
// *api/client.go, and the microservice's other non-test .go files (handlers are
// not assumed to live only in service.go). Operator-curated fields (general.name,
// general.cloud, general.frameworkVersion) are preserved across regeneration.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// errCheckDiff is returned by run when --check finds a mismatch between the
// existing manifest.yaml and the regenerated content. main translates it to
// exit code 2; tests assert on the sentinel directly.
var errCheckDiff = errors.New("regeneration would change manifest.yaml")

func main() {
	var (
		path  = flag.String("path", ".", "directory containing the microservice source files")
		check = flag.Bool("check", false, "exit nonzero if regeneration would change the file")
	)
	flag.Parse()

	err := run(*path, *check, time.Now().UTC())
	switch {
	case err == nil:
		return
	case errors.Is(err, errCheckDiff):
		fmt.Fprintln(os.Stderr, "genmanifest:", err)
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "genmanifest:", err)
		os.Exit(1)
	}
}

func run(dir string, check bool, now time.Time) error {
	manifestPath := filepath.Join(dir, "manifest.yaml")

	// Extract everything we can from the source code.
	extracted, err := extract(dir)
	if err != nil {
		return err
	}

	// Read existing manifest (if any) for operator-preserved fields and license header.
	existing, existingBytes, err := readExisting(manifestPath)
	if err != nil {
		return err
	}

	doc := merge(extracted, existing, now)

	// First, render with the existing modifiedAt (if any) to test for content
	// equality. This avoids spurious diffs from the timestamp alone - modifiedAt
	// is bumped only when something else actually changed.
	if existing.General.ModifiedAt != "" {
		doc.General.ModifiedAt = existing.General.ModifiedAt
	}
	out := emit(doc, existingBytes)

	if bytes.Equal(existingBytes, out) {
		// Content unchanged. Preserve modifiedAt; --check passes; write is a no-op.
		return nil
	}

	// Content changed. Bump modifiedAt to now and re-render.
	doc.General.ModifiedAt = now.Format(time.RFC3339)
	out = emit(doc, existingBytes)

	if check {
		return fmt.Errorf("%w: %s", errCheckDiff, manifestPath)
	}
	return os.WriteFile(manifestPath, out, 0o644)
}
