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

package cancelledfanoutflow

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/cancelledfanoutflow/cancelledfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ cancelledfanoutflowapi.Client
)

// branchSleep is how long each fan-out branch task blocks. The test cancels the
// flow at half this duration so exactly one branch is mid-execution at cancel time.
const branchSleep = 2 * time.Second

/*
Service implements cancelledfanoutflow.verify, which exercises cancelling a flow
mid-fan-out with the foreman pinned to a single worker.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// executed counts how many branch tasks (A/B/C) actually entered their body.
	// The test asserts this is exactly 1: with one worker the lone worker is blocked
	// in the first branch's sleep across the cancel, so the other two never start.
	executed atomic.Int32
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Executed returns the number of branch tasks that entered execution.
func (svc *Service) Executed() int {
	return int(svc.executed.Load())
}

/*
Source is the static fan-out source. It returns instantly so the foreman fans
out to A, B and C without consuming the worker.
*/
func (svc *Service) Source(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: Source
	return true, nil
}

// branch is the shared body of A, B and C: count the entry, then block for
// branchSleep so a mid-flight cancel can take effect while it runs.
func (svc *Service) branch(ctx context.Context) (sumExecutedOut int, err error) {
	svc.executed.Add(1)
	select {
	case <-time.After(branchSleep):
	case <-ctx.Done():
	}
	return 1, nil
}

/*
A is a fan-out branch. It records its execution and sleeps before contributing 1.
*/
func (svc *Service) A(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: A
	return svc.branch(ctx)
}

/*
B is a fan-out branch. It records its execution and sleeps before contributing 1.
*/
func (svc *Service) B(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: B
	return svc.branch(ctx)
}

/*
C is a fan-out branch. It records its execution and sleeps before contributing 1.
*/
func (svc *Service) C(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: C
	return svc.branch(ctx)
}

/*
J is the fan-in target. It surfaces the summed sumExecuted. In the cancel
scenario the flow is cancelled before fan-in, so J never runs.
*/
func (svc *Service) J(ctx context.Context, flow *workflow.Flow, sumExecuted int) (totalExecuted int, err error) { // MARKER: J
	return sumExecuted, nil
}

/*
CancelledFanOut defines the graph: Source -> {A, B, C} -> J. J is the explicit
fan-in over the sum* reducer field sumExecuted.
*/
func (svc *Service) CancelledFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: CancelledFanOut
	graph = workflow.NewGraph(cancelledfanoutflowapi.CancelledFanOut.URL())
	graph.AddTask("source", cancelledfanoutflowapi.Source.URL())
	graph.AddTask("a", cancelledfanoutflowapi.A.URL())
	graph.AddTask("b", cancelledfanoutflowapi.B.URL())
	graph.AddTask("c", cancelledfanoutflowapi.C.URL())
	graph.AddTask("j", cancelledfanoutflowapi.J.URL())
	graph.SetFanIn("j")
	graph.AddTransition("source", "a")
	graph.AddTransition("source", "b")
	graph.AddTransition("source", "c")
	graph.AddTransition("a", "j")
	graph.AddTransition("b", "j")
	graph.AddTransition("c", "j")
	graph.AddTransition("j", workflow.END)
	return graph, nil
}
