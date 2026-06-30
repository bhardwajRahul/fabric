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

package embedder

import (
	"context"
	"embed"
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/microbus-io/errors"
)

//go:embed *.py
var pythonSources embed.FS

//go:embed requirements.txt
var pythonRequirements []byte

// StartPyVenv synchronously starts the Python venv: pip install (skipped if requirements
// haven't changed), spawn the worker, load the embedded sources. Returns when the worker
// is Ready or on error. Tests opt in to running real Python by calling this directly.
func (svc *Service) StartPyVenv(ctx context.Context) error {
	return svc.venv.Start(ctx)
}

// activatePythonSubs brings every "python"-tagged subscription onto the bus. Called from the
// pyvenv.LivenessCallback when the venv reaches StateReady.
func (svc *Service) activatePythonSubs() error {
	for _, s := range svc.Subscriptions() {
		if !slices.Contains(s.Tags, "python") {
			continue
		}
		err := svc.ActivateSubscription(s.Name)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// deactivatePythonSubs takes every "python"-tagged subscription off the bus. Called from the
// pyvenv.LivenessCallback on StateDied so non-Python endpoints (e.g. /demo) keep serving
// while the venv is unhealthy.
func (svc *Service) deactivatePythonSubs() error {
	for _, s := range svc.Subscriptions() {
		if !slices.Contains(s.Tags, "python") {
			continue
		}
		err := svc.DeactivateSubscription(s.Name)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// parseRequirements splits requirements.txt into a list of pip packages, skipping blank lines
// and comments.
func parseRequirements(data []byte) []string {
	var pkgs []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pkgs = append(pkgs, line)
	}
	return pkgs
}

// readPythonSources walks the embedded *.py files and returns their contents in the order they
// should be Define-loaded into the venv: all helpers alphabetically first, then service.py
// last so service.py can reference names from earlier modules.
func readPythonSources() ([]string, error) {
	entries, err := fs.ReadDir(pythonSources, ".")
	if err != nil {
		return nil, errors.Trace(err)
	}
	var names []string
	hasServicePy := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		if e.Name() == "service.py" {
			hasServicePy = true
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if hasServicePy {
		names = append(names, "service.py")
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(pythonSources, name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		out = append(out, string(data))
	}
	return out, nil
}
