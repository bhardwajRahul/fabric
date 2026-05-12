package myservice

import (
	"context"
	"embed"
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/pyvenv"
)

//go:embed *.py
var pythonSources embed.FS

//go:embed requirements.txt
var pythonRequirements []byte

// StartPyVenv synchronously starts the Python venv: pip install (skipped if requirements
// haven't changed), spawn the worker, load the embedded sources. Returns when the worker
// is Ready or on error. Production startup launches this in the background from OnStartup;
// tests opt in by calling it directly when they want to exercise the real Python path.
func (svc *Service) StartPyVenv(ctx context.Context) error {
	return svc.venv.Start(ctx)
}

// onVenvLiveness reacts to async venv lifecycle transitions. StateReady activates the
// python-tagged subscriptions so calls into the venv accept traffic. StateDied deactivates
// them so callers see a clean 404 ack-timeout until recovery completes.
func (svc *Service) onVenvLiveness(state pyvenv.State, err error) {
	ctx := svc.Lifetime()
	switch state {
	case pyvenv.StateReady:
		actErr := svc.activatePythonSubs()
		if actErr != nil {
			svc.LogError(ctx, "Activating python subs", "error", actErr)
		}
	case pyvenv.StateDied:
		svc.LogWarn(ctx, "Python venv died", "error", err)
		dErr := svc.deactivatePythonSubs()
		if dErr != nil {
			svc.LogError(ctx, "Deactivating python subs", "error", dErr)
		}
		// Recover: restart the venv in the background. svc.venv.Start reuses the on-disk
		// venv and skips pip install if requirements are unchanged, so recovery is fast.
		svc.Go(ctx, func(ctx context.Context) error {
			rErr := svc.venv.Start(ctx)
			if rErr != nil {
				svc.LogError(ctx, "Restarting python venv failed", "error", rErr)
			}
			return nil
		})
	}
}

// activatePythonSubs brings every "python"-tagged subscription onto the bus. Iterating
// svc.Subscriptions() and matching the tag keeps Python endpoints separate from any other
// manual subscriptions the microservice may own.
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

// deactivatePythonSubs takes every "python"-tagged subscription off the bus. Non-Python subs
// stay reachable so the microservice can keep serving control-plane and UI traffic while the
// venv recovers.
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
