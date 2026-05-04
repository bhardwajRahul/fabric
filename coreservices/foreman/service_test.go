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
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

var (
	_ context.Context
	_ *testing.T
	_ application.Application
	_ connector.Connector
	_ foremanapi.Client
	_ workflow.Flow
)

func TestForeman_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("create", func(t *testing.T) { // MARKER: Create
		assert := testarossa.For(t)

		_, err := mock.Create(ctx, "test-workflow", nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockCreate(func(ctx context.Context, workflowName string, initialState any) (flowKey string, err error) {
			return "test-flow-id", nil
		})
		flowKey, err := mock.Create(ctx, "test-workflow", nil)
		assert.Expect(
			flowKey, "test-flow-id",
			err, nil,
		)
	})

	t.Run("start", func(t *testing.T) { // MARKER: Start
		assert := testarossa.For(t)

		err := mock.Start(ctx, "test-flow-id")
		assert.Contains(err.Error(), "not implemented")
		mock.MockStart(func(ctx context.Context, flowKey string) (err error) {
			return nil
		})
		err = mock.Start(ctx, "test-flow-id")
		assert.NoError(err)
	})

	t.Run("start_notify", func(t *testing.T) { // MARKER: StartNotify
		assert := testarossa.For(t)

		err := mock.StartNotify(ctx, "test-flow-id", "my.caller.host")
		assert.Contains(err.Error(), "not implemented")
		mock.MockStartNotify(func(ctx context.Context, flowKey string, notifyHostname string) (err error) {
			return nil
		})
		err = mock.StartNotify(ctx, "test-flow-id", "my.caller.host")
		assert.NoError(err)
	})

	t.Run("flow_snapshot", func(t *testing.T) { // MARKER: Snapshot
		assert := testarossa.For(t)

		_, _, err := mock.Snapshot(ctx, "test-flow-id")
		assert.Contains(err.Error(), "not implemented")
		mock.MockSnapshot(func(ctx context.Context, flowKey string) (status string, state map[string]any, err error) {
			return "completed", map[string]any{"x": 1}, nil
		})
		status, state, err := mock.Snapshot(ctx, "test-flow-id")
		assert.Expect(
			state["x"], 1,
			status, "completed",
			err, nil,
		)
	})

	t.Run("resume", func(t *testing.T) { // MARKER: Resume
		assert := testarossa.For(t)

		err := mock.Resume(ctx, "test-flow-id", nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockResume(func(ctx context.Context, flowKey string, resumeData any) (err error) {
			return nil
		})
		err = mock.Resume(ctx, "test-flow-id", map[string]any{"answer": 42})
		assert.NoError(err)
	})

	t.Run("cancel", func(t *testing.T) { // MARKER: Cancel
		assert := testarossa.For(t)

		err := mock.Cancel(ctx, "test-flow-id")
		assert.Contains(err.Error(), "not implemented")
		mock.MockCancel(func(ctx context.Context, flowKey string) (err error) {
			return nil
		})
		err = mock.Cancel(ctx, "test-flow-id")
		assert.NoError(err)
	})

	t.Run("purge_expired_flows", func(t *testing.T) { // MARKER: PurgeExpiredFlows
		assert := testarossa.For(t)

		err := mock.PurgeExpiredFlows(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockPurgeExpiredFlows(func(ctx context.Context) (err error) {
			return nil
		})
		err = mock.PurgeExpiredFlows(ctx)
		assert.NoError(err)
	})

	t.Run("create_task", func(t *testing.T) { // MARKER: CreateTask
		assert := testarossa.For(t)

		_, err := mock.CreateTask(ctx, "svc:428/my-task", nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockCreateTask(func(ctx context.Context, taskName string, initialState any) (flowKey string, err error) {
			return "test-flow-id", nil
		})
		flowKey, err := mock.CreateTask(ctx, "svc:428/my-task", nil)
		assert.Expect(
			flowKey, "test-flow-id",
			err, nil,
		)
	})

	t.Run("wait_for_stop", func(t *testing.T) { // MARKER: Await
		assert := testarossa.For(t)

		_, _, err := mock.Await(ctx, "test-flow-id")
		assert.Contains(err.Error(), "not implemented")
		mock.MockAwait(func(ctx context.Context, flowKey string) (status string, state map[string]any, err error) {
			return foremanapi.StatusCompleted, map[string]any{"result": "done"}, nil
		})
		status, state, err := mock.Await(ctx, "test-flow-id")
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "done",
			err, nil,
		)
	})

	t.Run("notify_status_change", func(t *testing.T) { // MARKER: NotifyStatusChange
		assert := testarossa.For(t)

		err := mock.NotifyStatusChange(ctx, "test-flow-id", foremanapi.StatusCompleted)
		assert.Contains(err.Error(), "not implemented")
		mock.MockNotifyStatusChange(func(ctx context.Context, flowKey string, status string) (err error) {
			return nil
		})
		err = mock.NotifyStatusChange(ctx, "test-flow-id", foremanapi.StatusCompleted)
		assert.NoError(err)
	})

	t.Run("flow_history", func(t *testing.T) { // MARKER: History
		assert := testarossa.For(t)

		_, err := mock.History(ctx, "test-flow-id")
		assert.Contains(err.Error(), "not implemented")
		mock.MockHistory(func(ctx context.Context, flowKey string) (steps []foremanapi.FlowStep, err error) {
			return []foremanapi.FlowStep{
				{StepDepth: 1, TaskName: "task-1", Status: "completed"},
			}, nil
		})
		steps, err := mock.History(ctx, "test-flow-id")
		assert.Expect(
			err, nil,
			len(steps), 1,
			steps[0].StepDepth, 1,
			steps[0].TaskName, "task-1",
			steps[0].Status, "completed",
		)
	})

	t.Run("continue", func(t *testing.T) { // MARKER: Continue
		assert := testarossa.For(t)

		_, err := mock.Continue(ctx, "0-1-abc", nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockContinue(func(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error) {
			return "0-2-def", nil
		})
		newFlowKey, err := mock.Continue(ctx, "0-1-abc", nil)
		assert.Expect(
			newFlowKey, "0-2-def",
			err, nil,
		)
	})
}

func TestForeman_LowLevel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	db, err := svc.shard(0)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("create", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, map[string]any{"input": "value"})
		assert.NoError(err)
		shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
		assert.NoError(err)
		assert.Equal(0, shardNum)

		// Verify flow row
		var status, workflowName, graphJSON, actorClaimsJSON, traceParent, notifyHostname, finalState, breakpointsJSON, threadToken string
		var stepID, forkedFlowID, forkedStepDepth, surgraphFlowID, surgraphStepDepth, threadID int
		err = db.QueryRowContext(ctx,
			"SELECT flow_token, workflow_name, graph, actor_claims, status, step_id, forked_flow_id, forked_step_depth, surgraph_flow_id, surgraph_step_depth, thread_id, thread_token, trace_parent, notify_hostname, final_state, breakpoints FROM microbus_flows WHERE flow_id=?",
			flowID,
		).Scan(&flowToken, &workflowName, &graphJSON, &actorClaimsJSON, &status, &stepID, &forkedFlowID, &forkedStepDepth, &surgraphFlowID, &surgraphStepDepth, &threadID, &threadToken, &traceParent, &notifyHostname, &finalState, &breakpointsJSON)
		assert.NoError(err)
		assert.Expect(
			len(flowToken) > 0, true,
			workflowName, testWorkflowURL,
			status, foremanapi.StatusCreated,
			stepID > 0, true,
			forkedFlowID, 0,
			forkedStepDepth, 0,
			surgraphFlowID, 0,
			surgraphStepDepth, 0,
			threadID, flowID,
			strings.TrimSpace(threadToken), strings.TrimSpace(flowToken),
			notifyHostname, "",
			finalState, "{}",
			breakpointsJSON, "{}",
		)
		// Graph should be valid JSON
		var g workflow.Graph
		assert.NoError(json.Unmarshal([]byte(graphJSON), &g))
		assert.Equal(testWorkflowURL, g.Name())

		// Verify step row
		var stepDepth, timeBudgetMs, breakpointHit, attempt int
		var stepToken, taskName, stateJSON, changesJSON, interruptPayloadJSON, stepStatus, gotoNext, stepError string
		err = db.QueryRowContext(ctx,
			"SELECT step_depth, step_token, task_name, state, changes, interrupt_payload, status, goto_next, error, time_budget_ms, breakpoint_hit, attempt FROM microbus_steps WHERE flow_id=?",
			flowID,
		).Scan(&stepDepth, &stepToken, &taskName, &stateJSON, &changesJSON, &interruptPayloadJSON, &stepStatus, &gotoNext, &stepError, &timeBudgetMs, &breakpointHit, &attempt)
		assert.NoError(err)
		assert.Expect(
			stepDepth, 1,
			len(stepToken) > 0, true,
			taskName, "https://test.workflow.host:428/task-a",
			stepStatus, foremanapi.StatusCreated,
			changesJSON, "{}",
			interruptPayloadJSON, "{}",
			gotoNext, "",
			stepError, "",
			breakpointHit, 0,
			attempt, 0,
			timeBudgetMs > 0, true,
		)
		// State should contain the initial input
		var state map[string]any
		assert.NoError(json.Unmarshal([]byte(stateJSON), &state))
		assert.Equal("value", state["input"])
	})

	t.Run("start_and_complete", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, map[string]any{"input": "test"})
		assert.NoError(err)
		_, flowID, _, err := parseFlowKey(flowKey)
		assert.NoError(err)

		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// Verify flow transitioned to running
		var status string
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&status)
		assert.Equal(foremanapi.StatusRunning, status)

		// Wait for completion
		status, state, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)

		// Verify flow row is completed with filtered final_state
		var finalStateJSON string
		db.QueryRowContext(ctx, "SELECT status, final_state FROM microbus_flows WHERE flow_id=?", flowID).Scan(&status, &finalStateJSON)
		assert.Equal(foremanapi.StatusCompleted, status)
		var finalState map[string]any
		assert.NoError(json.Unmarshal([]byte(finalStateJSON), &finalState))
		assert.Equal("hello world", finalState["result"])
		// DeclareOutputs("result") should have filtered out "input"
		_, hasInput := finalState["input"]
		assert.False(hasInput)

		// Verify all steps are completed
		var stepCount int
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND status=?", flowID, foremanapi.StatusCompleted).Scan(&stepCount)
		assert.Equal(2, stepCount)
	})

	t.Run("start_notify", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, nil)
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.StartNotify(ctx, flowKey, "my.notify.host")
		assert.NoError(err)

		// Verify notify_hostname was set
		var notifyHostname string
		db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", flowID).Scan(&notifyHostname)
		assert.Equal("my.notify.host", notifyHostname)
	})

	t.Run("cancel", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, nil)
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.Cancel(ctx, flowKey)
		assert.NoError(err)

		// Verify flow and step are cancelled
		var flowStatus, stepStatus string
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(foremanapi.StatusCancelled, flowStatus)
		db.QueryRowContext(ctx, "SELECT status FROM microbus_steps WHERE flow_id=? ORDER BY step_id DESC LIMIT_OFFSET(1, 0)", flowID).Scan(&stepStatus)
		assert.Equal(foremanapi.StatusCancelled, stepStatus)
	})

	t.Run("break_before", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, nil)
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.BreakBefore(ctx, flowKey, "https://test.workflow.host:428/task-b", true)
		assert.NoError(err)

		// Verify breakpoint was stored
		var breakpointsJSON string
		db.QueryRowContext(ctx, "SELECT breakpoints FROM microbus_flows WHERE flow_id=?", flowID).Scan(&breakpointsJSON)
		var breakpoints map[string]string
		assert.NoError(json.Unmarshal([]byte(breakpointsJSON), &breakpoints))
		assert.Equal("b", breakpoints["https://test.workflow.host:428/task-b"])

		// Start and verify it interrupts at task-b
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		status, _, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(foremanapi.StatusInterrupted, status)

		// Verify the interrupted step has breakpoint_hit=1
		var breakpointHit int
		db.QueryRowContext(ctx,
			"SELECT breakpoint_hit FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, foremanapi.StatusInterrupted,
		).Scan(&breakpointHit)
		assert.Equal(1, breakpointHit)
	})

	t.Run("interrupt_and_resume", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create and start a flow that will interrupt at task-b
		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, map[string]any{
			"needInput": true,
		})
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		status, _, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(foremanapi.StatusInterrupted, status)

		// Verify flow is interrupted
		var flowStatus string
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(foremanapi.StatusInterrupted, flowStatus)

		// Verify the interrupted step has the interrupt payload
		var stepStatus, interruptPayloadJSON string
		db.QueryRowContext(ctx,
			"SELECT status, interrupt_payload FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, foremanapi.StatusInterrupted,
		).Scan(&stepStatus, &interruptPayloadJSON)
		assert.Equal(foremanapi.StatusInterrupted, stepStatus)
		var payload map[string]any
		assert.NoError(json.Unmarshal([]byte(interruptPayloadJSON), &payload))
		assert.Equal("more data", payload["request"])

		// Resume with needInput=false so task-b completes normally
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"needInput": false})
		assert.NoError(err)
		status, state, err := foremanClient.Await(ctx, flowKey)
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				state["result"], "hello world",
			)
		}

		// Verify flow is completed in the database
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(foremanapi.StatusCompleted, flowStatus)
	})

	t.Run("fork", func(t *testing.T) {
		assert := testarossa.For(t)

		// Run to completion
		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, map[string]any{"input": "original"})
		assert.NoError(err)
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		status, _, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(foremanapi.StatusCompleted, status)

		// Get history to find a step key
		steps, err := foremanClient.History(ctx, flowKey)
		if !assert.NoError(err) || !assert.True(len(steps) >= 1) {
			return
		}
		stepKey := steps[0].StepKey // First step (task-a)

		// Fork from that step with state overrides
		forkedFlowKey, err := foremanClient.Fork(ctx, stepKey, map[string]any{"input": "forked"})
		assert.NoError(err)
		_, forkedFlowID, _, _ := parseFlowKey(forkedFlowKey)

		// Verify forked flow was created with lineage
		var forkedStatus, forkedWorkflowName string
		var forkedFlowIDRef, forkedStepDepth int
		db.QueryRowContext(ctx,
			"SELECT status, workflow_name, forked_flow_id, forked_step_depth FROM microbus_flows WHERE flow_id=?",
			forkedFlowID,
		).Scan(&forkedStatus, &forkedWorkflowName, &forkedFlowIDRef, &forkedStepDepth)
		_, origFlowID, _, _ := parseFlowKey(flowKey)
		assert.Expect(
			forkedStatus, foremanapi.StatusCreated,
			forkedWorkflowName, testWorkflowURL,
			forkedFlowIDRef, origFlowID,
			forkedStepDepth, steps[0].StepDepth,
		)

		// Verify the forked step's state contains the override
		var stateJSON string
		db.QueryRowContext(ctx, "SELECT state FROM microbus_steps WHERE flow_id=? AND step_depth=?", forkedFlowID, steps[0].StepDepth).Scan(&stateJSON)
		var state map[string]any
		assert.NoError(json.Unmarshal([]byte(stateJSON), &state))
		assert.Equal("forked", state["input"])

		// Run the forked flow and verify it completes
		err = foremanClient.Start(ctx, forkedFlowKey)
		assert.NoError(err)
		status, fState, err := foremanClient.Await(ctx, forkedFlowKey)
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				fState["result"], "hello world",
			)
		}
	})

	t.Run("continue", func(t *testing.T) {
		assert := testarossa.For(t)

		// Run to completion
		flowKey, err := foremanClient.Create(ctx, testWorkflowURL, map[string]any{"input": "first"})
		assert.NoError(err)
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		status, _, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(foremanapi.StatusCompleted, status)

		_, firstFlowID, firstFlowToken, _ := parseFlowKey(flowKey)

		// Verify the first flow's thread_id is its own flow_id and thread_token matches flow_token
		var threadID int
		var threadToken string
		db.QueryRowContext(ctx, "SELECT thread_id, thread_token FROM microbus_flows WHERE flow_id=?", firstFlowID).Scan(&threadID, &threadToken)
		assert.Equal(firstFlowID, threadID)
		assert.Equal(firstFlowToken, strings.TrimSpace(threadToken))

		// Continue with additional state
		newFlowKey, err := foremanClient.Continue(ctx, flowKey, map[string]any{"extra": "data"})
		assert.NoError(err)
		_, newFlowID, _, _ := parseFlowKey(newFlowKey)

		// Verify new flow was created with merged state
		var newStatus, newWorkflowName string
		db.QueryRowContext(ctx, "SELECT status, workflow_name FROM microbus_flows WHERE flow_id=?", newFlowID).Scan(&newStatus, &newWorkflowName)
		assert.Expect(
			newStatus, foremanapi.StatusCreated,
			newWorkflowName, testWorkflowURL,
		)

		// Verify the new flow shares the same thread_id and thread_token as the first flow
		var newThreadID int
		var newThreadToken string
		db.QueryRowContext(ctx, "SELECT thread_id, thread_token FROM microbus_flows WHERE flow_id=?", newFlowID).Scan(&newThreadID, &newThreadToken)
		assert.Equal(firstFlowID, newThreadID)
		assert.Equal(firstFlowToken, strings.TrimSpace(newThreadToken))

		// Verify the initial step's state contains both the previous output and additional state
		var stateJSON string
		db.QueryRowContext(ctx, "SELECT state FROM microbus_steps WHERE flow_id=? AND step_depth=1", newFlowID).Scan(&stateJSON)
		var state map[string]any
		assert.NoError(json.Unmarshal([]byte(stateJSON), &state))
		assert.Equal("hello world", state["result"]) // carried from previous flow's final_state
		assert.Equal("data", state["extra"])         // from additional state
	})
}

