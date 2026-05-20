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

package maxconcurrencyflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/maxconcurrencyflow/maxconcurrencyflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ maxconcurrencyflowapi.Client
)

/*
Service implements maxconcurrencyflow.verify, exercising adaptive per-task concurrency control via
a task that self-emits 429 above a configured cap.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu          sync.Mutex
	cap         int  // task self-limit; above this the task returns 429
	inFlight    int  // current concurrent executions of Bounded
	maxObserved int  // peak in-flight ever seen across the run
	rejections  int  // count of 429s the task itself emitted
	dwell       time.Duration
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	if svc.cap == 0 {
		svc.cap = 3 // default cap; tests override via SetCap before app.RunInTest
	}
	if svc.dwell == 0 {
		svc.dwell = 100 * time.Millisecond
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// SetCap configures the task's self-limit. Above this concurrent count, Bounded returns 429.
// Must be called before app.RunInTest. Returns the service for chaining inside Init.
func (svc *Service) SetCap(cap int) *Service {
	svc.cap = cap
	return svc
}

// SetDwell configures how long Bounded sleeps while in flight (simulates work).
// Longer dwell increases the odds of concurrent dispatches overlapping.
func (svc *Service) SetDwell(d time.Duration) *Service {
	svc.dwell = d
	return svc
}

// Observed returns (peak in-flight observed, total 429 rejections emitted).
func (svc *Service) Observed() (peak int, rejections int) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.maxObserved, svc.rejections
}

/*
Bounded admits up to cap concurrent in-flight executions. Above that it returns
http.StatusTooManyRequests so the foreman observes a 429 and exercises its
backpressure path. While admitted, the task sleeps to overlap with peers.
*/
func (svc *Service) Bounded(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: Bounded
	_ = tag
	svc.mu.Lock()
	svc.inFlight++
	now := svc.inFlight
	if now > svc.maxObserved {
		svc.maxObserved = now
	}
	if now > svc.cap {
		svc.inFlight--
		svc.rejections++
		svc.mu.Unlock()
		return false, errors.New("saturated", http.StatusTooManyRequests)
	}
	svc.mu.Unlock()

	err = svc.Sleep(ctx, svc.dwell)

	svc.mu.Lock()
	svc.inFlight--
	svc.mu.Unlock()
	if err != nil {
		return false, errors.Trace(err)
	}
	return true, nil
}

/*
MaxConcurrency defines the single-task graph (bounded -> END) used to drive the foreman through
the backpressure path.
*/
func (svc *Service) MaxConcurrency(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: MaxConcurrency
	graph = workflow.NewGraph(maxconcurrencyflowapi.MaxConcurrency.URL())
	graph.DeclareInputs("tag")
	graph.DeclareOutputs("tallied")
	graph.AddTask("bounded", maxconcurrencyflowapi.Bounded.URL())
	graph.AddTransition("bounded", workflow.END)
	return graph, nil
}
