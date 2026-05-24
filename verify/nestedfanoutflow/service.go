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

package nestedfanoutflow

import (
	"context"
	"fmt"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/nestedfanoutflow/nestedfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ nestedfanoutflowapi.Client
)

/*
Service implements nestedfanoutflow.verify, exercising two-level nested fan-out
via the subgraph escape hatch.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// TaskA is the outer fan-out source.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	return true, nil
}

// NormalB is the non-subgraph sibling of the outer fan-out.
func (svc *Service) NormalB(ctx context.Context, flow *workflow.Flow) (normalResult string, err error) { // MARKER: NormalB
	return "normal", nil
}

// TaskX is the inner subgraph entry.
func (svc *Service) TaskX(ctx context.Context, flow *workflow.Flow) (innerStarted bool, err error) { // MARKER: TaskX
	return true, nil
}

// TaskY contributes +10 to the inner sum reducer.
func (svc *Service) TaskY(ctx context.Context, flow *workflow.Flow) (sumInnerOut int, err error) { // MARKER: TaskY
	return 10, nil
}

// TaskZ contributes +20 to the inner sum reducer.
func (svc *Service) TaskZ(ctx context.Context, flow *workflow.Flow) (sumInnerOut int, err error) { // MARKER: TaskZ
	return 20, nil
}

// TaskW is the inner subgraph fan-in; reads the merged sumInner.
func (svc *Service) TaskW(ctx context.Context, flow *workflow.Flow, sumInner int) (innerResult int, err error) { // MARKER: TaskW
	return sumInner, nil
}

// TaskJ is the outer fan-in; combines NormalB's result with the inner subgraph's result.
func (svc *Service) TaskJ(ctx context.Context, flow *workflow.Flow, normalResult string, innerResult int) (finalResult string, err error) { // MARKER: TaskJ
	return fmt.Sprintf("%s/%d", normalResult, innerResult), nil
}

// Inner defines the subgraph X -> {Y, Z} -> W.
func (svc *Service) Inner(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Inner
	graph = workflow.NewGraph(nestedfanoutflowapi.Inner.URL())
	graph.AddTask("taskX", nestedfanoutflowapi.TaskX.URL())
	graph.AddTask("taskY", nestedfanoutflowapi.TaskY.URL())
	graph.AddTask("taskZ", nestedfanoutflowapi.TaskZ.URL())
	graph.AddTask("taskW", nestedfanoutflowapi.TaskW.URL())
	graph.SetFanIn("taskW")
	graph.AddTransition("taskX", "taskY")
	graph.AddTransition("taskX", "taskZ")
	graph.AddTransition("taskY", "taskW")
	graph.AddTransition("taskZ", "taskW")
	graph.AddTransition("taskW", workflow.END)
	return graph, nil
}

// Nested defines the outer graph A -> {NormalB, Inner subgraph} -> J.
func (svc *Service) Nested(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Nested
	graph = workflow.NewGraph(nestedfanoutflowapi.Nested.URL())
	graph.AddTask("taskA", nestedfanoutflowapi.TaskA.URL())
	graph.AddTask("normalB", nestedfanoutflowapi.NormalB.URL())
	graph.AddSubgraph("inner", nestedfanoutflowapi.Inner.URL())
	graph.AddTask("taskJ", nestedfanoutflowapi.TaskJ.URL())
	graph.SetFanIn("taskJ")
	graph.AddTransition("taskA", "normalB")
	graph.AddTransition("taskA", "inner")
	graph.AddTransition("normalB", "taskJ")
	graph.AddTransition("inner", "taskJ")
	graph.AddTransition("taskJ", workflow.END)
	return graph, nil
}
