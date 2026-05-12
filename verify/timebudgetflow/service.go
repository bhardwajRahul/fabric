package timebudgetflow

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/timebudgetflow/timebudgetflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ timebudgetflowapi.Client
)

/*
Service implements timebudgetflow.verify, exercising per-task time budgets.
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
TaskA is the entry point.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	return true, nil
}

/*
Slow sleeps for 500ms, well beyond the 50ms budget configured by the workflow.
The foreman cancels the request via the ctx; the task observes the cancellation
and returns ctx.Err(), which the foreman surfaces as a step failure with status 408.
*/
func (svc *Service) Slow(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Slow
	select {
	case <-time.After(500 * time.Millisecond):
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

/*
TimeBudget defines A -> Slow with a 50ms budget on Slow.
*/
func (svc *Service) TimeBudget(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: TimeBudget
	graph = workflow.NewGraph(timebudgetflowapi.TimeBudget.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("done")
	graph.AddTask("taskA", timebudgetflowapi.TaskA.URL())
	graph.AddTask("slow", timebudgetflowapi.Slow.URL())
	graph.AddTransition("taskA", "slow")
	graph.AddTransition("slow", workflow.END)
	graph.SetTimeBudget("slow", 50*time.Millisecond)
	return graph, nil
}
