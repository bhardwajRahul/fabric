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

package fairnessflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/fairnessflow/fairnessflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ fairnessflowapi.Client
)

/*
Service implements fairnessflow.verify, exercising two-level weighted fairness across fairness keys.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu    sync.Mutex
	order []string
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Order returns the tags in the order their flows were dispatched, oldest first.
func (svc *Service) Order() []string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return append([]string(nil), svc.order...)
}

/*
Tally sleeps for the requested delay, then appends its key tag to the dispatch order. The delay forces
saturation so the order in which a single worker picks up flows reflects the foreman's weighted
fairness selection across keys, not arrival timing.
*/
func (svc *Service) Tally(ctx context.Context, flow *workflow.Flow) (tallied bool, err error) { // MARKER: Tally
	var in fairnessflowapi.TallyIn
	flow.ParseState(&in)
	if in.DelayMs > 0 {
		err = svc.Sleep(ctx, time.Duration(in.DelayMs)*time.Millisecond)
		if err != nil {
			return false, errors.Trace(err)
		}
	}
	svc.mu.Lock()
	svc.order = append(svc.order, in.Tag)
	svc.mu.Unlock()
	return true, nil
}

/*
Fairness defines the single-task graph (tally -> END) used to observe weighted dispatch share across
fairness keys. The graph carries no timing; the fairness key and weight are per-flow options at Create.
*/
func (svc *Service) Fairness(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Fairness
	graph = workflow.NewGraph(fairnessflowapi.Fairness.URL())
	graph.DeclareInputs("tag", "delayMs")
	graph.DeclareOutputs("tallied")
	graph.AddTask("tally", fairnessflowapi.Tally.URL())
	graph.AddTransition("tally", workflow.END)
	return graph, nil
}