const testWorkflowURL = "https://test.workflow.host:428/my-workflow"

// newTestWorkflowSvc creates a minimal workflow service (taskA -> taskB -> END).
// taskA sets result="hello". taskB appends " world" unless needInput=true (then it interrupts).
func newTestWorkflowSvc() *connector.Connector {
	graphSvc := connector.New("test.workflow.host")
	graphSvc.Subscribe("MyWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(testWorkflowURL)
			g.DeclareInputs("*")
			g.DeclareOutputs("result")
			g.AddTransition("https://test.workflow.host:428/task-a", "https://test.workflow.host:428/task-b")
			g.AddTransition("https://test.workflow.host:428/task-b", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/my-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("TaskA",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("result", "hello")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-a"),
		sub.Web(),
	)
	graphSvc.Subscribe("TaskB",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			if f.GetBool("needInput") {
				f.Interrupt(map[string]any{"request": "more data"})
				return json.NewEncoder(w).Encode(&f)
			}
			f.SetString("result", f.GetString("result")+" world")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-b"),
		sub.Web(),
	)
	return graphSvc
}

func TestForeman_Create(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, map[string]any{"x": 1})
	if assert.NoError(err) {
		assert.True(flowKey != "")
	}

	// Invalid workflow name
	_, err = client.Create(ctx, "", nil)
	assert.Error(err)
}

func TestForeman_Start(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, _, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCompleted, status)
}

