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

// Command genupgrade is the deterministic arm of the versioned upgrade skills. Each fabric version that
// requires a mechanical source-code change registers an upgrade routine here; an upgrade skill invokes
// `genupgrade -v <version> -path <microservice dir>` for the mechanical part and handles the judgment
// parts itself. genupgrade operates on one microservice directory at a time and never calls other
// generators; the upgrade skill runs genservice as a separate step afterward.
//
// Registered upgrades are append-only and frozen once shipped: a routine describes the one-time change a
// specific version introduced, exactly like a versioned upgrade skill, and is never rewritten.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// upgrades maps a target fabric version to its mechanical upgrade routine, keyed by the version the
// upgrade brings a project up to. Each routine defines its own scope: -path is whatever directory that
// version operates on - a microservice directory (like 1.41.0) or the project root - and the paired
// upgrade skill invokes genupgrade accordingly (looping over microservices, or once at the root).
var upgrades = map[string]func(dir string) error{
	"1.41.0": upgradeV1_41_0,
}

func main() {
	version := flag.String("v", "", "fabric version to upgrade to, e.g. 1.41.0")
	path := flag.String("path", ".", "directory to upgrade (a microservice directory or the project root, per the version)")
	flag.Parse()

	if *version == "" {
		fmt.Fprintf(os.Stderr, "genupgrade: -v <version> is required; supported: %s\n", supportedVersions())
		os.Exit(2)
	}
	upgrade, ok := upgrades[*version]
	if !ok {
		fmt.Fprintf(os.Stderr, "genupgrade: no upgrade for version %q; supported: %s\n", *version, supportedVersions())
		os.Exit(2)
	}
	err := upgrade(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genupgrade: %s: %v\n", *path, err)
		os.Exit(1)
	}
}

// supportedVersions lists the registered target versions, sorted, for usage messages.
func supportedVersions() string {
	vs := make([]string, 0, len(upgrades))
	for v := range upgrades {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	return strings.Join(vs, ", ")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
