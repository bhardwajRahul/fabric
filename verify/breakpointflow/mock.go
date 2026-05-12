package breakpointflow

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/breakpointflow/breakpointflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA           func(ctx context.Context, flow *workflow.Flow) (stepA bool, err error)             // MARKER: TaskA
	mockTaskB           func(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error) // MARKER: TaskB
	mockTaskC           func(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error) // MARKER: TaskC
	mockBreakpointGraph func(ctx context.Context) (graph *workflow.Graph, err error)                       // MARKER: Breakpoint
	unsubMockBreakpoint func() error                                                                       // MARKER: Breakpoint
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockTaskA sets up a mock handler for TaskA.
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow) (stepA bool, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow) (stepA bool, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		stepA, err = svc.mockTaskA(ctx, flow)
	}
	return stepA, errors.Trace(err)
}

// MockTaskB sets up a mock handler for TaskB.
func (svc *Mock) MockTaskB(handler func(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error)) *Mock { // MARKER: TaskB
	svc.mockTaskB = handler
	return svc
}

// TaskB executes the mock handler.
func (svc *Mock) TaskB(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error) { // MARKER: TaskB
	if svc.mockTaskB != nil {
		stepB, err = svc.mockTaskB(ctx, flow, stepA)
	}
	return stepB, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		stepC, err = svc.mockTaskC(ctx, flow, stepB)
	}
	return stepC, errors.Trace(err)
}

// MockBreakpoint sets up a mock handler for the Breakpoint workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockBreakpoint(handler func(ctx context.Context, flow *workflow.Flow) (stepC bool, err error)) *Mock { // MARKER: Breakpoint
	if svc.unsubMockBreakpoint != nil {
		svc.unsubMockBreakpoint()
		svc.unsubMockBreakpoint = nil
	}
	if handler == nil {
		svc.mockBreakpointGraph = nil
		return svc
	}
	mockName := "MockBreakpoint" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-breakpoint-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockBreakpointGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(breakpointflowapi.Breakpoint.URL())
		g.AddTransition(mockTaskURL, workflow.END)
		g.DeclareInputs("*")
		g.DeclareOutputs("*")
		return g, nil
	}
	err := svc.Subscribe(mockName, func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			return errors.Trace(err)
		}
		snap := f.Snapshot()
		var in breakpointflowapi.BreakpointIn
		f.ParseState(&in)
		stepC, err := handler(r.Context(), &f)
		if err != nil {
			return err // No trace
		}
		out := breakpointflowapi.BreakpointOut{StepC: stepC}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(breakpointflowapi.BreakpointIn{}, breakpointflowapi.BreakpointOut{}),
	)
	if err == nil {
		svc.unsubMockBreakpoint = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Breakpoint returns the workflow graph, or a mocked graph if MockBreakpoint was called.
func (svc *Mock) Breakpoint(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Breakpoint
	if svc.mockBreakpointGraph != nil {
		graph, err = svc.mockBreakpointGraph(ctx)
	}
	return graph, errors.Trace(err)
}