func TestForeman_StartNotify(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.StartNotify(ctx, flowKey, "my.notify.host")
	assert.NoError(err)
	status, _, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCompleted, status)
}

func TestForeman_Cancel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Cancel(ctx, flowKey)
	assert.NoError(err)

	status, _, err := client.Snapshot(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCancelled, status)
}

func TestForeman_Resume(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	// Start a flow that interrupts at task-b
	flowKey, err := client.Create(ctx, testWorkflowURL, map[string]any{"needInput": true})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, _, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusInterrupted, status)

	// Resume
	err = client.Resume(ctx, flowKey, map[string]any{"needInput": false})
	assert.NoError(err)
	status, state, err := client.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)
	}
}

func TestForeman_Fork(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	// Run to completion
	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	client.Await(ctx, flowKey)

	// Fork from first step
	steps, err := client.History(ctx, flowKey)
	if !assert.NoError(err) || !assert.True(len(steps) >= 1) {
		return
	}
	forkedKey, err := client.Fork(ctx, steps[0].StepKey, map[string]any{"input": "forked"})
	if !assert.NoError(err) {
		return
	}

	// Run the forked flow
	err = client.Start(ctx, forkedKey)
	assert.NoError(err)
	status, state, err := client.Await(ctx, forkedKey)
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)
	}
}

