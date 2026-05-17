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

package dynamicsubgraphflow

import (
	"context"
	"fmt"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/dynamicsubgraphflow/dynamicsubgraphflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ dynamicsubgraphflowapi.Client
)

/*
Service implements dynamicsubgraphflow.verify, exercising the flow.Subgraph control signal.
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

/*
Parent calls flow.Subgraph on its first invocation, then re-runs after the child completes
with the child's outputs (innerDone, innerResult) merged into state.
*/
func (svc *Service) Parent(ctx context.Context, flow *workflow.Flow, value int, innerDone bool, innerResult int) (parentResult string, err error) { // MARKER: Parent
	if !innerDone {
		flow.Subgraph(dynamicsubgraphflowapi.Inner.URL(), map[string]any{"value": value})
		return "", nil
	}
	return fmt.Sprintf("parent:%d", innerResult), nil
}

// InnerA doubles value.
func (svc *Service) InnerA(ctx context.Context, flow *workflow.Flow, value int) (innerStage int, err error) { // MARKER: InnerA
	return value * 2, nil
}

// InnerB adds 3 to innerStage and marks the inner subgraph done.
func (svc *Service) InnerB(ctx context.Context, flow *workflow.Flow, innerStage int) (innerResult int, innerDone bool, err error) { // MARKER: InnerB
	return innerStage + 3, true, nil
}

// Inner defines the child subgraph InnerA -> InnerB.
func (svc *Service) Inner(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Inner
	graph = workflow.NewGraph(dynamicsubgraphflowapi.Inner.URL())
	graph.DeclareInputs("value")
	graph.DeclareOutputs("innerResult", "innerDone")
	graph.AddTask("innerA", dynamicsubgraphflowapi.InnerA.URL())
	graph.AddTask("innerB", dynamicsubgraphflowapi.InnerB.URL())
	graph.AddTransition("innerA", "innerB")
	graph.AddTransition("innerB", workflow.END)
	return graph, nil
}

// DynamicSubgraph defines a single-task parent graph that dynamically invokes Inner.
func (svc *Service) DynamicSubgraph(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: DynamicSubgraph
	graph = workflow.NewGraph(dynamicsubgraphflowapi.DynamicSubgraph.URL())
	graph.DeclareInputs("value")
	graph.DeclareOutputs("parentResult")
	graph.AddTask("parent", dynamicsubgraphflowapi.Parent.URL())
	graph.AddTransition("parent", workflow.END)
	return graph, nil
}
