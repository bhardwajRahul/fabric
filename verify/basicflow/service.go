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

package basicflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/basicflow/basicflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ basicflowapi.Client
)

/*
Service implements basicflow.verify which exercises the simplest sequential workflow shape.
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
TaskA writes "A" to the path.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (path string, err error) { // MARKER: TaskA
	return "A", nil
}

/*
TaskB appends "B" to the path.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskB
	return path + "B", nil
}

/*
TaskC appends "C" to the path.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskC
	return path + "C", nil
}

/*
Basic defines the workflow graph for the sequential A -> B -> C chain.
*/
func (svc *Service) Basic(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Basic
	graph = workflow.NewGraph(basicflowapi.Basic.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("path")
	graph.AddTask("taskA", basicflowapi.TaskA.URL())
	graph.AddTask("taskB", basicflowapi.TaskB.URL())
	graph.AddTask("taskC", basicflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