func TestForeman_BreakBefore(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.BreakBefore(ctx, flowKey, "https://test.workflow.host:428/task-b", true)
	assert.NoError(err)

	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, _, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusInterrupted, status)

	// Resume past the breakpoint
	err = client.Resume(ctx, flowKey, nil)
	assert.NoError(err)
	status, state, err := client.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)
	}
}

func TestForeman_CreateTask(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.CreateTask(ctx, "https://test.workflow.host:428/task-a", map[string]any{"input": "test"})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, state, err := client.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Equal(foremanapi.StatusCompleted, status)
		assert.Equal("hello", state["result"])
	}
}

func TestForeman_Snapshot(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, map[string]any{"input": "test"})
	assert := testarossa.For(t)
	if !assert.NoError(err) {
		return
	}

	// Snapshot of created flow
	status, _, err := client.Snapshot(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCreated, status)

	// Run to completion and snapshot again
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	client.Await(ctx, flowKey)
	status, state, err := client.Snapshot(ctx, flowKey)
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)
	}
}

func TestForeman_History(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	assert := testarossa.For(t)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	client.Await(ctx, flowKey)

	steps, err := client.History(ctx, flowKey)
	if assert.NoError(err) {
		assert.Equal(2, len(steps))
		assert.Equal("https://test.workflow.host:428/task-a", steps[0].TaskName)
		assert.Equal(foremanapi.StatusCompleted, steps[0].Status)
		assert.Equal("https://test.workflow.host:428/task-b", steps[1].TaskName)
		assert.Equal(foremanapi.StatusCompleted, steps[1].Status)
		assert.True(steps[0].StepKey != "")
		assert.True(steps[1].StepKey != "")
	}
}

