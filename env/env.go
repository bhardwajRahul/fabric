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

// Package env loads environment variables from env.yaml / env.local.yaml files into the
// real OS environment at package init time, following dotenv conventions. See CLAUDE.md
// for design rationale.
package env

import (
	"os"
	"path"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// pushEntry records the OS env state captured by a Push so that Pop can restore it.
type pushEntry struct {
	priorValue string
	priorSet   bool
}

var (
	pushed = map[string][]pushEntry{}
	mux    sync.Mutex
)

func init() {
	Load()
}

// Load walks ancestor directories from the current working directory up to the filesystem
// root, merges all env.yaml and env.local.yaml entries respecting precedence (subdirectory
// over ancestor; .local over non-local), and writes the merged result into the OS
// environment via os.Setenv.
//
// Real OS environment variables set before the binary launches always win — this matches
// the dotenv convention used by godotenv, python-dotenv, Node dotenv, and others. Operators
// who set values via systemd/k8s/docker get those values; yaml is a fallback for keys that
// are otherwise unset.
//
// Load is called automatically at package init. It can be called again to reload yaml
// changes, but in normal use the init-time invocation is sufficient.
func Load() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	// Build the priority order: closer directories override more distant ancestors;
	// .local overrides non-local within a directory. We collect *paths* in priority
	// order (highest priority first), then merge in *reverse* so highest-priority
	// values stick.
	split := strings.Split(wd, string(os.PathSeparator))
	pathsHighToLow := []string{}
	for p := 0; p < len(split); p++ {
		dir := string(os.PathSeparator) + path.Join(split[:len(split)-p]...)
		// Within a directory: local overrides non-local.
		pathsHighToLow = append(pathsHighToLow,
			path.Join(dir, "env.local.yaml"),
			path.Join(dir, "env.yaml"),
		)
	}

	merged := map[string]string{}
	// Iterate low-to-high so high-priority writes overwrite earlier ones.
	for i := len(pathsHighToLow) - 1; i >= 0; i-- {
		file, err := os.Open(pathsHighToLow[i])
		if err != nil {
			continue
		}
		var kv map[string]string
		err = yaml.NewDecoder(file).Decode(&kv)
		file.Close()
		if err != nil {
			continue
		}
		for k, v := range kv {
			merged[k] = v
		}
	}

	// Apply to the OS environment, but never override values already present.
	// Real OS env wins (dotenv convention).
	for k, v := range merged {
		if _, present := os.LookupEnv(k); !present {
			os.Setenv(k, v)
		}
	}
}

// Lookup returns the value of the environment variable.
// It is a thin wrapper over os.LookupEnv. yaml-loaded values are visible because Load
// has populated the OS environment at package init.
// Environment variable keys are case-sensitive.
func Lookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Get returns the value of the environment variable.
// It is a thin wrapper over os.Getenv. yaml-loaded values are visible because Load
// has populated the OS environment at package init.
// Environment variable keys are case-sensitive.
func Get(key string) string {
	return os.Getenv(key)
}

// Push sets an environment variable and remembers the prior OS env state so Pop can
// restore it. Push is goroutine-safe but the global env it mutates is not — tests
// using Push must not run with t.Parallel.
//
// Pushed values are visible to any code reading os.Getenv, including third-party
// libraries that read env vars directly. This is what makes Push/Pop useful for
// controlling third-party SDK behavior in tests.
func Push(key string, value string) {
	mux.Lock()
	defer mux.Unlock()
	prior, set := os.LookupEnv(key)
	pushed[key] = append(pushed[key], pushEntry{priorValue: prior, priorSet: set})
	os.Setenv(key, value)
}

// Pop restores the environment variable to the value captured by the most recent Push.
// Calling Pop without a matching Push panics.
func Pop(key string) {
	mux.Lock()
	defer mux.Unlock()
	entries := pushed[key]
	last := entries[len(entries)-1] // panics on underflow, matching prior behavior
	pushed[key] = entries[:len(entries)-1]
	if last.priorSet {
		os.Setenv(key, last.priorValue)
	} else {
		os.Unsetenv(key)
	}
}
