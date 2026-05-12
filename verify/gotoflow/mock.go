package gotoflow

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

	"github.com/microbus-io/fabric/verify/gotoflow/gotoflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA        func(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error)             // MARKER: TaskA
	mockTaskB        func(ctx context.Context, flow *workflow.Flow, loops int, target int) (visited bool, err error) // MARKER: TaskB
	mockTaskC        func(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error)           // MARKER: TaskC
	mockBadGotoer    func(ctx context.Context, flow *workflow.Flow) (stamp bool, err error)                          // MARKER: BadGotoer
	mockGotoGraph    func(ctx context.Context) (graph *workflow.Graph, err error)                                    // MARKER: Goto
	unsubMockGoto    func() error                                                                                    // MARKER: Goto
	mockBadGotoGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                    // MARKER: BadGoto
	unsubMockBadGoto func() error                                                                                    // MARKER: BadGoto
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		loopsOut, err = svc.mockTaskA(ctx, flow, loops)
	}
	return loopsOut, errors.Trace(err)
}

// MockTaskB sets up a mock handler for TaskB.
func (svc *Mock) MockTaskB(handler func(ctx context.Context, flow *workflow.Flow, loops int, target int) (visited bool, err error)) *Mock { // MARKER: TaskB
	svc.mockTaskB = handler
	return svc
}

// TaskB executes the mock handler.
func (svc *Mock) TaskB(ctx context.Context, flow *workflow.Flow, loops int, target int) (visited bool, err error) { // MARKER: TaskB
	if svc.mockTaskB != nil {
		visited, err = svc.mockTaskB(ctx, flow, loops, target)
	}
	return visited, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		finalLoops, err = svc.mockTaskC(ctx, flow, loops)
	}
	return finalLoops, errors.Trace(err)
}

// MockBadGotoer sets up a mock handler for BadGotoer.
func (svc *Mock) MockBadGotoer(handler func(ctx context.Context, flow *workflow.Flow) (stamp bool, err error)) *Mock { // MARKER: BadGotoer
	svc.mockBadGotoer = handler
	return svc
}

// BadGotoer executes the mock handler.
func (svc *Mock) BadGotoer(ctx context.Context, flow *workflow.Flow) (stamp bool, err error) { // MARKER: BadGotoer
	if svc.mockBadGotoer != nil {
		stamp, err = svc.mockBadGotoer(ctx, flow)
	}
	return stamp, errors.Trace(err)
}

// MockGoto sets up a mock handler for the Goto workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockGoto(handler func(ctx context.Context, flow *workflow.Flow, target int) (finalLoops int, err error)) *Mock { // MARKER: Goto
	if svc.unsubMockGoto != nil {
		svc.unsubMockGoto()
		svc.unsubMockGoto = nil
	}
	if handler == nil {
		svc.mockGotoGraph = nil
		return svc
	}
	mockName := "MockGoto" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-goto-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockGotoGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(gotoflowapi.Goto.URL())
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
		var in gotoflowapi.GotoIn
		f.ParseState(&in)
		finalLoops, err := handler(r.Context(), &f, in.Target)
		if err != nil {
			return err // No trace
		}
		out := gotoflowapi.GotoOut{FinalLoops: finalLoops}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(gotoflowapi.GotoIn{}, gotoflowapi.GotoOut{}),
	)
	if err == nil {
		svc.unsubMockGoto = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Goto returns the workflow graph, or a mocked graph if MockGoto was called.
func (svc *Mock) Goto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Goto
	if svc.mockGotoGraph != nil {
		graph, err = svc.mockGotoGraph(ctx)
	}
	return graph, errors.Trace(err)
}

// MockBadGoto sets up a mock handler for the BadGoto workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockBadGoto(handler func(ctx context.Context, flow *workflow.Flow) (stamp bool, err error)) *Mock { // MARKER: BadGoto
	if svc.unsubMockBadGoto != nil {
		svc.unsubMockBadGoto()
		svc.unsubMockBadGoto = nil
	}
	if handler == nil {
		svc.mockBadGotoGraph = nil
		return svc
	}
	mockName := "MockBadGoto" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-bad-goto-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockBadGotoGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(gotoflowapi.BadGoto.URL())
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
		var in gotoflowapi.BadGotoIn
		f.ParseState(&in)
		stamp, err := handler(r.Context(), &f)
		if err != nil {
			return err // No trace
		}
		out := gotoflowapi.BadGotoOut{Stamp: stamp}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(gotoflowapi.BadGotoIn{}, gotoflowapi.BadGotoOut{}),
	)
	if err == nil {
		svc.unsubMockBadGoto = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// BadGoto returns the workflow graph, or a mocked graph if MockBadGoto was called.
func (svc *Mock) BadGoto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: BadGoto
	if svc.mockBadGotoGraph != nil {
		graph, err = svc.mockBadGotoGraph(ctx)
	}
	return graph, errors.Trace(err)
}