func TestForeman_List(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	// Create two flows
	flowKey1, _ := client.Create(ctx, testWorkflowURL, nil)
	flowKey2, _ := client.Create(ctx, testWorkflowURL, nil)
	assert := testarossa.For(t)

	// List created flows
	flows, err := client.List(ctx, foremanapi.Query{Status: foremanapi.StatusCreated})
	if assert.NoError(err) {
		assert.True(len(flows) >= 2)
	}

	// Complete one and list again
	client.Start(ctx, flowKey1)
	client.Await(ctx, flowKey1)

	flows, err = client.List(ctx, foremanapi.Query{Status: foremanapi.StatusCompleted})
	if assert.NoError(err) {
		assert.True(len(flows) >= 1)
	}

	// Cancel the other
	client.Cancel(ctx, flowKey2)
	flows, err = client.List(ctx, foremanapi.Query{Status: foremanapi.StatusCancelled})
	if assert.NoError(err) {
		assert.True(len(flows) >= 1)
	}
}

func TestForeman_Retry(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Set up a workflow with a task that fails once then succeeds
	graphSvc := connector.New("test.fail.host")
	graphSvc.Subscribe("FailWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.fail.host:428/fail-workflow")
			g.DeclareInputs("*")
			g.DeclareOutputs("result")
			g.AddTransition("https://test.fail.host:428/fail-task", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/fail-workflow"),
		sub.Web(),
	)
	// This task fails on first attempt, succeeds on retry
	var failOnce bool
	graphSvc.Subscribe("FailTask",
		func(w http.ResponseWriter, r *http.Request) error {
			if !failOnce {
				failOnce = true
				return errors.New("transient error")
			}
			var f workflow.Flow
			json.NewDecoder(r.Body).Decode(&f)
			f.SetString("result", "recovered")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/fail-task"),
		sub.Web(),
	)

	tester := connector.New("tester.retry")
	app := application.New()
	svc := NewService()
	app.Add(svc, graphSvc, tester)
	app.RunInTest(t)
	retryClient := foremanapi.NewClient(tester)

	flowKey, err := retryClient.Create(ctx, "https://test.fail.host:428/fail-workflow", nil)
	if !assert.NoError(err) {
		return
	}
	err = retryClient.Start(ctx, flowKey)
	assert.NoError(err)
	status, _, err := retryClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusFailed, status)

	// Retry the failed step
	err = retryClient.Retry(ctx, flowKey)
	assert.NoError(err)
	status, state, err := retryClient.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "recovered",
		)
	}
}

