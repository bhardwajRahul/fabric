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

package dynamicfanoutflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/dynamicfanoutflow/dynamicfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ dynamicfanoutflowapi.Client
)

/*
Service implements dynamicfanoutflow.verify which exercises forEach dynamic fan-out.
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
TaskA is the forEach source. It echoes the input items list back to state.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) { // MARKER: TaskA
	return items, nil
}

/*
TaskB runs once per element. Returns 1, contributing to sumProcessed via the sum* reducer.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error) { // MARKER: TaskB
	if item == "" {
		return 0, nil
	}
	return 1, nil
}

/*
TaskC is the fan-in target. It surfaces the final sumProcessed as processedCount.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error) { // MARKER: TaskC
	return sumProcessed, nil
}

/*
DynamicFanOut defines the graph: A -> forEach(items) -> B -> C.
*/
func (svc *Service) DynamicFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: DynamicFanOut
	graph = workflow.NewGraph(dynamicfanoutflowapi.DynamicFanOut.URL())
	graph.DeclareInputs("items")
	graph.DeclareOutputs("processedCount")
	graph.AddTask("taskA", dynamicfanoutflowapi.TaskA.URL())
	graph.AddTask("taskB", dynamicfanoutflowapi.TaskB.URL())
	graph.AddTask("taskC", dynamicfanoutflowapi.TaskC.URL())
	graph.SetFanIn("taskC")
	graph.AddTransitionForEach("taskA", "taskB", "items", "item")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
