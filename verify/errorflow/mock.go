package errorflow

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

	"github.com/microbus-io/fabric/verify/errorflow/errorflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA      func(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error)        // MARKER: TaskA
	mockTaskB      func(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error)            // MARKER: TaskB
	mockHandler    func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error) // MARKER: Handler
	mockTaskC      func(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error)        // MARKER: TaskC
	mockErrorGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                         // MARKER: Error
	unsubMockError func() error                                                                                         // MARKER: Error
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		triggerOut, err = svc.mockTaskA(ctx, flow, trigger)
	}
	return triggerOut, errors.Trace(err)
}

// MockTaskB sets up a mock handler for TaskB.
func (svc *Mock) MockTaskB(handler func(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error)) *Mock { // MARKER: TaskB
	svc.mockTaskB = handler
	return svc
}

// TaskB executes the mock handler.
func (svc *Mock) TaskB(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error) { // MARKER: TaskB
	if svc.mockTaskB != nil {
		result, err = svc.mockTaskB(ctx, flow, trigger)
	}
	return result, errors.Trace(err)
}

// MockHandler sets up a mock handler for Handler.
func (svc *Mock) MockHandler(handler func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error)) *Mock { // MARKER: Handler
	svc.mockHandler = handler
	return svc
}

// Handler executes the mock handler.
func (svc *Mock) Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error) { // MARKER: Handler
	if svc.mockHandler != nil {
		result, err = svc.mockHandler(ctx, flow, onErr)
	}
	return result, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		finalResult, err = svc.mockTaskC(ctx, flow, result)
	}
	return finalResult, errors.Trace(err)
}

// MockError sets up a mock handler for the Error workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockError(handler func(ctx context.Context, flow *workflow.Flow, trigger string) (finalResult string, err error)) *Mock { // MARKER: Error
	if svc.unsubMockError != nil {
		svc.unsubMockError()
		svc.unsubMockError = nil
	}
	if handler == nil {
		svc.mockErrorGraph = nil
		return svc
	}
	mockName := "MockError" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-error-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockErrorGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(errorflowapi.Error.URL())
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
		var in errorflowapi.ErrorIn
		f.ParseState(&in)
		finalResult, err := handler(r.Context(), &f, in.Trigger)
		if err != nil {
			return err // No trace
		}
		out := errorflowapi.ErrorOut{FinalResult: finalResult}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(errorflowapi.ErrorIn{}, errorflowapi.ErrorOut{}),
	)
	if err == nil {
		svc.unsubMockError = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Error returns the workflow graph, or a mocked graph if MockError was called.
func (svc *Mock) Error(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Error
	if svc.mockErrorGraph != nil {
		graph, err = svc.mockErrorGraph(ctx)
	}
	return graph, errors.Trace(err)
}