func TestForeman_Run(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	status, state, err := client.Run(ctx, testWorkflowURL, map[string]any{"input": "test"})
	if assert.NoError(err) {
		assert.Expect(
			status, foremanapi.StatusCompleted,
			state["result"], "hello world",
		)
	}
}

func TestForeman_Continue(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	// Run first flow
	flowKey, err := client.Create(ctx, testWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, _, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCompleted, status)

	// The flowKey returned by Create is also the threadKey
	threadKey := flowKey

	// Continue using the threadKey (which is the first flow's key)
	newFlowKey, err := client.Continue(ctx, threadKey, map[string]any{"extra": "data"})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, newFlowKey)
	assert.NoError(err)
	status, state, err := client.Await(ctx, newFlowKey)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(foremanapi.StatusCompleted, status)
	assert.Equal("hello world", state["result"])

	// Continue again using the original threadKey (not the intermediate flowKey)
	thirdFlowKey, err := client.Continue(ctx, threadKey, map[string]any{"turn": 3})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, thirdFlowKey)
	assert.NoError(err)
	status, _, err = client.Await(ctx, thirdFlowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCompleted, status)

	// Continue using an intermediate flowKey (should also work)
	fourthFlowKey, err := client.Continue(ctx, newFlowKey, map[string]any{"turn": 4})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, fourthFlowKey)
	assert.NoError(err)
	status, _, err = client.Await(ctx, fourthFlowKey)
	assert.NoError(err)
	assert.Equal(foremanapi.StatusCompleted, status)

	// List by thread should show all 4 flows
	flows, err := client.List(ctx, foremanapi.Query{ThreadKey: threadKey})
	if assert.NoError(err) {
		assert.Equal(4, len(flows))
		// All should share the same ThreadKey
		for _, f := range flows {
			assert.Equal(threadKey, f.ThreadKey)
		}
	}
}

const testErrorWorkflowURL = "https://test.error.host:428/error-workflow"

