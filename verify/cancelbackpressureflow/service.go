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

package cancelbackpressureflow

import (
	"context"
	"net/http"
	"sync"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/cancelbackpressureflow/cancelbackpressureflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ cancelbackpressureflowapi.Client
)

/*
Service implements cancelbackpressureflow.verify. The fixture races a flow Cancel against a task
that returns 429, verifying the handleBackpressure status guard - a 429 arriving after the step
has been cancelled must not resurrect it from cancelled back to pending.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	readyOnce sync.Once
	ready     chan struct{}
	release   chan struct{}
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	svc.ready = make(chan struct{})
	svc.release = make(chan struct{})
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Ready returns a channel that closes once BounceAndCancel enters its parked wait.
// The test blocks on this before issuing Cancel.
func (svc *Service) Ready() <-chan struct{} {
	return svc.ready
}

// Release wakes the parked BounceAndCancel invocation so it returns 429.
func (svc *Service) Release() {
	defer func() { _ = recover() }() // tolerate a duplicate close from a cleanup path
	close(svc.release)
}

/*
BounceAndCancel signals "running", parks until Release, then returns 429. The foreman dispatches
the 429 through handleBackpressure; the test coordinates so the step is already cancelled by then
and the bounce UPDATE's status='running' predicate must short-circuit.
*/
func (svc *Service) BounceAndCancel(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) { // MARKER: BounceAndCancel
	_ = tag
	svc.readyOnce.Do(func() { close(svc.ready) })
	select {
	case <-svc.release:
	case <-ctx.Done():
		return false, errors.Trace(ctx.Err())
	}
	return false, errors.New("saturated", http.StatusTooManyRequests)
}

/*
CancelBackpressure is a single-task graph that runs BounceAndCancel.
*/
func (svc *Service) CancelBackpressure(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: CancelBackpressure
	graph = workflow.NewGraph(cancelbackpressureflowapi.CancelBackpressure.URL())
	graph.AddTask("bounce-and-cancel", cancelbackpressureflowapi.BounceAndCancel.URL())
	graph.AddTransition("bounce-and-cancel", workflow.END)
	return graph, nil
}
