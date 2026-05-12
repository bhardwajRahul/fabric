package conditionalflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/conditionalflow/conditionalflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ conditionalflowapi.Client
)

/*
Service implements conditionalflow.verify, exercising when-based conditional transitions.
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
TaskA passes the score through to state.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error) { // MARKER: TaskA
	return score, nil
}

/*
TaskHigh runs when score>=50.
*/
func (svc *Service) TaskHigh(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) { // MARKER: TaskHigh
	return "high", nil
}

/*
TaskLow runs when score<50.
*/
func (svc *Service) TaskLow(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) { // MARKER: TaskLow
	return "low", nil
}

/*
TaskC surfaces the branch that ran.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error) { // MARKER: TaskC
	return branch, nil
}

/*
Conditional defines the graph A -> {TaskHigh (when score>=50), TaskLow (when score<50)} -> C.
*/
func (svc *Service) Conditional(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Conditional
	graph = workflow.NewGraph(conditionalflowapi.Conditional.URL())
	graph.DeclareInputs("score")
	graph.DeclareOutputs("finalBranch")
	graph.AddTask("taskA", conditionalflowapi.TaskA.URL())
	graph.AddTask("taskHigh", conditionalflowapi.TaskHigh.URL())
	graph.AddTask("taskLow", conditionalflowapi.TaskLow.URL())
	graph.AddTask("taskC", conditionalflowapi.TaskC.URL())
	graph.SetFanIn("taskC")
	graph.AddTransitionWhen("taskA", "taskHigh", "score >= 50")
	graph.AddTransitionWhen("taskA", "taskLow", "score < 50")
	graph.AddTransition("taskHigh", "taskC")
	graph.AddTransition("taskLow", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}