// newTestErrorWorkflowSvc creates a workflow where taskA fails if failTask=true,
// routing to an error handler that captures the error. Otherwise taskA succeeds to taskB.
// taskA -> taskB -> END (happy path)
// taskA -> errorHandler -> END (error path)
func newTestErrorWorkflowSvc() *connector.Connector {
	svc := connector.New("test.error.host")
	svc.Subscribe("ErrorWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			taskA := "https://test.error.host:428/task-a"
			taskB := "https://test.error.host:428/task-b"
			errorHandler := "https://test.error.host:428/error-handler"

			g := workflow.NewGraph(testErrorWorkflowURL)
			g.DeclareInputs("*")
			g.DeclareOutputs("*")
			g.AddTransition(taskA, taskB)
			g.AddTransition(taskB, workflow.END)
			g.AddErrorTransition(taskA, errorHandler)
			g.AddTransition(errorHandler, workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/error-workflow"),
		sub.Web(),
	)
	svc.Subscribe("TaskA",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			if f.GetBool("failTask") {
				return errors.New("task-a failed intentionally", http.StatusInternalServerError)
			}
			f.SetString("result", "success")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-a"),
		sub.Web(),
	)
	svc.Subscribe("TaskB",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("result", f.GetString("result")+" via task-b")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-b"),
		sub.Web(),
	)
	svc.Subscribe("ErrorHandler",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			// Read the error from onErr state field as a TracedError
			var onErr errors.TracedError
			f.Get("onErr", &onErr)
			f.SetString("result", "handled: "+onErr.Error())
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/error-handler"),
		sub.Web(),
	)
	return svc
}

func TestForeman_ErrorTransition(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestErrorWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	// Happy path: taskA succeeds, goes to taskB
	flowKey, err := client.Create(ctx, testErrorWorkflowURL, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, state, err := client.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Equal(foremanapi.StatusCompleted, status)
		assert.Equal("success via task-b", state["result"])
	}

	// Error path: taskA fails, routes to errorHandler
	flowKey, err = client.Create(ctx, testErrorWorkflowURL, map[string]any{"failTask": true})
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	status, state, err = client.Await(ctx, flowKey)
	if assert.NoError(err) {
		assert.Equal(foremanapi.StatusCompleted, status)
		assert.Equal("handled: task-a failed intentionally", state["result"])
	}
}

// TestForeman_SubgraphFanInRace exercises the bug where completeSurgraphFlow could
// match a sibling step (still RUNNING during fan-in processing) instead of the parked
// surgraph step, because both share the same flow_id and step_depth.
//
// Setup:
//   - The main workflow fans out at one step_depth to a slow sibling task and a subgraph.
//   - The slow sibling sleeps long enough that it is still status=running when the
//     subgraph completes and completeSurgraphFlow runs.
//   - The slow sibling is registered first in the graph so it gets the lower step_id;
//     completeSurgraphFlow's SELECT (no ORDER BY) will return it before the surgraph step.
//
// Without the lease_expires filter in completeSurgraphFlow, the SELECT picks the slow
// sibling, the surgraph step stays parked forever, and the workflow never completes.
// With the fix, only the parked surgraph step (lease >> 1 hour) matches.
func TestForeman_SubgraphFanInRace(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	const (
		host           = "test.race.host"
		mainWorkflow   = "https://test.race.host:428/main-workflow"
		subWorkflow    = "https://test.race.host:428/sub-workflow"
		startTask      = "https://test.race.host:428/start"
		slowTask       = "https://test.race.host:428/slow-task"
		finalTask      = "https://test.race.host:428/final-task"
		subTask        = "https://test.race.host:428/sub-task"
		slowTaskSleep  = 100 * time.Millisecond
		expectedResult = "done"
	)

	graphSvc := connector.New(host)

	// Subgraph: a single instant task -> END.
	graphSvc.Subscribe("SubWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(subWorkflow)
			g.DeclareInputs("*")
			g.AddTransition(subTask, workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/sub-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("SubTask",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/sub-task"),
		sub.Web(),
	)

	// Main workflow: start fans out to {slow-task, sub-workflow}; both fan in to final-task.
	// slow-task is registered FIRST so it gets the lower step_id.
	graphSvc.Subscribe("MainWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(mainWorkflow)
			g.DeclareInputs("*")
			g.DeclareOutputs("result")
			g.AddSubgraph(subWorkflow)
			g.AddTransition(startTask, slowTask)
			g.AddTransition(startTask, subWorkflow)
			g.AddTransition(slowTask, finalTask)
			g.AddTransition(subWorkflow, finalTask)
			g.AddTransition(finalTask, workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/main-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("Start",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/start"),
		sub.Web(),
	)
	graphSvc.Subscribe("SlowTask",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			// Sleep so the step row stays status=running while the subgraph completes
			// and completeSurgraphFlow runs its SELECT.
			time.Sleep(slowTaskSleep)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/slow-task"),
		sub.Web(),
	)
	graphSvc.Subscribe("FinalTask",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("result", expectedResult)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/final-task"),
		sub.Web(),
	)

	tester := connector.New("tester.race")
	app := application.New()
	svc := NewService()
	app.Add(svc, graphSvc, tester)
	app.RunInTest(t)
	client := foremanapi.NewClient(tester)

	// Generous timeout to absorb scheduling jitter when run alongside other parallel tests.
	// A healthy run completes in well under a second; the bug version hangs indefinitely.
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	status, state, err := client.Run(runCtx, mainWorkflow, nil)
	if !assert.NoError(err) {
		return
	}
	assert.Expect(
		status, foremanapi.StatusCompleted,
		state["result"], expectedResult,
	)
}

// TestForeman_MultipleParallelSubgraphs exercises the case where multiple subgraphs are
// parked at the same flow_id + step_depth (static fan-out to two subgraph children).
// Each parked surgraph step has a long lease, so the previous lease-threshold filter could
// still match the wrong one. The PK lookup via surgraph_step_id is required for correctness.
func TestForeman_MultipleParallelSubgraphs(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	const (
		host          = "test.parsubs.host"
		mainWorkflow  = "https://test.parsubs.host:428/main-workflow"
		subA          = "https://test.parsubs.host:428/sub-a"
		subB          = "https://test.parsubs.host:428/sub-b"
		taskA         = "https://test.parsubs.host:428/task-a"
		taskB         = "https://test.parsubs.host:428/task-b"
		startTask     = "https://test.parsubs.host:428/start"
		finalTask     = "https://test.parsubs.host:428/final-task"
		expectedValue = "ok"
	)

	graphSvc := connector.New(host)

	// Register each subgraph (workflow definition + its single task).
	// Asymmetric delays force the second-registered subgraph (lower-priority insert order
	// in the parent's fan-out) to complete first. Each subgraph writes to a distinct output
	// field so we can verify both completions reached the parent state correctly - if the
	// wrong surgraph step is matched on completion, one or both outputs go missing.
	registerSub := func(name, workflowRoute, taskRoute, workflowURL, taskURL, outputField, outputValue string, delay time.Duration) {
		graphSvc.Subscribe(name+"Workflow",
			func(w http.ResponseWriter, r *http.Request) error {
				g := workflow.NewGraph(workflowURL)
				g.DeclareInputs("*")
				g.DeclareOutputs(outputField)
				g.AddTransition(taskURL, workflow.END)
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(map[string]any{"graph": g})
			},
			sub.At("GET", workflowRoute),
			sub.Web(),
		)
		graphSvc.Subscribe(name+"Task",
			func(w http.ResponseWriter, r *http.Request) error {
				var f workflow.Flow
				if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
					return err
				}
				if delay > 0 {
					time.Sleep(delay)
				}
				f.SetString(outputField, outputValue)
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(&f)
			},
			sub.At("POST", taskRoute),
			sub.Web(),
		)
	}
	// SubA inserted first into the parent's fan-out (lower step_id) but slow to complete.
	// SubB inserted second (higher step_id) but completes immediately.
	registerSub("SubA", ":428/sub-a", ":428/task-a", subA, taskA, "outA", "from-A", 100*time.Millisecond)
	registerSub("SubB", ":428/sub-b", ":428/task-b", subB, taskB, "outB", "from-B", 0)

	// Main: start fans out to both subgraphs; both fan in to final-task.
	graphSvc.Subscribe("MainWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(mainWorkflow)
			g.DeclareInputs("*")
			g.DeclareOutputs("result", "outA", "outB")
			g.AddSubgraph(subA)
			g.AddSubgraph(subB)
			g.AddTransition(startTask, subA)
			g.AddTransition(startTask, subB)
			g.AddTransition(subA, finalTask)
			g.AddTransition(subB, finalTask)
			g.AddTransition(finalTask, workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/main-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("Start",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/start"),
		sub.Web(),
	)
	graphSvc.Subscribe("FinalTask",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("result", expectedValue)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/final-task"),
		sub.Web(),
	)

	tester := connector.New("tester.parsubs")
	app := application.New()
	svc := NewService()
	app.Add(svc, graphSvc, tester)
	app.RunInTest(t)
	client := foremanapi.NewClient(tester)

	// Generous timeout to absorb scheduling jitter when run alongside other parallel tests.
	// A healthy run completes in well under a second; the bug version hangs indefinitely.
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	status, state, err := client.Run(runCtx, mainWorkflow, nil)
	if !assert.NoError(err) {
		return
	}
	assert.Expect(
		status, foremanapi.StatusCompleted,
		state["result"], expectedValue,
		state["outA"], "from-A",
		state["outB"], "from-B",
	)
}
