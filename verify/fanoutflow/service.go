package fanoutflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/fanoutflow/fanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ fanoutflowapi.Client
)

/*
Service implements fanoutflow.verify which exercises static fan-out + fan-in.
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
TaskA is the fan-out source. Marks A as visited.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (markA bool, err error) { // MARKER: TaskA
	return true, nil
}

/*
TaskB is one of three parallel branches. Marks B as visited.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, markA bool) (markB bool, err error) { // MARKER: TaskB
	return markA, nil
}

/*
TaskC is one of three parallel branches. Marks C as visited.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, markA bool) (markC bool, err error) { // MARKER: TaskC
	return markA, nil
}

/*
TaskD is one of three parallel branches. Marks D as visited.
*/
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow, markA bool) (markD bool, err error) { // MARKER: TaskD
	return markA, nil
}

/*
TaskE is the fan-in target. Confirms all three branches executed.
*/
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow, markB bool, markC bool, markD bool) (allMarked bool, err error) { // MARKER: TaskE
	return markB && markC && markD, nil
}

/*
FanOut defines the workflow graph for A -> {B, C, D} -> E.
*/
func (svc *Service) FanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: FanOut
	graph = workflow.NewGraph(fanoutflowapi.FanOut.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("allMarked")
	graph.AddTask("taskA", fanoutflowapi.TaskA.URL())
	graph.AddTask("taskB", fanoutflowapi.TaskB.URL())
	graph.AddTask("taskC", fanoutflowapi.TaskC.URL())
	graph.AddTask("taskD", fanoutflowapi.TaskD.URL())
	graph.AddTask("taskE", fanoutflowapi.TaskE.URL())
	graph.SetFanIn("taskE")
	// Fan-out from A
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskA", "taskC")
	graph.AddTransition("taskA", "taskD")
	// Fan-in at E
	graph.AddTransition("taskB", "taskE")
	graph.AddTransition("taskC", "taskE")
	graph.AddTransition("taskD", "taskE")
	graph.AddTransition("taskE", workflow.END)
	return graph, nil
}
