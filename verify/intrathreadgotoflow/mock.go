package intrathreadgotoflow

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

	"github.com/microbus-io/fabric/verify/intrathreadgotoflow/intrathreadgotoflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA                func(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error)                   // MARKER: TaskA
	mockLoopTask             func(ctx context.Context, flow *workflow.Flow, loops int, target int) (loopsOut int, err error)         // MARKER: LoopTask
	mockNormalC              func(ctx context.Context, flow *workflow.Flow) (stamp string, err error)                                // MARKER: NormalC
	mockTaskD                func(ctx context.Context, flow *workflow.Flow, loops int, stamp string) (finalResult string, err error) // MARKER: TaskD
	mockIntraThreadGotoGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                            // MARKER: IntraThreadGoto
	unsubMockIntraThreadGoto func() error                                                                                            // MARKER: IntraThreadGoto
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		targetOut, err = svc.mockTaskA(ctx, flow, target)
	}
	return targetOut, errors.Trace(err)
}

// MockLoopTask sets up a mock handler for LoopTask.
func (svc *Mock) MockLoopTask(handler func(ctx context.Context, flow *workflow.Flow, loops int, target int) (loopsOut int, err error)) *Mock { // MARKER: LoopTask
	svc.mockLoopTask = handler
	return svc
}

// LoopTask executes the mock handler.
func (svc *Mock) LoopTask(ctx context.Context, flow *workflow.Flow, loops int, target int) (loopsOut int, err error) { // MARKER: LoopTask
	if svc.mockLoopTask != nil {
		loopsOut, err = svc.mockLoopTask(ctx, flow, loops, target)
	}
	return loopsOut, errors.Trace(err)
}

// MockNormalC sets up a mock handler for NormalC.
func (svc *Mock) MockNormalC(handler func(ctx context.Context, flow *workflow.Flow) (stamp string, err error)) *Mock { // MARKER: NormalC
	svc.mockNormalC = handler
	return svc
}

// NormalC executes the mock handler.
func (svc *Mock) NormalC(ctx context.Context, flow *workflow.Flow) (stamp string, err error) { // MARKER: NormalC
	if svc.mockNormalC != nil {
		stamp, err = svc.mockNormalC(ctx, flow)
	}
	return stamp, errors.Trace(err)
}

// MockTaskD sets up a mock handler for TaskD.
func (svc *Mock) MockTaskD(handler func(ctx context.Context, flow *workflow.Flow, loops int, stamp string) (finalResult string, err error)) *Mock { // MARKER: TaskD
	svc.mockTaskD = handler
	return svc
}

// TaskD executes the mock handler.
func (svc *Mock) TaskD(ctx context.Context, flow *workflow.Flow, loops int, stamp string) (finalResult string, err error) { // MARKER: TaskD
	if svc.mockTaskD != nil {
		finalResult, err = svc.mockTaskD(ctx, flow, loops, stamp)
	}
	return finalResult, errors.Trace(err)
}

// MockIntraThreadGoto sets up a mock handler for the IntraThreadGoto workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockIntraThreadGoto(handler func(ctx context.Context, flow *workflow.Flow, target int) (finalResult string, err error)) *Mock { // MARKER: IntraThreadGoto
	if svc.unsubMockIntraThreadGoto != nil {
		svc.unsubMockIntraThreadGoto()
		svc.unsubMockIntraThreadGoto = nil
	}
	if handler == nil {
		svc.mockIntraThreadGotoGraph = nil
		return svc
	}
	mockName := "MockIntraThreadGoto" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-intra-thread-goto-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockIntraThreadGotoGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(intrathreadgotoflowapi.IntraThreadGoto.URL())
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
		var in intrathreadgotoflowapi.IntraThreadGotoIn
		f.ParseState(&in)
		finalResult, err := handler(r.Context(), &f, in.Target)
		if err != nil {
			return err // No trace
		}
		out := intrathreadgotoflowapi.IntraThreadGotoOut{FinalResult: finalResult}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(intrathreadgotoflowapi.IntraThreadGotoIn{}, intrathreadgotoflowapi.IntraThreadGotoOut{}),
	)
	if err == nil {
		svc.unsubMockIntraThreadGoto = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// IntraThreadGoto returns the workflow graph, or a mocked graph if MockIntraThreadGoto was called.
func (svc *Mock) IntraThreadGoto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: IntraThreadGoto
	if svc.mockIntraThreadGotoGraph != nil {
		graph, err = svc.mockIntraThreadGotoGraph(ctx)
	}
	return graph, errors.Trace(err)
}
