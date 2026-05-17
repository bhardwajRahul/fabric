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

package retryloopflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/retryloopflow/retryloopflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ retryloopflowapi.Client
)

/*
Service implements retryloopflow.verify, the SKIP-marked OnError retry loop pattern.
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

// TaskA passes target through.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error) { // MARKER: TaskA
	return target, nil
}

// TaskB errors if attempts<target, otherwise succeeds.
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, attempts, target int) (succeeded bool, err error) { // MARKER: TaskB
	if attempts < target {
		return false, errors.New("not enough attempts: %d/%d", attempts, target)
	}
	return true, nil
}

// Handler increments attempts and routes back to B via the normal transition.
func (svc *Service) Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError, attempts int) (attemptsOut int, err error) { // MARKER: Handler
	return attempts + 1, nil
}

// TaskC surfaces the final attempts count.
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error) { // MARKER: TaskC
	return attempts, nil
}

/*
RetryLoop defines A -> B -> C with B onError -> Handler -> B (back-edge cycle).
*/
func (svc *Service) RetryLoop(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: RetryLoop
	graph = workflow.NewGraph(retryloopflowapi.RetryLoop.URL())
	graph.DeclareInputs("target")
	graph.DeclareOutputs("finalAttempts")
	graph.AddTask("taskA", retryloopflowapi.TaskA.URL())
	graph.AddTask("taskB", retryloopflowapi.TaskB.URL())
	graph.AddTask("handler", retryloopflowapi.Handler.URL())
	graph.AddTask("taskC", retryloopflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransitionOnError("taskB", "handler")
	graph.AddTransition("handler", "taskB")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
