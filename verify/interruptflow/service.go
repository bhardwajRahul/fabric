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
	graph.DeclareInputs("prompt")
	graph.DeclareOutputs("result")
	graph.AddTask("taskA", interruptflowapi.TaskA.URL())
	graph.AddTask("awaitInput", interruptflowapi.AwaitInput.URL())
	graph.AddTask("compose", interruptflowapi.Compose.URL())
	graph.AddTransition("taskA", "awaitInput")
	graph.AddTransition("awaitInput", "compose")
	graph.AddTransition("compose", workflow.END)
	return graph, nil
}
