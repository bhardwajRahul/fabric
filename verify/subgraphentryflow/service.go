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

package subgraphentryflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/subgraphentryflow/subgraphentryflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ subgraphentryflowapi.Client
)

/*
Service implements subgraphentryflow.verify, exercising a subgraph as both the first and last node of a workflow graph.
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

// TaskInner is the sole task of the Inner subgraph.
func (svc *Service) TaskInner(ctx context.Context, flow *workflow.Flow) (innerResult string, err error) { // MARKER: TaskInner
	return "inner", nil
}

// TaskTail is the sole task of the Tail subgraph; it reads the upstream subgraph's output.
func (svc *Service) TaskTail(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error) { // MARKER: TaskTail
	return innerResult + "/tail", nil
}

// RunInner invokes the Inner subgraph via flow.Subgraph and adopts its innerResult. It is the entry
// point of the Outer graph, so a subgraph call sits at the entry-point position.
func (svc *Service) RunInner(ctx context.Context, flow *workflow.Flow) (innerResult string, err error) { // MARKER: RunInner
	out, yield, err := flow.Subgraph(subgraphentryflowapi.Inner.URL(), nil)
	if err != nil {
		return "", errors.Trace(err)
	}
	if yield {
		return "", nil
	}
	if v, ok := out["innerResult"].(string); ok {
		return v, nil
	}
	return "", nil
}

// RunTail invokes the Tail subgraph via flow.Subgraph and adopts its finalResult. The upstream
// innerResult is passed explicitly so the child task can read it.
func (svc *Service) RunTail(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error) { // MARKER: RunTail
	out, yield, err := flow.Subgraph(subgraphentryflowapi.Tail.URL(), map[string]any{"innerResult": innerResult})
	if err != nil {
		return "", errors.Trace(err)
	}
	if yield {
		return "", nil
	}
	if v, ok := out["finalResult"].(string); ok {
		return v, nil
	}
	return "", nil
}

// Inner defines the single-task subgraph taskInner -> END.
func (svc *Service) Inner(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Inner
	graph = workflow.NewGraph(subgraphentryflowapi.Inner.URL())
	graph.AddTask("taskInner", subgraphentryflowapi.TaskInner.URL())
	graph.AddTransition("taskInner", workflow.END)
	return graph, nil
}

// Tail defines the single-task subgraph taskTail -> END.
func (svc *Service) Tail(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Tail
	graph = workflow.NewGraph(subgraphentryflowapi.Tail.URL())
	graph.AddTask("taskTail", subgraphentryflowapi.TaskTail.URL())
	graph.AddTransition("taskTail", workflow.END)
	return graph, nil
}

// Outer defines the coordinator-shape graph inner -> tail -> END, where both nodes are subgraphs.
func (svc *Service) Outer(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Outer
	graph = workflow.NewGraph(subgraphentryflowapi.Outer.URL())
	graph.AddTask("runInner", subgraphentryflowapi.RunInner.URL())
	graph.AddTask("runTail", subgraphentryflowapi.RunTail.URL())
	graph.AddTransition("runInner", "runTail")
	graph.AddTransition("runTail", workflow.END)
	return graph, nil
}
