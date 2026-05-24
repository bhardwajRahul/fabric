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

package errorflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/errorflow/errorflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ errorflowapi.Client
)

/*
Service implements errorflow.verify which exercises onError routing.
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
TaskA passes the trigger through to state.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error) { // MARKER: TaskA
	return trigger, nil
}

/*
TaskB returns "ok" normally, or an error when the trigger is "fail".
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error) { // MARKER: TaskB
	if trigger == "fail" {
		return "", errors.New("triggered failure")
	}
	return "normal", nil
}

/*
Handler runs when TaskB errors. It records the error and produces a recovery result.
*/
func (svc *Service) Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error) { // MARKER: Handler
	if onErr == nil {
		return "recovered:no-error", nil
	}
	return "recovered:" + onErr.Error(), nil
}

/*
TaskC consumes whichever result reached it.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error) { // MARKER: TaskC
	return "final:" + result, nil
}

/*
Error defines the graph A -> B -> C with B onError -> Handler -> C.
*/
func (svc *Service) Error(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Error
	graph = workflow.NewGraph(errorflowapi.Error.URL())
	graph.AddTask("taskA", errorflowapi.TaskA.URL())
	graph.AddTask("taskB", errorflowapi.TaskB.URL())
	graph.AddTask("handler", errorflowapi.Handler.URL())
	graph.AddTask("taskC", errorflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransitionOnError("taskB", "handler")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("handler", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
