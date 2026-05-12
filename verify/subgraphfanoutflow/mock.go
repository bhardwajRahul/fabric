package subgraphfanoutflow

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

	"github.com/microbus-io/fabric/verify/subgraphfanoutflow/subgraphfanoutflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA          func(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                         // MARKER: TaskA
	mockNormalB        func(ctx context.Context, flow *workflow.Flow) (resultB string, err error)                                                       // MARKER: NormalB
	mockTaskX          func(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error)                                                         // MARKER: TaskX
	mockTaskY          func(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error)                                       // MARKER: TaskY
	mockNormalD        func(ctx context.Context, flow *workflow.Flow) (resultD string, err error)                                                       // MARKER: NormalD
	mockTaskE          func(ctx context.Context, flow *workflow.Flow, resultB string, subResult string, resultD string) (finalResult string, err error) // MARKER: TaskE
	mockSubGraph       func(ctx context.Context) (graph *workflow.Graph, err error)                                                                     // MARKER: Sub
	unsubMockSub       func() error                                                                                                                     // MARKER: Sub
	mockSubFanOutGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                                                     // MARKER: SubFanOut
	unsubMockSubFanOut func() error                                                                                                                     // MARKER: SubFanOut
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow) (started bool, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		started, err = svc.mockTaskA(ctx, flow)
	}
	return started, errors.Trace(err)
}

// MockNormalB sets up a mock handler for NormalB.
func (svc *Mock) MockNormalB(handler func(ctx context.Context, flow *workflow.Flow) (resultB string, err error)) *Mock { // MARKER: NormalB
	svc.mockNormalB = handler
	return svc
}

// NormalB executes the mock handler.
func (svc *Mock) NormalB(ctx context.Context, flow *workflow.Flow) (resultB string, err error) { // MARKER: NormalB
	if svc.mockNormalB != nil {
		resultB, err = svc.mockNormalB(ctx, flow)
	}
	return resultB, errors.Trace(err)
}

// MockTaskX sets up a mock handler for TaskX.
func (svc *Mock) MockTaskX(handler func(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error)) *Mock { // MARKER: TaskX
	svc.mockTaskX = handler
	return svc
}

// TaskX executes the mock handler.
func (svc *Mock) TaskX(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error) { // MARKER: TaskX
	if svc.mockTaskX != nil {
		xPassed, err = svc.mockTaskX(ctx, flow)
	}
	return xPassed, errors.Trace(err)
}

// MockTaskY sets up a mock handler for TaskY.
func (svc *Mock) MockTaskY(handler func(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error)) *Mock { // MARKER: TaskY
	svc.mockTaskY = handler
	return svc
}

// TaskY executes the mock handler.
func (svc *Mock) TaskY(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error) { // MARKER: TaskY
	if svc.mockTaskY != nil {
		subResult, err = svc.mockTaskY(ctx, flow, xPassed)
	}
	return subResult, errors.Trace(err)
}

// MockNormalD sets up a mock handler for NormalD.
func (svc *Mock) MockNormalD(handler func(ctx context.Context, flow *workflow.Flow) (resultD string, err error)) *Mock { // MARKER: NormalD
	svc.mockNormalD = handler
	return svc
}

// NormalD executes the mock handler.
func (svc *Mock) NormalD(ctx context.Context, flow *workflow.Flow) (resultD string, err error) { // MARKER: NormalD
	if svc.mockNormalD != nil {
		resultD, err = svc.mockNormalD(ctx, flow)
	}
	return resultD, errors.Trace(err)
}

// MockTaskE sets up a mock handler for TaskE.
func (svc *Mock) MockTaskE(handler func(ctx context.Context, flow *workflow.Flow, resultB string, subResult string, resultD string) (finalResult string, err error)) *Mock { // MARKER: TaskE
	svc.mockTaskE = handler
	return svc
}

// TaskE executes the mock handler.
func (svc *Mock) TaskE(ctx context.Context, flow *workflow.Flow, resultB string, subResult string, resultD string) (finalResult string, err error) { // MARKER: TaskE
	if svc.mockTaskE != nil {
		finalResult, err = svc.mockTaskE(ctx, flow, resultB, subResult, resultD)
	}
	return finalResult, errors.Trace(err)
}

// MockSub sets up a mock handler for the Sub workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockSub(handler func(ctx context.Context, flow *workflow.Flow) (subResult string, err error)) *Mock { // MARKER: Sub
	if svc.unsubMockSub != nil {
		svc.unsubMockSub()
		svc.unsubMockSub = nil
	}
	if handler == nil {
		svc.mockSubGraph = nil
		return svc
	}
	mockName := "MockSub" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-sub-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockSubGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(subgraphfanoutflowapi.Sub.URL())
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
		var in subgraphfanoutflowapi.SubIn
		f.ParseState(&in)
		subResult, err := handler(r.Context(), &f)
		if err != nil {
			return err // No trace
		}
		out := subgraphfanoutflowapi.SubOut{SubResult: subResult}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(subgraphfanoutflowapi.SubIn{}, subgraphfanoutflowapi.SubOut{}),
	)
	if err == nil {
		svc.unsubMockSub = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Sub returns the workflow graph, or a mocked graph if MockSub was called.
func (svc *Mock) Sub(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Sub
	if svc.mockSubGraph != nil {
		graph, err = svc.mockSubGraph(ctx)
	}
	return graph, errors.Trace(err)
}

// MockSubFanOut sets up a mock handler for the SubFanOut workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockSubFanOut(handler func(ctx context.Context, flow *workflow.Flow) (finalResult string, err error)) *Mock { // MARKER: SubFanOut
	if svc.unsubMockSubFanOut != nil {
		svc.unsubMockSubFanOut()
		svc.unsubMockSubFanOut = nil
	}
	if handler == nil {
		svc.mockSubFanOutGraph = nil
		return svc
	}
	mockName := "MockSubFanOut" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-sub-fan-out-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockSubFanOutGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(subgraphfanoutflowapi.SubFanOut.URL())
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
		var in subgraphfanoutflowapi.SubFanOutIn
		f.ParseState(&in)
		finalResult, err := handler(r.Context(), &f)
		if err != nil {
			return err // No trace
		}
		out := subgraphfanoutflowapi.SubFanOutOut{FinalResult: finalResult}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(subgraphfanoutflowapi.SubFanOutIn{}, subgraphfanoutflowapi.SubFanOutOut{}),
	)
	if err == nil {
		svc.unsubMockSubFanOut = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// SubFanOut returns the workflow graph, or a mocked graph if MockSubFanOut was called.
func (svc *Mock) SubFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: SubFanOut
	if svc.mockSubFanOutGraph != nil {
		graph, err = svc.mockSubFanOutGraph(ctx)
	}
	return graph, errors.Trace(err)
}
