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

package nestedfailfanoutflow

import (
	"context"
	"net/http"
	"sync"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/nestedfailfanoutflow/nestedfailfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ nestedfailfanoutflowapi.Client
)

/*
Service implements nestedfailfanoutflow.verify, a 3x3 nested forEach where one inner branch
fails. Verifies that the other 8 inner branches still execute to completion and that the flow
transitions to failed only after every inner has terminated.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu             sync.Mutex
	innerStarts    int
	innerCompleted int
	joinIRuns      int
	joinORuns      int
	gateInnerSlow  chan struct{}
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	svc.gateInnerSlow = make(chan struct{})
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// ReleaseSlowInners closes the inner gate so the 8 non-failing inner tasks can complete.
// Called by the test after observing all 9 inner tasks have started.
func (svc *Service) ReleaseSlowInners() {
	close(svc.gateInnerSlow)
}

// Counters returns the current execution counters under the service lock.
func (svc *Service) Counters() (innerStarts, innerCompleted, joinIRuns, joinORuns int) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.innerStarts, svc.innerCompleted, svc.joinIRuns, svc.joinORuns
}

// TaskA is the entry that emits the outer forEach array.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (outers []int, err error) { // MARKER: TaskA
	return []int{0, 1, 2}, nil
}

// TaskO is the per-outer-branch task. It emits the inner forEach array and stamps the outer
// index onto state so each inner branch can identify its outer.
func (svc *Service) TaskO(ctx context.Context, flow *workflow.Flow, outerItem int) (inners []int, currentOuter int, err error) { // MARKER: TaskO
	return []int{0, 1, 2}, outerItem, nil
}

// TaskI is the per-(outer, inner) cell task. The cell (outer=1, inner=1) fails synchronously;
// every other cell blocks on the test gate so all 9 cells are demonstrably in flight at once.
func (svc *Service) TaskI(ctx context.Context, flow *workflow.Flow, currentOuter int, innerItem int) (err error) { // MARKER: TaskI
	svc.mu.Lock()
	svc.innerStarts++
	svc.mu.Unlock()

	if currentOuter == 1 && innerItem == 1 {
		return errors.New("simulated failure at outer=1 inner=1")
	}

	select {
	case <-svc.gateInnerSlow:
	case <-ctx.Done():
		return errors.Trace(ctx.Err())
	}

	svc.mu.Lock()
	svc.innerCompleted++
	svc.mu.Unlock()
	return nil
}

// JoinI is the inner cohort fan-in. Fires only for outer branches whose inner cohort had no
// failures - i.e. outer=0 and outer=2. The failed inner cohort (outer=1) never reaches this
// step because its failure is propagated to the outer cohort directly.
func (svc *Service) JoinI(ctx context.Context, flow *workflow.Flow) (err error) { // MARKER: JoinI
	svc.mu.Lock()
	svc.joinIRuns++
	svc.mu.Unlock()
	return nil
}

// JoinO is the outer cohort fan-in. Never fires in this fixture because the outer cohort always
// has a failed branch (outer=1, whose inner cohort had a failure that propagated up).
func (svc *Service) JoinO(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: JoinO
	svc.mu.Lock()
	svc.joinORuns++
	svc.mu.Unlock()
	return true, nil
}

// Nested defines the 3x3 nested forEach graph:
//
//	taskA --forEach(outers)--> taskO --forEach(inners)--> taskI --> joinI --> joinO --> END
//
// Both joinI and joinO are SetFanIn nodes (inner and outer cohort fan-ins respectively).
func (svc *Service) Nested(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Nested
	graph = workflow.NewGraph(nestedfailfanoutflowapi.Nested.URL())
	graph.AddTask("taskA", nestedfailfanoutflowapi.TaskA.URL())
	graph.AddTask("taskO", nestedfailfanoutflowapi.TaskO.URL())
	graph.AddTask("taskI", nestedfailfanoutflowapi.TaskI.URL())
	graph.AddTask("joinI", nestedfailfanoutflowapi.JoinI.URL())
	graph.AddTask("joinO", nestedfailfanoutflowapi.JoinO.URL())
	graph.SetFanIn("joinI")
	graph.SetFanIn("joinO")
	graph.AddTransitionForEach("taskA", "taskO", "outers", "outerItem")
	graph.AddTransitionForEach("taskO", "taskI", "inners", "innerItem")
	graph.AddTransition("taskI", "joinI")
	graph.AddTransition("joinI", "joinO")
	graph.AddTransition("joinO", workflow.END)
	return graph, nil
}
