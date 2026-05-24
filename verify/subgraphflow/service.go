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

package subgraphflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/subgraphflow/subgraphflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ subgraphflowapi.Client
)

/*
Service implements subgraphflow.verify, exercising subgraph invocation.
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
TaskA passes the seed through.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, seed string) (seedOut string, err error) { // MARKER: TaskA
	return seed, nil
}

/*
TaskX is the subgraph entry. It reads `seed` and writes `innerStage`.
*/
func (svc *Service) TaskX(ctx context.Context, flow *workflow.Flow, seed string) (innerStage string, err error) { // MARKER: TaskX
	return "X(" + seed + ")", nil
}

/*
TaskY runs after TaskX in the subgraph. Reads `innerStage`, writes `innerResult`.
*/
func (svc *Service) TaskY(ctx context.Context, flow *workflow.Flow, innerStage string) (innerResult string, err error) { // MARKER: TaskY
	return "Y(" + innerStage + ")", nil
}

/*
TaskZ runs in the parent after the subgraph. Reads `innerResult` (merged in from the subgraph)
and produces a final result.
*/
func (svc *Service) TaskZ(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error) { // MARKER: TaskZ
	return "Z(" + innerResult + ")", nil
}

/*
Inner defines the subgraph X -> Y. It declares `seed` as input and `innerResult` as output;
the parent step's state crosses the boundary filtered through these.
*/
func (svc *Service) Inner(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Inner
	graph = workflow.NewGraph(subgraphflowapi.Inner.URL())
	graph.AddTask("taskX", subgraphflowapi.TaskX.URL())
	graph.AddTask("taskY", subgraphflowapi.TaskY.URL())
	graph.AddTransition("taskX", "taskY")
	graph.AddTransition("taskY", workflow.END)
	return graph, nil
}

/*
Parent defines the graph A -> [Inner subgraph] -> Z.
*/
func (svc *Service) Parent(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Parent
	graph = workflow.NewGraph(subgraphflowapi.Parent.URL())
	graph.AddTask("taskA", subgraphflowapi.TaskA.URL())
	graph.AddSubgraph("inner", subgraphflowapi.Inner.URL())
	graph.AddTask("taskZ", subgraphflowapi.TaskZ.URL())
	graph.AddTransition("taskA", "inner")
	graph.AddTransition("inner", "taskZ")
	graph.AddTransition("taskZ", workflow.END)
	return graph, nil
}
