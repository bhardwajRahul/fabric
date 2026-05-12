package aliasflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/aliasflow/aliasflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ aliasflowapi.Client
)

/*
Service implements aliasflow.verify, exercising a workflow graph in which the same task URL
appears at two distinct positions under two distinct node names.
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
TaskS dispatches the flow. When branch=="alt" it calls flow.Goto("bPrime") to take the alternate
path; otherwise the default normal transition s -> a fires.
*/
func (svc *Service) TaskS(ctx context.Context, flow *workflow.Flow, branch string) (branchOut string, err error) { // MARKER: TaskS
	if branch == "alt" {
		flow.Goto("bPrime")
	}
	return branch, nil
}

/*
TaskA appends "A" to the path.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskA
	return path + "A", nil
}

/*
TaskB appends "B" to the path. Reused at two graph positions under names "b" and "bPrime".
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskB
	return path + "B", nil
}

/*
TaskC appends "C" to the path.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskC
	return path + "C", nil
}

/*
TaskD appends "D" to the path.
*/
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error) { // MARKER: TaskD
	return path + "D", nil
}

/*
Alias defines the graph s -> a -> b -> c -> END with an alternate Goto-driven path
s -> bPrime -> d -> END. The two nodes "b" and "bPrime" share the same task URL (TaskB.URL())
but are independently identified in the graph and in the step history.
*/
func (svc *Service) Alias(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Alias
	graph = workflow.NewGraph(aliasflowapi.Alias.URL())
	graph.DeclareInputs("branch")
	graph.DeclareOutputs("path")
	graph.AddTask("s", aliasflowapi.TaskS.URL())
	graph.AddTask("a", aliasflowapi.TaskA.URL())
	graph.AddTask("b", aliasflowapi.TaskB.URL())
	graph.AddTask("c", aliasflowapi.TaskC.URL())
	graph.AddTask("bPrime", aliasflowapi.TaskB.URL())
	graph.AddTask("d", aliasflowapi.TaskD.URL())
	graph.AddTransition("s", "a")
	graph.AddTransitionGoto("s", "bPrime")
	graph.AddTransition("a", "b")
	graph.AddTransition("b", "c")
	graph.AddTransition("c", workflow.END)
	graph.AddTransition("bPrime", "d")
	graph.AddTransition("d", workflow.END)
	return graph, nil
}
