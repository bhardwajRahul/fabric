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

package saturatedbandflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/saturatedbandflow/saturatedbandflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ saturatedbandflowapi.Client
)

/*
Service implements saturatedbandflow.verify, exercising the foreman's saturated-band fallthrough
in runRefill.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu sync.Mutex

	// Bounded task state: self-429s above cap.
	boundedCap        int
	boundedInFlight   int
	boundedRejections int
	boundedDwell      time.Duration

	// Open task state.
	openDwell time.Duration

	// Completion order, in insertion order. Each entry is a (tag, time) pair.
	completions []completion
}

type completion struct {
	tag string
	at  time.Time
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	if svc.boundedCap == 0 {
		svc.boundedCap = 2
	}
	if svc.boundedDwell == 0 {
		svc.boundedDwell = 80 * time.Millisecond
	}
	if svc.openDwell == 0 {
		svc.openDwell = 40 * time.Millisecond
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// SetBoundedCap sets the cap above which Bounded self-emits 429.
func (svc *Service) SetBoundedCap(cap int) *Service {
	svc.boundedCap = cap
	return svc
}

// Rejections returns how many 429s the Bounded task emitted.
func (svc *Service) Rejections() int {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.boundedRejections
}

// Completions returns the ordered list of (tag, completion-time) pairs.
func (svc *Service) Completions() []completion {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	out := make([]completion, len(svc.completions))
	copy(out, svc.completions)
	return out
}

/*
Bounded self-emits http.StatusTooManyRequests above boundedCap concurrent in-flight executions.
*/
func (svc *Service) Bounded(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: Bounded
	svc.mu.Lock()
	svc.boundedInFlight++
	if svc.boundedInFlight > svc.boundedCap {
		svc.boundedInFlight--
		svc.boundedRejections++
		svc.mu.Unlock()
		return false, errors.New("saturated", http.StatusTooManyRequests)
	}
	svc.mu.Unlock()

	err = svc.Sleep(ctx, svc.boundedDwell)

	svc.mu.Lock()
	svc.boundedInFlight--
	svc.completions = append(svc.completions, completion{tag: tag, at: time.Now()})
	svc.mu.Unlock()
	if err != nil {
		return false, errors.Trace(err)
	}
	return true, nil
}

/*
Open always succeeds; sleeps briefly to simulate work and to keep concurrent dispatch overlapping.
*/
func (svc *Service) Open(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: Open
	err = svc.Sleep(ctx, svc.openDwell)
	if err != nil {
		return false, errors.Trace(err)
	}
	svc.mu.Lock()
	svc.completions = append(svc.completions, completion{tag: tag, at: time.Now()})
	svc.mu.Unlock()
	return true, nil
}

/*
SaturatedBand is a single-task graph routing through the throttled Bounded task.
*/
func (svc *Service) SaturatedBand(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: SaturatedBand
	graph = workflow.NewGraph(saturatedbandflowapi.SaturatedBand.URL())
	graph.AddTask("bounded", saturatedbandflowapi.Bounded.URL())
	graph.AddTransition("bounded", workflow.END)
	return graph, nil
}

/*
OpenBand is a single-task graph routing through the unrestricted Open task.
*/
func (svc *Service) OpenBand(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: OpenBand
	graph = workflow.NewGraph(saturatedbandflowapi.OpenBand.URL())
	graph.AddTask("open", saturatedbandflowapi.Open.URL())
	graph.AddTransition("open", workflow.END)
	return graph, nil
}
