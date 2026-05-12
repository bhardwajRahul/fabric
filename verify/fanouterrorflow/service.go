package fanouterrorflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/fanouterrorflow/fanouterrorflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ fanouterrorflowapi.Client
)

/*
Service implements fanouterrorflow.verify, exercising OnError + fan-out.
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
TaskE is the fan-in target. It surfaces "recovered=true" if the handler ran.
*/
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow, handled, markB, markC, markD bool) (recovered bool, err error) { // MARKER: TaskE
	// The error path should have run Handler (handled=true).
	// markB should NOT be true (TaskB errored).
	// markC/markD may or may not be true depending on race with sibling-cancel.
	return handled && !markB, nil
}

/*
FanOutError defines the graph A -> {B, C, D} -> E with B onError -> Handler -> E.
*/
func (svc *Service) FanOutError(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: FanOutError
	graph = workflow.NewGraph(fanouterrorflowapi.FanOutError.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("recovered")
	graph.AddTask("taskA", fanouterrorflowapi.TaskA.URL())
	graph.AddTask("taskB", fanouterrorflowapi.TaskB.URL())
	graph.AddTask("taskC", fanouterrorflowapi.TaskC.URL())
	graph.AddTask("taskD", fanouterrorflowapi.TaskD.URL())
	graph.AddTask("handler", fanouterrorflowapi.Handler.URL())
	graph.AddTask("taskE", fanouterrorflowapi.TaskE.URL())
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
