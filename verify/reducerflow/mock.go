package reducerflow

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

	"github.com/microbus-io/fabric/verify/reducerflow/reducerflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA        func(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                                                           // MARKER: TaskA
	mockTaskB        func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)                                             // MARKER: TaskB
	mockTaskC        func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)                                             // MARKER: TaskC
	mockTaskD        func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)                                             // MARKER: TaskD
	mockTaskE        func(ctx context.Context, flow *workflow.Flow, sumTotal int, listTags []string, setSeen []string) (finalSum int, finalList []string, finalSet []string, err error) // MARKER: TaskE
	mockReducerGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                       // MARKER: Reducer
	unsubMockReducer func() error                                                                                                                                                       // MARKER: Reducer
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

// MockTaskB sets up a mock handler for TaskB.
func (svc *Mock) MockTaskB(handler func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)) *Mock { // MARKER: TaskB
	svc.mockTaskB = handler
	return svc
}

// TaskB executes the mock handler.
func (svc *Mock) TaskB(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error) { // MARKER: TaskB
	if svc.mockTaskB != nil {
		sumTotalOut, listTagsOut, setSeenOut, err = svc.mockTaskB(ctx, flow)
	}
	return sumTotalOut, listTagsOut, setSeenOut, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		sumTotalOut, listTagsOut, setSeenOut, err = svc.mockTaskC(ctx, flow)
	}
	return sumTotalOut, listTagsOut, setSeenOut, errors.Trace(err)
}

// MockTaskD sets up a mock handler for TaskD.
func (svc *Mock) MockTaskD(handler func(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error)) *Mock { // MARKER: TaskD
	svc.mockTaskD = handler
	return svc
}

// TaskD executes the mock handler.
func (svc *Mock) TaskD(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut []string, setSeenOut []string, err error) { // MARKER: TaskD
	if svc.mockTaskD != nil {
		sumTotalOut, listTagsOut, setSeenOut, err = svc.mockTaskD(ctx, flow)
	}
	return sumTotalOut, listTagsOut, setSeenOut, errors.Trace(err)
}

// MockTaskE sets up a mock handler for TaskE.
func (svc *Mock) MockTaskE(handler func(ctx context.Context, flow *workflow.Flow, sumTotal int, listTags []string, setSeen []string) (finalSum int, finalList []string, finalSet []string, err error)) *Mock { // MARKER: TaskE
	svc.mockTaskE = handler
	return svc
}

// TaskE executes the mock handler.
func (svc *Mock) TaskE(ctx context.Context, flow *workflow.Flow, sumTotal int, listTags []string, setSeen []string) (finalSum int, finalList []string, finalSet []string, err error) { // MARKER: TaskE
	if svc.mockTaskE != nil {
		finalSum, finalList, finalSet, err = svc.mockTaskE(ctx, flow, sumTotal, listTags, setSeen)
	}
	return finalSum, finalList, finalSet, errors.Trace(err)
}

// MockReducer sets up a mock handler for the Reducer workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockReducer(handler func(ctx context.Context, flow *workflow.Flow) (finalSum int, finalList []string, finalSet []string, err error)) *Mock { // MARKER: Reducer
	if svc.unsubMockReducer != nil {
		svc.unsubMockReducer()
		svc.unsubMockReducer = nil
	}
	if handler == nil {
		svc.mockReducerGraph = nil
		return svc
	}
	mockName := "MockReducer" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-reducer-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockReducerGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(reducerflowapi.Reducer.URL())
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
		var in reducerflowapi.ReducerIn
		f.ParseState(&in)
		finalSum, finalList, finalSet, err := handler(r.Context(), &f)
		if err != nil {
			return err // No trace
		}
		out := reducerflowapi.ReducerOut{FinalSum: finalSum, FinalList: finalList, FinalSet: finalSet}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(reducerflowapi.ReducerIn{}, reducerflowapi.ReducerOut{}),
	)
	if err == nil {
		svc.unsubMockReducer = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Reducer returns the workflow graph, or a mocked graph if MockReducer was called.
func (svc *Mock) Reducer(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Reducer
	if svc.mockReducerGraph != nil {
		graph, err = svc.mockReducerGraph(ctx)
	}
	return graph, errors.Trace(err)
}
