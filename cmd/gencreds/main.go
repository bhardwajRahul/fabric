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

// gencreds reads a microservice bundle's source code, derives per-service
// NATS ACL rule sets via AST analysis, and signs them into
// <hostname>_nats.creds files using the operator's account NKey. Runs at
// deploy time.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	bundle := flag.String("bundle", "", "path to main.go that lists services via app.Add(...)")
	manifests := flag.String("manifests", "", "comma-separated list of service directories (alternative to --bundle)")
	signingKey := flag.String("signing-key", "", "path to operator's account NKey seed file (private)")
	out := flag.String("out", "./creds", "output directory for <hostname>_nats.creds files")
	plane := flag.String("plane", "microbus", "plane the issued .creds are tied to (default: microbus)")
	rotate := flag.Bool("rotate-user-nkeys", true, "generate fresh user NKeys on every run (default)")
	persist := flag.String("persist-user-nkeys", "", "directory for stable per-service user NKeys (overrides --rotate-user-nkeys)")
	expiration := flag.Duration("expiration", 0, "lifetime of the signed user JWT (e.g. 720h); 0 = no expiration")
	flag.Parse()

	if *signingKey == "" {
		fmt.Fprintln(os.Stderr, "gencreds: --signing-key is required")
		os.Exit(1)
	}
	if *bundle == "" && *manifests == "" {
		fmt.Fprintln(os.Stderr, "gencreds: one of --bundle or --manifests is required")
		os.Exit(1)
	}

	cfg := config{
		bundle:     *bundle,
		manifests:  *manifests,
		signingKey: *signingKey,
		out:        *out,
		plane:      *plane,
		rotate:     *rotate,
		persist:    *persist,
		expiration: *expiration,
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "gencreds:", err)
		var be *budgetErr
		if errors.As(err, &be) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// config holds the resolved CLI flags. It is the single argument to run() so
// tests can drive the tool without exec'ing the binary.
type config struct {
	bundle     string
	manifests  string
	signingKey string
	out        string
	plane      string
	rotate     bool
	persist    string
	expiration time.Duration
}

func run(cfg config) error {
	services, err := resolveBundle(cfg.bundle, cfg.manifests)
	if err != nil {
		return fmt.Errorf("resolve bundle: %w", err)
	}
	if len(services) == 0 {
		return fmt.Errorf("bundle resolved to zero services")
	}

	accountKP, err := loadAccountKey(cfg.signingKey)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}

	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	for _, s := range services {
		creds, err := signService(s, accountKP, cfg)
		if err != nil {
			return fmt.Errorf("sign %s: %w", s.Hostname, err)
		}
		dst := fmt.Sprintf("%s/%s_nats.creds", cfg.out, s.Hostname)
		if err := os.WriteFile(dst, creds, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
	}
	return nil
}
