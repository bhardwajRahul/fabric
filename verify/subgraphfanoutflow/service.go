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

package subgraphfanoutflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/subgraphfanoutflow/subgraphfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ subgraphfanoutflowapi.Client
)

/*
Service implements subgraphfanoutflow.verify, exercising a subgraph as one branch of an outer fan-out.
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

// TaskA is the fan-out source.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	return true, nil
}

// NormalB produces resultB.
func (svc *Service) NormalB(ctx context.Context, flow *workflow.Flow) (resultB string, err error) { // MARKER: NormalB
	return "b", nil
}

// TaskX is the subgraph entry.
func (svc *Service) TaskX(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error) { // MARKER: TaskX
	return true, nil
}

// TaskY runs after TaskX in the subgraph.
func (svc *Service) TaskY(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error) { // MARKER: TaskY
	if xPassed {
		return "sub", nil
	}
	return "sub-no-x", nil
}

// NormalD produces resultD.
func (svc *Service) NormalD(ctx context.Context, flow *workflow.Flow) (resultD string, err error) { // MARKER: NormalD
	return "d", nil
}

// TaskE is the outer fan-in.
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow, resultB, subResult, resultD string) (finalResult string, err error) { // MARKER: TaskE
	return resultB + "/" + subResult + "/" + resultD, nil
}

// Sub defines the subgraph X -> Y.
func (svc *Service) Sub(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Sub
	graph = workflow.NewGraph(subgraphfanoutflowapi.Sub.URL())
	graph.AddTask("taskX", subgraphfanoutflowapi.TaskX.URL())
	graph.AddTask("taskY", subgraphfanoutflowapi.TaskY.URL())
	graph.AddTransition("taskX", "taskY")
	graph.AddTransition("taskY", workflow.END)
	return graph, nil
}

// SubFanOut defines A -> {NormalB, Sub, NormalD} -> E.
func (svc *Service) SubFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: SubFanOut
	graph = workflow.NewGraph(subgraphfanoutflowapi.SubFanOut.URL())
	graph.AddTask("taskA", subgraphfanoutflowapi.TaskA.URL())
	graph.AddTask("normalB", subgraphfanoutflowapi.NormalB.URL())
	graph.AddSubgraph("sub", subgraphfanoutflowapi.Sub.URL())
	graph.AddTask("normalD", subgraphfanoutflowapi.NormalD.URL())
	graph.AddTask("taskE", subgraphfanoutflowapi.TaskE.URL())
	graph.SetFanIn("taskE")
	graph.AddTransition("taskA", "normalB")
	graph.AddTransition("taskA", "sub")
	graph.AddTransition("taskA", "normalD")
	graph.AddTransition("normalB", "taskE")
	graph.AddTransition("sub", "taskE")
	graph.AddTransition("normalD", "taskE")
	graph.AddTransition("taskE", workflow.END)
	return graph, nil
}
