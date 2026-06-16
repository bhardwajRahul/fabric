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

// gentopology renders the application's topology diagram (a Mermaid graph)
// from source. For each service in the bundle, it detects: the *api
// dependencies on other services, event-source subscriptions, SQL usage
// (database/sql, sequel, or dwarf engine), and HTTP egress + the external host(s) targeted.
// The resulting graph captures the runtime relationships an operator needs
// to see - what depends on what, what touches a database, what calls out
// to external clouds.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// errCheckDiff is returned by run when --check finds the existing topology
// file differs from what the source-driven scan would produce. main translates
// it to exit code 2 so CI can distinguish a drift detection from arbitrary
// failures.
var errCheckDiff = errors.New("regeneration would change the topology file")

func main() {
	bundle := flag.String("bundle", "", "path to main.go that lists services via app.Add(...)")
	manifests := flag.String("manifests", "", "comma-separated list of service directories (alternative to --bundle)")
	out := flag.String("out", "", "path for the topology .mmd file (default: <bundle dir>/topology.mmd)")
	check := flag.Bool("check", false, "exit nonzero if regeneration would change the file")
	flag.Parse()

	if *bundle == "" && *manifests == "" {
		fmt.Fprintln(os.Stderr, "gentopology: one of --bundle or --manifests is required")
		os.Exit(1)
	}

	outPath := *out
	if outPath == "" && *bundle != "" {
		abs, _ := filepath.Abs(*bundle)
		outPath = filepath.Join(filepath.Dir(abs), "topology.mmd")
	}
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "gentopology: --out is required when --manifests is used without --bundle")
		os.Exit(1)
	}

	cfg := config{
		bundle:    *bundle,
		manifests: *manifests,
		out:       outPath,
		check:     *check,
	}
	err := run(cfg)
	switch {
	case err == nil:
		return
	case errors.Is(err, errCheckDiff):
		fmt.Fprintln(os.Stderr, "gentopology:", err)
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "gentopology:", err)
		os.Exit(1)
	}
}

// config holds resolved CLI flags. Single-arg form lets tests drive run()
// without exec'ing the binary.
type config struct {
	bundle    string
	manifests string
	out       string
	check     bool
}

func run(cfg config) error {
	services, err := resolveBundle(cfg.bundle, cfg.manifests)
	if err != nil {
		return fmt.Errorf("resolve bundle: %w", err)
	}
	if len(services) == 0 {
		return fmt.Errorf("bundle resolved to zero services")
	}
	bundleDir := ""
	if cfg.bundle != "" {
		abs, _ := filepath.Abs(cfg.bundle)
		bundleDir = filepath.Dir(abs)
	}
	scanned, err := scanAll(services, bundleDir)
	if err != nil {
		return err
	}
	got := renderMermaid(scanned)

	if cfg.check {
		existing, err := os.ReadFile(cfg.out)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(existing, got) {
			return fmt.Errorf("%w: %s", errCheckDiff, cfg.out)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfg.out, got, 0o644)
}
