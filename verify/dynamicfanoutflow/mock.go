package dynamicfanoutflow

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

	"github.com/microbus-io/fabric/verify/dynamicfanoutflow/dynamicfanoutflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA              func(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error)    // MARKER: TaskA
	mockTaskB              func(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error)     // MARKER: TaskB
	mockTaskC              func(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error) // MARKER: TaskC
	mockDynamicFanOutGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                     // MARKER: DynamicFanOut
	unsubMockDynamicFanOut func() error                                                                                     // MARKER: DynamicFanOut
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		itemsOut, err = svc.mockTaskA(ctx, flow, items)
	}
	return itemsOut, errors.Trace(err)
}

// MockTaskB sets up a mock handler for TaskB.
func (svc *Mock) MockTaskB(handler func(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error)) *Mock { // MARKER: TaskB
	svc.mockTaskB = handler
	return svc
}

// TaskB executes the mock handler.
func (svc *Mock) TaskB(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error) { // MARKER: TaskB
	if svc.mockTaskB != nil {
		sumProcessedOut, err = svc.mockTaskB(ctx, flow, item)
	}
	return sumProcessedOut, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		processedCount, err = svc.mockTaskC(ctx, flow, sumProcessed)
	}
	return processedCount, errors.Trace(err)
}

// MockDynamicFanOut sets up a mock handler for the DynamicFanOut workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockDynamicFanOut(handler func(ctx context.Context, flow *workflow.Flow, items []string) (processedCount int, err error)) *Mock { // MARKER: DynamicFanOut
	if svc.unsubMockDynamicFanOut != nil {
		svc.unsubMockDynamicFanOut()
		svc.unsubMockDynamicFanOut = nil
	}
	if handler == nil {
		svc.mockDynamicFanOutGraph = nil
		return svc
	}
	mockName := "MockDynamicFanOut" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-dynamic-fan-out-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockDynamicFanOutGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(dynamicfanoutflowapi.DynamicFanOut.URL())
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
		var in dynamicfanoutflowapi.DynamicFanOutIn
		f.ParseState(&in)
		processedCount, err := handler(r.Context(), &f, in.Items)
		if err != nil {
			return err // No trace
		}
		out := dynamicfanoutflowapi.DynamicFanOutOut{ProcessedCount: processedCount}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(dynamicfanoutflowapi.DynamicFanOutIn{}, dynamicfanoutflowapi.DynamicFanOutOut{}),
	)
	if err == nil {
		svc.unsubMockDynamicFanOut = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// DynamicFanOut returns the workflow graph, or a mocked graph if MockDynamicFanOut was called.
func (svc *Mock) DynamicFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: DynamicFanOut
	if svc.mockDynamicFanOutGraph != nil {
		graph, err = svc.mockDynamicFanOutGraph(ctx)
	}
	return graph, errors.Trace(err)
}
