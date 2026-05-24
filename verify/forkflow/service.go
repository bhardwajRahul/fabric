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

package forkflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/forkflow/forkflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ forkflowapi.Client
)

/*
Service implements forkflow.verify, exercising the Fork API.
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

// TaskA passes value through.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) { // MARKER: TaskA
	return value, nil
}

// TaskB doubles value.
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) { // MARKER: TaskB
	return value * 2, nil
}

// TaskC adds 1 to value.
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, value int) (valueOut int, err error) { // MARKER: TaskC
	return value + 1, nil
}

// Pipe defines A -> B -> C.
func (svc *Service) Pipe(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Pipe
	graph = workflow.NewGraph(forkflowapi.Pipe.URL())
	graph.AddTask("taskA", forkflowapi.TaskA.URL())
	graph.AddTask("taskB", forkflowapi.TaskB.URL())
	graph.AddTask("taskC", forkflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
