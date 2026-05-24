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

package retryflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/retryflow/retryflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ retryflowapi.Client
)

/*
Service implements retryflow.verify, exercising flow.Retry.
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
TaskA passes the target through.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error) { // MARKER: TaskA
	return target, nil
}

/*
Flaky increments attempts; if attempts<target, calls flow.Retry up to 5 attempts.
*/
func (svc *Service) Flaky(ctx context.Context, flow *workflow.Flow, attempts, target int) (attemptsOut int, err error) { // MARKER: Flaky
	attempts++
	if attempts >= target {
		// Success on this attempt.
		return attempts, nil
	}
	// Want another attempt.
	if !flow.Retry(5, 0, 0, 0) {
		// Retry budget exhausted; surface the actual error so the flow fails.
		return attempts, errors.New("flaky exhausted retries at attempt %d", attempts)
	}
	return attempts, nil
}

/*
TaskB surfaces the final attempts count.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error) { // MARKER: TaskB
	return attempts, nil
}

/*
Retry defines the graph A -> Flaky -> B.
*/
func (svc *Service) Retry(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Retry
	graph = workflow.NewGraph(retryflowapi.Retry.URL())
	graph.AddTask("taskA", retryflowapi.TaskA.URL())
	graph.AddTask("flaky", retryflowapi.Flaky.URL())
	graph.AddTask("taskB", retryflowapi.TaskB.URL())
	graph.AddTransition("taskA", "flaky")
	graph.AddTransition("flaky", "taskB")
	graph.AddTransition("taskB", workflow.END)
	return graph, nil
}
