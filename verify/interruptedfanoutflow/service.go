package interruptedfanoutflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/interruptedfanoutflow/interruptedfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ interruptedfanoutflowapi.Client
)

/*
Service implements interruptedfanoutflow.verify, which exercises one fan-out
branch interrupting and being resumed before the fan-in completes.
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
Src is the static fan-out source. It returns instantly so the foreman fans out
to A, B and C.
*/
func (svc *Service) Src(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: Src
	return true, nil
}

/*
A is a normal fan-out branch contributing 1 to sumExecuted.
*/
func (svc *Service) A(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: A
	return 1, nil
}

/*
B is the interrupting branch. On its first run it parks the flow via
flow.Interrupt and contributes nothing. After Resume injects resumed=true into
its state, it re-runs, falls through, and contributes 1.
*/
func (svc *Service) B(ctx context.Context, flow *workflow.Flow, resumed bool) (sumExecutedOut int, err error) { // MARKER: B
	if !resumed {
		flow.Interrupt(map[string]any{"branch": "B"})
		return 0, nil
	}
	return 1, nil
}

/*
C is a normal fan-out branch contributing 1 to sumExecuted.
*/
func (svc *Service) C(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: C
	return 1, nil
}

/*
J is the fan-in target. It surfaces the summed sumExecuted (3 once A, B and C
have all contributed) as totalExecuted.
*/
func (svc *Service) J(ctx context.Context, flow *workflow.Flow, sumExecuted int) (totalExecuted int, err error) { // MARKER: J
	return sumExecuted, nil
}

/*
InterruptedFanOut defines the graph: Src -> {A, B, C} -> J. B interrupts on its
first run; the flow only completes once B is resumed.
*/
func (svc *Service) InterruptedFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: InterruptedFanOut
	graph = workflow.NewGraph(interruptedfanoutflowapi.InterruptedFanOut.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("sumExecuted", "totalExecuted")
	graph.AddTask("src", interruptedfanoutflowapi.Src.URL())
	graph.AddTask("a", interruptedfanoutflowapi.A.URL())
	graph.AddTask("b", interruptedfanoutflowapi.B.URL())
	graph.AddTask("c", interruptedfanoutflowapi.C.URL())
	graph.AddTask("j", interruptedfanoutflowapi.J.URL())
	graph.SetFanIn("j")
	graph.AddTransition("src", "a")
	graph.AddTransition("src", "b")
	graph.AddTransition("src", "c")
	graph.AddTransition("a", "j")
	graph.AddTransition("b", "j")
	graph.AddTransition("c", "j")
	graph.AddTransition("j", workflow.END)
	return graph, nil
}
