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

package distributedbackpressureflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/distributedbackpressureflow/distributedbackpressureflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ distributedbackpressureflowapi.Client
)

/*
Service implements distributedbackpressureflow.verify, exercising adaptive concurrency under
multi-replica and multi-shard deployment.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu          sync.Mutex
	cap         int
	inFlight    int
	maxObserved int
	rejections  int
	completions int
	dwell       time.Duration
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	if svc.cap == 0 {
		svc.cap = 4
	}
	if svc.dwell == 0 {
		svc.dwell = 50 * time.Millisecond
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// SetCap configures the task's self-limit.
func (svc *Service) SetCap(cap int) *Service {
	svc.cap = cap
	return svc
}

// SetDwell configures the task's per-execution sleep.
func (svc *Service) SetDwell(d time.Duration) *Service {
	svc.dwell = d
	return svc
}

// Observed returns (peak in-flight, total rejections, total completions).
func (svc *Service) Observed() (peak int, rejections int, completions int) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.maxObserved, svc.rejections, svc.completions
}

/*
Bounded self-emits http.StatusTooManyRequests above its cap. Both replicas of this fixture share the
same Service singleton (registered once in the test app), so the cap is genuinely cluster-wide.
*/
func (svc *Service) Bounded(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: Bounded
	_ = tag
	svc.mu.Lock()
	svc.inFlight++
	if svc.inFlight > svc.maxObserved {
		svc.maxObserved = svc.inFlight
	}
	if svc.inFlight > svc.cap {
		svc.inFlight--
		svc.rejections++
		svc.mu.Unlock()
		return false, errors.New("saturated", http.StatusTooManyRequests)
	}
	svc.mu.Unlock()

	err = svc.Sleep(ctx, svc.dwell)

	svc.mu.Lock()
	svc.inFlight--
	svc.completions++
	svc.mu.Unlock()
	if err != nil {
		return false, errors.Trace(err)
	}
	return true, nil
}

/*
DistributedBackpressure is a single-task graph routing through Bounded.
*/
func (svc *Service) DistributedBackpressure(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: DistributedBackpressure
	graph = workflow.NewGraph(distributedbackpressureflowapi.DistributedBackpressure.URL())
	graph.DeclareInputs("tag")
	graph.DeclareOutputs("tallied")
	graph.AddTask("bounded", distributedbackpressureflowapi.Bounded.URL())
	graph.AddTransition("bounded", workflow.END)
	return graph, nil
}
