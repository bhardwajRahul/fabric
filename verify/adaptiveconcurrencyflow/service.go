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

package adaptiveconcurrencyflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/throttle"

	"github.com/microbus-io/fabric/verify/adaptiveconcurrencyflow/adaptiveconcurrencyflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ adaptiveconcurrencyflowapi.Client
)

/*
Service implements adaptiveconcurrencyflow.verify, exercising the foreman's rate controller against
a rate-bounded backend. The task admits up to a configurable ops/sec and emits 429 above that.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu          sync.Mutex
	rate        int
	throttle    *throttle.Throttle
	completions int
	rejections  int
	dwell       time.Duration
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	if svc.rate == 0 {
		svc.rate = 7
	}
	svc.throttle = throttle.New(time.Second, svc.rate)
	if svc.dwell == 0 {
		svc.dwell = 10 * time.Millisecond
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// SetRate configures the task's per-second admission rate. Must be called before app.RunInTest.
func (svc *Service) SetRate(rate int) *Service {
	svc.rate = rate
	return svc
}

// SetDwell sets the per-invocation sleep duration.
func (svc *Service) SetDwell(d time.Duration) *Service {
	svc.dwell = d
	return svc
}

// Observed returns the running counters (completions, rejections).
func (svc *Service) Observed() (completions, rejections int) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.completions, svc.rejections
}

/*
Adaptive is gated by an internal sliding-window throttle. Returns 429 when the per-second budget is
exhausted; otherwise sleeps dwell, increments the completions counter, and returns.
*/
func (svc *Service) Adaptive(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: Adaptive
	_ = tag
	if admit, _ := svc.throttle.Allow(); !admit {
		svc.mu.Lock()
		svc.rejections++
		svc.mu.Unlock()
		return false, errors.New("saturated", http.StatusTooManyRequests)
	}
	if err = svc.Sleep(ctx, svc.dwell); err != nil {
		return false, errors.Trace(err)
	}
	svc.mu.Lock()
	svc.completions++
	svc.mu.Unlock()
	return true, nil
}

/*
AdaptiveConcurrency is a single-task graph routing through the rate-bounded Adaptive task.
*/
func (svc *Service) AdaptiveConcurrency(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: AdaptiveConcurrency
	graph = workflow.NewGraph(adaptiveconcurrencyflowapi.AdaptiveConcurrency.URL())
	graph.DeclareInputs("tag")
	graph.DeclareOutputs("tallied")
	graph.AddTask("adaptive", adaptiveconcurrencyflowapi.Adaptive.URL())
	graph.AddTransition("adaptive", workflow.END)
	return graph, nil
}
