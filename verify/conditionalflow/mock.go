package conditionalflow

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

	"github.com/microbus-io/fabric/verify/conditionalflow/conditionalflowapi"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTaskA            func(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error)           // MARKER: TaskA
	mockTaskHigh         func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)          // MARKER: TaskHigh
	mockTaskLow          func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)          // MARKER: TaskLow
	mockTaskC            func(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error) // MARKER: TaskC
	mockConditionalGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                  // MARKER: Conditional
	unsubMockConditional func() error                                                                                  // MARKER: Conditional
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
func (svc *Mock) MockTaskA(handler func(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error)) *Mock { // MARKER: TaskA
	svc.mockTaskA = handler
	return svc
}

// TaskA executes the mock handler.
func (svc *Mock) TaskA(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error) { // MARKER: TaskA
	if svc.mockTaskA != nil {
		scoreOut, err = svc.mockTaskA(ctx, flow, score)
	}
	return scoreOut, errors.Trace(err)
}

// MockTaskHigh sets up a mock handler for TaskHigh.
func (svc *Mock) MockTaskHigh(handler func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)) *Mock { // MARKER: TaskHigh
	svc.mockTaskHigh = handler
	return svc
}

// TaskHigh executes the mock handler.
func (svc *Mock) TaskHigh(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) { // MARKER: TaskHigh
	if svc.mockTaskHigh != nil {
		branch, err = svc.mockTaskHigh(ctx, flow, score)
	}
	return branch, errors.Trace(err)
}

// MockTaskLow sets up a mock handler for TaskLow.
func (svc *Mock) MockTaskLow(handler func(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)) *Mock { // MARKER: TaskLow
	svc.mockTaskLow = handler
	return svc
}

// TaskLow executes the mock handler.
func (svc *Mock) TaskLow(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error) { // MARKER: TaskLow
	if svc.mockTaskLow != nil {
		branch, err = svc.mockTaskLow(ctx, flow, score)
	}
	return branch, errors.Trace(err)
}

// MockTaskC sets up a mock handler for TaskC.
func (svc *Mock) MockTaskC(handler func(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error)) *Mock { // MARKER: TaskC
	svc.mockTaskC = handler
	return svc
}

// TaskC executes the mock handler.
func (svc *Mock) TaskC(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error) { // MARKER: TaskC
	if svc.mockTaskC != nil {
		finalBranch, err = svc.mockTaskC(ctx, flow, branch)
	}
	return finalBranch, errors.Trace(err)
}

// MockConditional sets up a mock handler for the Conditional workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockConditional(handler func(ctx context.Context, flow *workflow.Flow, score int) (finalBranch string, err error)) *Mock { // MARKER: Conditional
	if svc.unsubMockConditional != nil {
		svc.unsubMockConditional()
		svc.unsubMockConditional = nil
	}
	if handler == nil {
		svc.mockConditionalGraph = nil
		return svc
	}
	mockName := "MockConditional" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-conditional-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockConditionalGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(conditionalflowapi.Conditional.URL())
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
		var in conditionalflowapi.ConditionalIn
		f.ParseState(&in)
		finalBranch, err := handler(r.Context(), &f, in.Score)
		if err != nil {
			return err // No trace
		}
		out := conditionalflowapi.ConditionalOut{FinalBranch: finalBranch}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(conditionalflowapi.ConditionalIn{}, conditionalflowapi.ConditionalOut{}),
	)
	if err == nil {
		svc.unsubMockConditional = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// Conditional returns the workflow graph, or a mocked graph if MockConditional was called.
func (svc *Mock) Conditional(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Conditional
	if svc.mockConditionalGraph != nil {
		graph, err = svc.mockConditionalGraph(ctx)
	}
	return graph, errors.Trace(err)
}
