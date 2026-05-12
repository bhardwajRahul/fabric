package breakpointflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/breakpointflow/breakpointflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ breakpointflowapi.Client
)

/*
Service implements breakpointflow.verify, exercising BreakBefore + Resume.
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
TaskA marks stepA as visited.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (stepA bool, err error) { // MARKER: TaskA
	return true, nil
}

/*
TaskB marks stepB as visited. Should NOT run if the breakpoint is hit before this task.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error) { // MARKER: TaskB
	return stepA, nil
}

/*
TaskC marks stepC as visited.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error) { // MARKER: TaskC
	return stepB, nil
}

/*
Breakpoint defines the graph A -> B -> C.
*/
func (svc *Service) Breakpoint(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Breakpoint
	graph = workflow.NewGraph(breakpointflowapi.Breakpoint.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("stepC")
	graph.AddTask("taskA", breakpointflowapi.TaskA.URL())
	graph.AddTask("taskB", breakpointflowapi.TaskB.URL())
	graph.AddTask("taskC", breakpointflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
