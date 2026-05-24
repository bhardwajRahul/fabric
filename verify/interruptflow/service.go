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

package interruptflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/interruptflow/interruptflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ interruptflowapi.Client
)

/*
Service implements interruptflow.verify, exercising flow.Interrupt + Resume.
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
TaskA passes the prompt through to state.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, prompt string) (promptOut string, err error) { // MARKER: TaskA
	return prompt, nil
}

/*
AwaitInput interrupts the flow until userInput is provided via Resume. On the
re-execution after Resume, userInput is in state and the task falls through.
*/
func (svc *Service) AwaitInput(ctx context.Context, flow *workflow.Flow, userInput string) (userInputOut string, err error) { // MARKER: AwaitInput
	if userInput == "" {
		flow.Interrupt(map[string]any{"requestedInput": "userInput"})
		return "", nil
	}
	return userInput, nil
}

/*
Compose joins prompt and userInput into the final result.
*/
func (svc *Service) Compose(ctx context.Context, flow *workflow.Flow, prompt, userInput string) (result string, err error) { // MARKER: Compose
	return prompt + ", " + userInput, nil
}

/*
Interruptor defines the graph A -> AwaitInput -> Compose.
*/
func (svc *Service) Interruptor(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Interruptor
	graph = workflow.NewGraph(interruptflowapi.Interruptor.URL())
	graph.AddTask("taskA", interruptflowapi.TaskA.URL())
	graph.AddTask("awaitInput", interruptflowapi.AwaitInput.URL())
	graph.AddTask("compose", interruptflowapi.Compose.URL())
	graph.AddTransition("taskA", "awaitInput")
	graph.AddTransition("awaitInput", "compose")
	graph.AddTransition("compose", workflow.END)
	return graph, nil
}
