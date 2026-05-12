package continueflow

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

	"github.com/microbus-io/fabric/verify/continueflow/continueflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockIncrement     func(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) // MARKER: Increment
	mockCountingGraph func(ctx context.Context) (graph *workflow.Graph, err error)                            // MARKER: Counting
	unsubMockCounting func() error                                                                            // MARKER: Counting
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

// MockIncrement sets up a mock handler for Increment.
func (svc *Mock) MockIncrement(handler func(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error)) *Mock { // MARKER: Increment
	svc.mockIncrement = handler
	return svc
}

// Increment executes the mock handler.
func (svc *Mock) Increment(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) { // MARKER: Increment
	if svc.mockIncrement != nil {
		counterOut, err = svc.mockIncrement(ctx, flow, counter)
	}
	return counterOut, errors.Trace(err)
}

// MockCounting sets up a mock handler for the Counting workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockCounting(handler func(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error)) *Mock { // MARKER: Counting
	if svc.unsubMockCounting != nil {
		svc.unsubMockCounting()
		svc.unsubMockCounting = nil
	}
	if handler == nil {
		svc.mockCountingGraph = nil
		return svc
	}
	mockName := "MockCounting" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-counting-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockCountingGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(continueflowapi.Counting.URL())
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
		var in continueflowapi.CountingIn
		f.ParseState(&in)
		counterOut, err := handler(r.Context(), &f, in.Counter)
		if err != nil {
			return err // No trace
		}
		out := continueflowapi.CountingOut{CounterOut: counterOut}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(continueflowapi.CountingIn{}, continueflowapi.CountingOut{}),
	)
	if err == nil {
		svc.unsubMockCounting = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Counting returns the workflow graph, or a mocked graph if MockCounting was called.
func (svc *Mock) Counting(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Counting
	if svc.mockCountingGraph != nil {
		graph, err = svc.mockCountingGraph(ctx)
	}
	return graph, errors.Trace(err)
}
