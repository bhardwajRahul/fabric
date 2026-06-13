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

package onerrorsiblingsflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/onerrorsiblingsflow/onerrorsiblingsflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ onerrorsiblingsflowapi.Client
)

/*
Service implements onerrorsiblingsflow.verify, exercising OnError + fan-out.
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
TaskA is the fan-out source. Marks the flow as started.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	return true, nil
}

/*
TaskB always errors, triggering its onError transition.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, started bool) (markB bool, err error) { // MARKER: TaskB
	return false, errors.New("triggered failure in TaskB")
}

/*
TaskC is a normal sibling that runs in parallel with TaskB.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, started bool) (markC bool, err error) { // MARKER: TaskC
	return started, nil
}

/*
TaskD is a normal sibling that runs in parallel with TaskB.
*/
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow, started bool) (markD bool, err error) { // MARKER: TaskD
	return started, nil
}

/*
Handler handles TaskB's error and sets handled=true.
*/
func (svc *Service) Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (handled bool, err error) { // MARKER: Handler
	return onErr != nil, nil
}

/*
TaskE is the fan-in target. It surfaces two booleans:
  - recovered: the handler ran (handled=true) and TaskB never set markB.
  - siblingsRan: TaskC and TaskD both ran to completion (not cancelled).
*/
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow, handled, markB, markC, markD bool) (recovered, siblingsRan bool, err error) { // MARKER: TaskE
	return handled && !markB, markC && markD, nil
}

/*
FanOutError defines the graph A -> {B, C, D} -> E with B onError -> Handler -> E.
*/
func (svc *Service) FanOutError(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: FanOutError
	graph = workflow.NewGraph(onerrorsiblingsflowapi.FanOutError.URL())
	graph.AddTask("taskA", onerrorsiblingsflowapi.TaskA.URL())
	graph.AddTask("taskB", onerrorsiblingsflowapi.TaskB.URL())
	graph.AddTask("taskC", onerrorsiblingsflowapi.TaskC.URL())
	graph.AddTask("taskD", onerrorsiblingsflowapi.TaskD.URL())
	graph.AddTask("handler", onerrorsiblingsflowapi.Handler.URL())
	graph.AddTask("taskE", onerrorsiblingsflowapi.TaskE.URL())
	graph.SetFanIn("taskE")
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskA", "taskC")
	graph.AddTransition("taskA", "taskD")
	graph.AddTransitionOnError("taskB", "handler")
	graph.AddTransition("taskB", "taskE")
	graph.AddTransition("taskC", "taskE")
	graph.AddTransition("taskD", "taskE")
	graph.AddTransition("handler", "taskE")
	graph.AddTransition("taskE", workflow.END)
	return graph, nil
}
