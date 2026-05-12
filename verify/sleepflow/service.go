package sleepflow

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/sleepflow/sleepflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ sleepflowapi.Client
	_ time.Duration
)

/*
Service implements sleepflow.verify, exercising flow.Sleep.
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
TaskA passes the sleep duration through.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (sleepForOut time.Duration, err error) { // MARKER: TaskA
	return sleepFor, nil
}

/*
TaskB calls flow.Sleep(sleepFor) to delay TaskC's dispatch.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (marked bool, err error) { // MARKER: TaskB
	flow.Sleep(sleepFor)
	return true, nil
}

/*
TaskC runs after the sleep elapses.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, marked bool) (completed bool, err error) { // MARKER: TaskC
	return marked, nil
}

/*
Delay defines the graph A -> B -> C with B calling flow.Sleep before C runs.
The workflow is named Delay (not Sleep) to avoid colliding with the connector's own Sleep method.
*/
func (svc *Service) Delay(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Delay
	graph = workflow.NewGraph(sleepflowapi.Delay.URL())
	graph.DeclareInputs("sleepFor")
	graph.DeclareOutputs("completed")
	graph.AddTask("taskA", sleepflowapi.TaskA.URL())
	graph.AddTask("taskB", sleepflowapi.TaskB.URL())
	graph.AddTask("taskC", sleepflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
