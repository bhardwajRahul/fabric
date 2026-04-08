/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package foreman

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

var (
	_ *workflow.Flow
	_ foremanapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockCreate      func(ctx context.Context, workflowName string, initialState any) (flowID string, err error) // MARKER: Create
	mockStart       func(ctx context.Context, flowID string) (err error)                                        // MARKER: Start
	mockStartNotify func(ctx context.Context, flowID string, notifyHostname string) (err error)                 // MARKER: StartNotify
	mockSnapshot    func(ctx context.Context, flowID string) (status string, state map[string]any, err error)   // MARKER: Snapshot

	mockResume              func(ctx context.Context, flowID string, resumeData any) (err error)                                    // MARKER: Resume
	mockFork                func(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error) // MARKER: Fork
	mockCancel              func(ctx context.Context, flowID string) (err error)                                                    // MARKER: Cancel
	mockHistory             func(ctx context.Context, flowID string) (steps []foremanapi.FlowStep, err error)                       // MARKER: History
	mockRetry               func(ctx context.Context, flowID string) (err error)                                                    // MARKER: Retry
	mockList                func(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error)           // MARKER: List
	mockCreateTask          func(ctx context.Context, taskName string, initialState any) (flowID string, err error)                 // MARKER: CreateTask
	mockEnqueue             func(ctx context.Context, shard int, stepID int) (err error)                                            // MARKER: Enqueue
	mockAwait               func(ctx context.Context, flowID string) (status string, state map[string]any, err error)               // MARKER: Await
	mockNotifyStatusChange  func(ctx context.Context, flowID string, status string) (err error)                                     // MARKER: NotifyStatusChange
	mockPurgeExpiredFlows   func(ctx context.Context) (err error)                                                                   // MARKER: PurgeExpiredFlows
	mockBreakBefore         func(ctx context.Context, flowID string, taskName string, enabled bool) (err error)                                          // MARKER: BreakBefore
	mockRun                 func(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) // MARKER: Run
	mockContinue            func(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error)                // MARKER: Continue
	mockOnObserveQueueDepth func(ctx context.Context) (err error)                                                                                            // MARKER: QueueDepth
	mockHistoryMermaid      func(w http.ResponseWriter, r *http.Request) (err error)                                                // MARKER: HistoryMermaid
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

// OnChangedNumShards is a no-op in the mock.
func (svc *Mock) OnChangedNumShards(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockCreate sets up a mock handler for Create.
func (svc *Mock) MockCreate(handler func(ctx context.Context, workflowName string, initialState any) (flowID string, err error)) *Mock { // MARKER: Create
	svc.mockCreate = handler
	return svc
}

// Create executes the mock handler.
func (svc *Mock) Create(ctx context.Context, workflowName string, initialState any) (flowID string, err error) { // MARKER: Create
	if svc.mockCreate == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	flowID, err = svc.mockCreate(ctx, workflowName, initialState)
	return flowID, errors.Trace(err)
}

// MockStart sets up a mock handler for Start.
func (svc *Mock) MockStart(handler func(ctx context.Context, flowID string) (err error)) *Mock { // MARKER: Start
	svc.mockStart = handler
	return svc
}

// Start executes the mock handler.
func (svc *Mock) Start(ctx context.Context, flowID string) (err error) { // MARKER: Start
	if svc.mockStart == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockStart(ctx, flowID)
	return errors.Trace(err)
}

// MockStartNotify sets up a mock handler for StartNotify.
func (svc *Mock) MockStartNotify(handler func(ctx context.Context, flowID string, notifyHostname string) (err error)) *Mock { // MARKER: StartNotify
	svc.mockStartNotify = handler
	return svc
}

// StartNotify executes the mock handler.
func (svc *Mock) StartNotify(ctx context.Context, flowID string, notifyHostname string) (err error) { // MARKER: StartNotify
	if svc.mockStartNotify == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockStartNotify(ctx, flowID, notifyHostname)
	return errors.Trace(err)
}

// MockSnapshot sets up a mock handler for Snapshot.
func (svc *Mock) MockSnapshot(handler func(ctx context.Context, flowID string) (status string, state map[string]any, err error)) *Mock { // MARKER: Snapshot
	svc.mockSnapshot = handler
	return svc
}

// Snapshot executes the mock handler.
func (svc *Mock) Snapshot(ctx context.Context, flowID string) (status string, state map[string]any, err error) { // MARKER: Snapshot
	if svc.mockSnapshot == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	status, state, err = svc.mockSnapshot(ctx, flowID)
	return status, state, errors.Trace(err)
}

// MockResume sets up a mock handler for Resume.
func (svc *Mock) MockResume(handler func(ctx context.Context, flowID string, resumeData any) (err error)) *Mock { // MARKER: Resume
	svc.mockResume = handler
	return svc
}

// Resume executes the mock handler.
func (svc *Mock) Resume(ctx context.Context, flowID string, resumeData any) (err error) { // MARKER: Resume
	if svc.mockResume == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockResume(ctx, flowID, resumeData)
	return errors.Trace(err)
}

// MockFork sets up a mock handler for Fork.
func (svc *Mock) MockFork(handler func(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error)) *Mock { // MARKER: Fork
	svc.mockFork = handler
	return svc
}

// Fork executes the mock handler.
func (svc *Mock) Fork(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error) { // MARKER: Fork
	if svc.mockFork == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	newFlowKey, err = svc.mockFork(ctx, stepKey, stateOverrides)
	return newFlowKey, errors.Trace(err)
}

// MockCancel sets up a mock handler for Cancel.
func (svc *Mock) MockCancel(handler func(ctx context.Context, flowID string) (err error)) *Mock { // MARKER: Cancel
	svc.mockCancel = handler
	return svc
}

// Cancel executes the mock handler.
func (svc *Mock) Cancel(ctx context.Context, flowID string) (err error) { // MARKER: Cancel
	if svc.mockCancel == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockCancel(ctx, flowID)
	return errors.Trace(err)
}

// MockHistory sets up a mock handler for History.
func (svc *Mock) MockHistory(handler func(ctx context.Context, flowID string) (steps []foremanapi.FlowStep, err error)) *Mock { // MARKER: History
	svc.mockHistory = handler
	return svc
}

// History executes the mock handler.
func (svc *Mock) History(ctx context.Context, flowID string) (steps []foremanapi.FlowStep, err error) { // MARKER: History
	if svc.mockHistory == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	steps, err = svc.mockHistory(ctx, flowID)
	return steps, errors.Trace(err)
}

// MockRetry sets up a mock handler for Retry.
func (svc *Mock) MockRetry(handler func(ctx context.Context, flowID string) (err error)) *Mock { // MARKER: Retry
	svc.mockRetry = handler
	return svc
}

// Retry executes the mock handler.
func (svc *Mock) Retry(ctx context.Context, flowID string) (err error) { // MARKER: Retry
	if svc.mockRetry == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockRetry(ctx, flowID)
	return errors.Trace(err)
}

// MockList sets up a mock handler for List.
func (svc *Mock) MockList(handler func(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error)) *Mock { // MARKER: List
	svc.mockList = handler
	return svc
}

// List executes the mock handler.
func (svc *Mock) List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error) { // MARKER: List
	if svc.mockList == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	flows, err = svc.mockList(ctx, query)
	return flows, errors.Trace(err)
}

// MockCreateTask sets up a mock handler for CreateTask.
func (svc *Mock) MockCreateTask(handler func(ctx context.Context, taskName string, initialState any) (flowID string, err error)) *Mock { // MARKER: CreateTask
	svc.mockCreateTask = handler
	return svc
}

// CreateTask executes the mock handler.
func (svc *Mock) CreateTask(ctx context.Context, taskName string, initialState any) (flowID string, err error) { // MARKER: CreateTask
	if svc.mockCreateTask == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	flowID, err = svc.mockCreateTask(ctx, taskName, initialState)
	return flowID, errors.Trace(err)
}

// MockEnqueue sets up a mock handler for Enqueue.
func (svc *Mock) MockEnqueue(handler func(ctx context.Context, shard int, stepID int) (err error)) *Mock { // MARKER: Enqueue
	svc.mockEnqueue = handler
	return svc
}

// Enqueue executes the mock handler.
func (svc *Mock) Enqueue(ctx context.Context, shard int, stepID int) (err error) { // MARKER: Enqueue
	if svc.mockEnqueue == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockEnqueue(ctx, shard, stepID)
	return errors.Trace(err)
}

// MockAwait sets up a mock handler for Await.
func (svc *Mock) MockAwait(handler func(ctx context.Context, flowID string) (status string, state map[string]any, err error)) *Mock { // MARKER: Await
	svc.mockAwait = handler
	return svc
}

// Await executes the mock handler.
func (svc *Mock) Await(ctx context.Context, flowID string) (status string, state map[string]any, err error) { // MARKER: Await
	if svc.mockAwait == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	status, state, err = svc.mockAwait(ctx, flowID)
	return status, state, errors.Trace(err)
}

// MockNotifyStatusChange sets up a mock handler for NotifyStatusChange.
func (svc *Mock) MockNotifyStatusChange(handler func(ctx context.Context, flowID string, status string) (err error)) *Mock { // MARKER: NotifyStatusChange
	svc.mockNotifyStatusChange = handler
	return svc
}

// NotifyStatusChange executes the mock handler.
func (svc *Mock) NotifyStatusChange(ctx context.Context, flowID string, status string) (err error) { // MARKER: NotifyStatusChange
	if svc.mockNotifyStatusChange == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockNotifyStatusChange(ctx, flowID, status)
	return errors.Trace(err)
}

// MockPurgeExpiredFlows sets up a mock handler for PurgeExpiredFlows.
func (svc *Mock) MockPurgeExpiredFlows(handler func(ctx context.Context) (err error)) *Mock { // MARKER: PurgeExpiredFlows
	svc.mockPurgeExpiredFlows = handler
	return svc
}

// PurgeExpiredFlows executes the mock handler.
func (svc *Mock) PurgeExpiredFlows(ctx context.Context) (err error) { // MARKER: PurgeExpiredFlows
	if svc.mockPurgeExpiredFlows == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockPurgeExpiredFlows(ctx)
	return errors.Trace(err)
}

// MockBreakBefore sets up a mock handler for BreakBefore.
func (svc *Mock) MockBreakBefore(handler func(ctx context.Context, flowID string, taskName string, enabled bool) (err error)) *Mock { // MARKER: BreakBefore
	svc.mockBreakBefore = handler
	return svc
}

// BreakBefore executes the mock handler.
func (svc *Mock) BreakBefore(ctx context.Context, flowID string, taskName string, enabled bool) (err error) { // MARKER: BreakBefore
	if svc.mockBreakBefore == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockBreakBefore(ctx, flowID, taskName, enabled)
	return errors.Trace(err)
}

// MockContinue sets up a mock handler for Continue.
func (svc *Mock) MockContinue(handler func(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error)) *Mock { // MARKER: Continue
	svc.mockContinue = handler
	return svc
}

// Continue executes the mock handler.
func (svc *Mock) Continue(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error) { // MARKER: Continue
	if svc.mockContinue == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	newFlowKey, err = svc.mockContinue(ctx, threadKey, additionalState)
	return newFlowKey, errors.Trace(err)
}

// MockRun sets up a mock handler for Run.
func (svc *Mock) MockRun(handler func(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error)) *Mock { // MARKER: Run
	svc.mockRun = handler
	return svc
}

// Run executes the mock handler.
func (svc *Mock) Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) { // MARKER: Run
	if svc.mockRun == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	status, state, err = svc.mockRun(ctx, workflowName, initialState)
	return status, state, errors.Trace(err)
}

// MockOnObserveQueueDepth sets up a mock handler for OnObserveQueueDepth.
func (svc *Mock) MockOnObserveQueueDepth(handler func(ctx context.Context) (err error)) *Mock { // MARKER: QueueDepth
	svc.mockOnObserveQueueDepth = handler
	return svc
}

// OnObserveQueueDepth executes the mock handler.
func (svc *Mock) OnObserveQueueDepth(ctx context.Context) (err error) { // MARKER: QueueDepth
	if svc.mockOnObserveQueueDepth == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnObserveQueueDepth(ctx)
	return errors.Trace(err)
}

// MockHistoryMermaid sets up a mock handler for HistoryMermaid.
func (svc *Mock) MockHistoryMermaid(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: HistoryMermaid
	svc.mockHistoryMermaid = handler
	return svc
}

// HistoryMermaid executes the mock handler.
func (svc *Mock) HistoryMermaid(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HistoryMermaid
	if svc.mockHistoryMermaid == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockHistoryMermaid(w, r)
	return errors.Trace(err)
}
