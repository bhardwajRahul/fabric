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
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/examples/embedder/embedderapi"
	"github.com/microbus-io/pyvenv"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ embedderapi.Client
)

/*
Service implements the embedder.example microservice.

Embedder loads a sentence-transformers model in an in-process Python virtual environment via
the [github.com/microbus-io/pyvenv] module and exposes two typed Go endpoints that delegate
to it: Embed returns the vector for one string; Similarity returns the cosine similarity
between two strings. The model loads on first Start of the venv and stays warm for subsequent
calls.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	venv *pyvenv.Venv

	initMu     sync.Mutex
	initStatus string // "not_started" | "pending" | "ready" | "error"
	initError  string
}

// OnStartup is called when the microservice is started up. The Python venv is constructed
// but not started; the demo page's "Initialize Python VM" button triggers the actual Start
// in the background so the multi-second pip install + model download is visible to the user
// rather than buried in the startup banner.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	sources, err := readPythonSources()
	if err != nil {
		return errors.Trace(err)
	}
	svc.venv, err = pyvenv.New(pyvenv.Config{
		Sources:          sources,
		Requirements:     parseRequirements(pythonRequirements),
		MaxWorkers:       svc.MaxWorkers(),
		Logger:           svc,
		LivenessCallback: svc.onVenvLiveness,
	})
	if err != nil {
		return errors.Trace(err)
	}
	svc.setInitStatus("not_started", "")
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	if svc.venv != nil {
		err := svc.venv.Close(ctx)
		if err != nil {
			svc.LogError(ctx, "Closing python venv failed", "error", err)
		}
	}
	return nil
}

// onVenvLiveness reacts to async venv lifecycle transitions. StateReady activates the
// python-tagged subscriptions and flips initStatus to "ready"; StateDied deactivates them
// and flips initStatus to "error" so the demo page surfaces the failure. Start failures
// (no StateReady → no StateDied) are handled by the goroutine in DemoInit.
func (svc *Service) onVenvLiveness(state pyvenv.State, err error) {
	ctx := svc.Lifetime()
	switch state {
	case pyvenv.StateReady:
		actErr := svc.activatePythonSubs()
		if actErr != nil {
			svc.LogError(ctx, "Activating python subs", "error", actErr)
		}
		svc.setInitStatus("ready", "")
	case pyvenv.StateDied:
		svc.LogWarn(ctx, "Python venv died", "error", err)
		dErr := svc.deactivatePythonSubs()
		if dErr != nil {
			svc.LogError(ctx, "Deactivating python subs", "error", dErr)
		}
		msg := "python subprocess exited"
		if err != nil {
			msg = err.Error()
		}
		svc.setInitStatus("error", msg)
	}
}

func (svc *Service) setInitStatus(status, msg string) {
	svc.initMu.Lock()
	svc.initStatus = status
	svc.initError = msg
	svc.initMu.Unlock()
}

// Embed returns the sentence-embedding vector for the input text.
func (svc *Service) Embed(ctx context.Context, text string) (vector []float64, err error) { // MARKER: Embed
	if svc.venv == nil || !svc.venv.Ready() {
		return nil, errors.New("venv not ready", http.StatusServiceUnavailable)
	}
	in := embedderapi.EmbedIn{
		Text: text,
	}
	var out embedderapi.EmbedOut
	err = svc.venv.CallAndAwait(ctx, "embed", in, &out)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return out.Vector, nil
}

// Similarity returns the cosine similarity between the embeddings of strings a and b.
func (svc *Service) Similarity(ctx context.Context, a string, b string) (score float64, err error) { // MARKER: Similarity
	if svc.venv == nil || !svc.venv.Ready() {
		return 0, errors.New("venv not ready", http.StatusServiceUnavailable)
	}
	in := embedderapi.SimilarityIn{
		A: a,
		B: b,
	}
	var out embedderapi.SimilarityOut
	err = svc.venv.CallAndAwait(ctx, "similarity", in, &out)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return out.Score, nil
}

// Demo serves the interactive demo page for the embedder.
func (svc *Service) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	err = svc.WriteResTemplate(w, "demo.html", nil)
	return errors.Trace(err)
}

// DemoInit kicks off Python venv Start in the background. Idempotent: a second invocation
// while initialization is pending or already ready returns the current status without
// restarting.
func (svc *Service) DemoInit(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DemoInit
	svc.initMu.Lock()
	switch svc.initStatus {
	case "pending", "ready":
		status := svc.initStatus
		svc.initMu.Unlock()
		return writeJSON(w, map[string]any{"status": status})
	}
	svc.initStatus = "pending"
	svc.initError = ""
	svc.initMu.Unlock()

	svc.Go(r.Context(), func(ctx context.Context) error {
		err := svc.StartPyVenv(ctx)
		if err != nil {
			svc.LogError(ctx, "Python venv Start failed", "error", err)
			svc.setInitStatus("error", err.Error())
			return err
		}
		// Success: the LivenessCallback flips status to "ready" and activates subs.
		return nil
	})
	return writeJSON(w, map[string]any{"status": "pending"})
}

// DemoStatus is a long-poll endpoint. The client passes its last-seen ETag via If-None-Match;
// the server snapshots the current status + tailed logs, hashes them into an ETag, and:
//   - returns 200 with the new ETag and body if the snapshot differs;
//   - holds the connection (polling every 500ms) until the snapshot changes;
//   - returns 204 No Content if the request context is within ~1s of its deadline without any
//     change, so the client can immediately re-issue the request with the same ETag.
//
// A fresh client (no If-None-Match) gets the current snapshot back on the first iteration.
func (svc *Service) DemoStatus(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DemoStatus
	clientETag := r.Header.Get("If-None-Match")
	deadline, hasDeadline := r.Context().Deadline()

	const tickInterval = 500 * time.Millisecond
	const safetyMargin = 1 * time.Second

	for {
		snap := svc.demoStatusSnapshot()
		if snap.etag != clientETag {
			w.Header().Set("ETag", snap.etag)
			return writeJSON(w, map[string]any{
				"status":  snap.status,
				"message": snap.message,
				"logs":    snap.logs,
			})
		}

		if hasDeadline && time.Until(deadline) <= safetyMargin {
			w.Header().Set("ETag", snap.etag)
			w.WriteHeader(http.StatusNoContent)
			return nil
		}

		select {
		case <-r.Context().Done():
			return nil
		case <-time.After(tickInterval):
		}
	}
}

// demoSnapshot captures the current status, error message, and tailed log buffer, plus a
// SHA-1 ETag over the combination. ETag changes when any of the three changes.
type demoSnapshot struct {
	status, message, logs, etag string
}

func (svc *Service) demoStatusSnapshot() demoSnapshot {
	svc.initMu.Lock()
	status := svc.initStatus
	if status == "" {
		status = "not_started"
	}
	msg := svc.initError
	svc.initMu.Unlock()

	logs := ""
	if svc.venv != nil {
		logs = string(svc.venv.TailStdOut()) + string(svc.venv.TailStdErr())
	}

	h := sha1.New()
	h.Write([]byte(status))
	h.Write([]byte{0})
	h.Write([]byte(msg))
	h.Write([]byte{0})
	h.Write([]byte(logs))
	etag := `"` + hex.EncodeToString(h.Sum(nil)) + `"`

	return demoSnapshot{status: status, message: msg, logs: logs, etag: etag}
}

func writeJSON(w http.ResponseWriter, body any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(body)
}
