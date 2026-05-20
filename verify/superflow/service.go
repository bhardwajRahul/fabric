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

package superflow

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/superflow/superflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ superflowapi.Client
)

// taskNames enumerates every task in the Super and SuperSub graphs.
// The visits map is keyed by these names; tests assert on counts via Visits.
var taskNames = []string{
	"TaskA", "TaskB", "TaskC", "TaskD", "TaskE", "TaskZ", "ErrorHandler",
	"SubTaskA", "SubTaskB",
}

/*
Service implements superflow.verify, a single fixture whose Super graph
exercises every workflow transition primitive (sequential, forEach fan-out
with fan-in, OnError with sibling-cancel, conditional, subgraph call, and
goto). Per-task runtime behavior is injected through the workflow state
under the Behaviors map, so a single graph can express many test cases.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	visits map[string]*atomic.Int64
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	svc.visits = make(map[string]*atomic.Int64, len(taskNames))
	for _, name := range taskNames {
		svc.visits[name] = new(atomic.Int64)
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Visits returns the number of times the named task has been entered since the
// last ResetVisitCounters. Returns 0 for unknown task names.
func (svc *Service) Visits(name string) int64 {
	c, ok := svc.visits[name]
	if !ok {
		return 0
	}
	return c.Load()
}

// AllVisits returns a snapshot of every task's visit count.
func (svc *Service) AllVisits() map[string]int64 {
	out := make(map[string]int64, len(svc.visits))
	for name, c := range svc.visits {
		out[name] = c.Load()
	}
	return out
}

// ResetVisitCounters zeroes every counter. Call between subtests sharing one
// service instance.
func (svc *Service) ResetVisitCounters() {
	for _, c := range svc.visits {
		c.Store(0)
	}
}

// step is the common task body: count the visit, parse the shared state, and
// apply any behavior knobs configured for this task. Returns true if the caller
// should return early (interrupt / retry / fatal error path applied).
func (svc *Service) step(ctx context.Context, flow *workflow.Flow, name string) (early bool, err error) {
	svc.visits[name].Add(1)
	var st superflowapi.SuperflowState
	flow.ParseState(&st)
	b, ok := st.Behaviors[name]
	if !ok {
		return false, nil
	}
	if b.SleepMs > 0 {
		err = svc.Sleep(ctx, time.Duration(b.SleepMs)*time.Millisecond)
		if err != nil {
			return true, errors.Trace(err)
		}
	}
	if b.Retry {
		flow.RetryNow()
		return true, nil
	}
	if b.Interrupt {
		flow.Interrupt(map[string]any{"interruptedBy": name})
		return true, nil
	}
	if b.Goto != "" {
		flow.Goto(b.Goto)
	}
	if b.ErrorStatus != 0 {
		return true, errors.New("injected error from "+name, b.ErrorStatus)
	}
	return false, nil
}

// TaskA is the entry of the Super graph.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskA
	_, err = svc.step(ctx, flow, "TaskA")
	return err
}

// TaskB is the forEach source; its sole outgoing transition iterates items.
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskB
	_, err = svc.step(ctx, flow, "TaskB")
	return err
}

// TaskC runs once per forEach element and converges into TaskD's fan-in.
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskC
	_, err = svc.step(ctx, flow, "TaskC")
	return err
}

// TaskD is the forEach fan-in target. Its conditional outgoing transitions
// either invoke the SuperSub subgraph or jump straight to TaskE.
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskD
	_, err = svc.step(ctx, flow, "TaskD")
	return err
}

// TaskE is the conditional fan-in target. Supports a goto to TaskZ.
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskE
	_, err = svc.step(ctx, flow, "TaskE")
	return err
}

// TaskZ is the goto target out of TaskE.
func (svc *Service) TaskZ(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: TaskZ
	_, err = svc.step(ctx, flow, "TaskZ")
	return err
}

// ErrorHandler receives flows whose TaskC raised an error and rejoins the
// fan-in at TaskD so the workflow can still complete.
func (svc *Service) ErrorHandler(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: ErrorHandler
	_, err = svc.step(ctx, flow, "ErrorHandler")
	return err
}

// SubTaskA is the entry of the SuperSub subgraph.
func (svc *Service) SubTaskA(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: SubTaskA
	_, err = svc.step(ctx, flow, "SubTaskA")
	return err
}

// SubTaskB is the second step of the SuperSub subgraph.
func (svc *Service) SubTaskB(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: SubTaskB
	_, err = svc.step(ctx, flow, "SubTaskB")
	return err
}

/*
Super defines the unified verification graph. Coverage:
- sequential transitions (A -> B)
- dynamic fan-out via forEach (B -> C per items[i])
- OnError + sibling-cancel (C -> ErrorHandler)
- explicit fan-in (TaskD)
- conditional fan-out (TaskD's when branches)
- subgraph call (TaskD -> SuperSubCall when useSubgraph)
- conditional fan-in (TaskE)
- withGoto (TaskE -> TaskZ when a prior task issued flow.Goto)
*/
func (svc *Service) Super(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Super
	graph = workflow.NewGraph(superflowapi.Super.URL())
	graph.DeclareInputs("items", "useSubgraph", "behaviors")
	graph.DeclareOutputs()
	graph.AddTask("taskA", superflowapi.TaskA.URL())
	graph.AddTask("taskB", superflowapi.TaskB.URL())
	graph.AddTask("taskC", superflowapi.TaskC.URL())
	graph.AddTask("taskD", superflowapi.TaskD.URL())
	graph.AddTask("taskE", superflowapi.TaskE.URL())
	graph.AddTask("taskZ", superflowapi.TaskZ.URL())
	graph.AddTask("errorHandler", superflowapi.ErrorHandler.URL())
	graph.AddSubgraph("superSubCall", superflowapi.SuperSub.URL())

	// Fan-in points: TaskD for the forEach cohort, TaskE for the conditional.
	graph.SetFanIn("taskD")
	graph.SetFanIn("taskE")

	// Sequential head
	graph.AddTransition("taskA", "taskB")

	// Dynamic fan-out via forEach
	graph.AddTransitionForEach("taskB", "taskC", "items", "item")

	// Fan-in into TaskD, with OnError side-route through ErrorHandler
	graph.AddTransition("taskC", "taskD")
	graph.AddTransitionOnError("taskC", "errorHandler")
	graph.AddTransition("errorHandler", "taskD")

	// Conditional fan-out at TaskD, converging at TaskE
	graph.AddTransitionWhen("taskD", "superSubCall", "useSubgraph == true")
	graph.AddTransitionWhen("taskD", "taskE", "useSubgraph != true")
	graph.AddTransition("superSubCall", "taskE")

	// Tail with optional goto
	graph.AddTransitionGoto("taskE", "taskZ")
	graph.AddTransition("taskE", workflow.END)
	graph.AddTransition("taskZ", workflow.END)
	return graph, nil
}

// SuperSub is the nested subgraph: a minimal two-task sequence.
func (svc *Service) SuperSub(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: SuperSub
	graph = workflow.NewGraph(superflowapi.SuperSub.URL())
	graph.DeclareInputs("behaviors")
	graph.DeclareOutputs()
	graph.AddTask("subTaskA", superflowapi.SubTaskA.URL())
	graph.AddTask("subTaskB", superflowapi.SubTaskB.URL())
	graph.AddTransition("subTaskA", "subTaskB")
	graph.AddTransition("subTaskB", workflow.END)
	return graph, nil
}
