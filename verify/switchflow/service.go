package switchflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/switchflow/switchflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ switchflowapi.Client
)

/*
Service implements switchflow.verify, exercising first-match-wins Switch transitions.
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
Router passes the amount through to state so downstream Switch transitions can match on it.
*/
func (svc *Service) Router(ctx context.Context, flow *workflow.Flow, amount int) (amountOut int, err error) { // MARKER: Router
	return amount, nil
}

/*
HandleHigh runs when the first switch arm matches (amount>=10000).
*/
func (svc *Service) HandleHigh(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error) { // MARKER: HandleHigh
	return "high", nil
}

/*
HandleMid runs when the second switch arm matches (amount>=1000).
*/
func (svc *Service) HandleMid(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error) { // MARKER: HandleMid
	return "mid", nil
}

/*
HandleLow runs as the default switch arm (when="true").
*/
func (svc *Service) HandleLow(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error) { // MARKER: HandleLow
	return "low", nil
}

/*
Switch defines the graph Router -> {HandleHigh (amount>=10000) | HandleMid (amount>=1000) | HandleLow (true)} -> END.
Exactly one branch runs per execution; the trailing arm uses when="true" so every input is covered.
*/
func (svc *Service) Switch(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Switch
	graph = workflow.NewGraph(switchflowapi.Switch.URL())
	graph.AddTask("router", switchflowapi.Router.URL())
	graph.AddTask("handleHigh", switchflowapi.HandleHigh.URL())
	graph.AddTask("handleMid", switchflowapi.HandleMid.URL())
	graph.AddTask("handleLow", switchflowapi.HandleLow.URL())
	graph.AddTransitionSwitch("router", "handleHigh", "amount >= 10000")
	graph.AddTransitionSwitch("router", "handleMid", "amount >= 1000")
	graph.AddTransitionSwitch("router", "handleLow", "true")
	graph.AddTransition("handleHigh", workflow.END)
	graph.AddTransition("handleMid", workflow.END)
	graph.AddTransition("handleLow", workflow.END)
	return graph, nil
}

/*
SwitchNoMatch defines the same router but with no default branch: only two arms (high and
mid). An input that satisfies neither predicate ends the flow at the router with no branch
field set. The pinned semantic is "no-match -> flow completes cleanly, side-channel absent."
*/
func (svc *Service) SwitchNoMatch(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: SwitchNoMatch
	graph = workflow.NewGraph(switchflowapi.SwitchNoMatch.URL())
	graph.AddTask("router", switchflowapi.Router.URL())
	graph.AddTask("handleHigh", switchflowapi.HandleHigh.URL())
	graph.AddTask("handleMid", switchflowapi.HandleMid.URL())
	graph.AddTransitionSwitch("router", "handleHigh", "amount >= 10000")
	graph.AddTransitionSwitch("router", "handleMid", "amount >= 1000")
	graph.AddTransition("handleHigh", workflow.END)
	graph.AddTransition("handleMid", workflow.END)
	return graph, nil
}
