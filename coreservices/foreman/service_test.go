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
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
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

// MARKER: Create

// MARKER: Start

// MARKER: StartNotify

// MARKER: Snapshot

// MARKER: Resume

// MARKER: Cancel

// MARKER: CreateTask

// MARKER: Await

// MARKER: NotifyStatusChange

// MARKER: History

// MARKER: Continue

// outcomeStatus extracts the Status from a FlowOutcome, returning "" on nil.
func outcomeStatus(o *workflow.FlowOutcome) string {
	if o == nil {
		return ""
	}
	return o.Status
}

// outcomeState extracts the State from a FlowOutcome, returning nil on nil.
func outcomeState(o *workflow.FlowOutcome) map[string]any {
	if o == nil {
		return nil
	}
	return o.State
}

// outcomeStatusState extracts the Status and State from a FlowOutcome.
func outcomeStatusState(o *workflow.FlowOutcome) (string, map[string]any) {
	if o == nil {
		return "", nil
	}
	return o.Status, o.State
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

	db, err := svc.shard(1) // shards are 1-based
	if err != nil {
		t.Fatal(err)
	}

	t.Run("create", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"input": "value"}, nil)
		assert.NoError(err)
		shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
		assert.NoError(err)
		assert.Equal(1, shardNum)

		// Verify flow row
		var status, workflowName, graphJSON, actorClaimsJSON, traceParent, notifyHostname, finalState, breakpointsJSON, threadToken string
		var stepID, surgraphFlowID, surgraphStepDepth, threadID int
		err = db.QueryRowContext(ctx,
			"SELECT flow_token, workflow_name, graph, actor_claims, status, step_id, surgraph_flow_id, surgraph_step_depth, thread_id, thread_token, trace_parent, notify_hostname, final_state, breakpoints FROM microbus_flows WHERE flow_id=?",
			flowID,
		).Scan(&flowToken, &workflowName, &graphJSON, &actorClaimsJSON, &status, &stepID, &surgraphFlowID, &surgraphStepDepth, &threadID, &threadToken, &traceParent, &notifyHostname, &finalState, &breakpointsJSON)
		assert.NoError(err)
		assert.Expect(
			len(flowToken) > 0, true,
			workflowName, "https://test.workflow.host:428/my-workflow",
			strings.TrimSpace(status), workflow.StatusCreated,
			stepID > 0, true,
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
		assert.Equal("https://test.workflow.host:428/my-workflow", g.Name())

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
			taskName, "taskA",
			strings.TrimSpace(stepStatus), workflow.StatusCreated,
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

		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"input": "test"}, nil)
		assert.NoError(err)
		_, flowID, _, err := parseFlowKey(flowKey)
		assert.NoError(err)

		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// Don't snapshot the intermediate "running" state - a two-task no-sleep
		// workflow can race to completed before the SELECT lands. The terminal
		// status assertion below proves Start transitioned the flow out of
		// "created".

		// Wait for completion
		outcome, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Expect(
			outcome.Status, workflow.StatusCompleted,
			outcome.State["result"], "hello world",
		)

		// Verify flow row is completed with filtered final_state
		var status string
		var finalStateJSON string
		db.QueryRowContext(ctx, "SELECT status, final_state FROM microbus_flows WHERE flow_id=?", flowID).Scan(&status, &finalStateJSON)
		assert.Equal(workflow.StatusCompleted, strings.TrimSpace(status))
		var finalState map[string]any
		assert.NoError(json.Unmarshal([]byte(finalStateJSON), &finalState))
		assert.Equal("hello world", finalState["result"])

		// Verify all steps are completed
		var stepCount int
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND status=?", flowID, workflow.StatusCompleted).Scan(&stepCount)
		assert.Equal(2, stepCount)
	})

	t.Run("start_notify", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
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

		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.Cancel(ctx, flowKey, "test cancel")
		assert.NoError(err)

		// Verify flow and step are cancelled, and cancel_reason is recorded
		var flowStatus, cancelReason, stepStatus string
		db.QueryRowContext(ctx, "SELECT status, cancel_reason FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus, &cancelReason)
		assert.Equal(workflow.StatusCancelled, strings.TrimSpace(flowStatus))
		assert.Equal("test cancel", strings.TrimSpace(cancelReason))
		db.QueryRowContext(ctx, "SELECT status FROM microbus_steps WHERE flow_id=? ORDER BY step_id DESC LIMIT_OFFSET(1, 0)", flowID).Scan(&stepStatus)
		assert.Equal(workflow.StatusCancelled, strings.TrimSpace(stepStatus))
	})

	t.Run("break_before", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
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
		outcome, err := foremanClient.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Equal(workflow.StatusInterrupted, status)

		// Verify the interrupted step has breakpoint_hit=1
		var breakpointHit int
		db.QueryRowContext(ctx,
			"SELECT breakpoint_hit FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, workflow.StatusInterrupted,
		).Scan(&breakpointHit)
		assert.Equal(1, breakpointHit)
	})

	t.Run("resume_breakpoint", func(t *testing.T) {
		assert := testarossa.For(t)

		// Pause before task-b via a breakpoint (task-a sets result="hello")
		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.BreakBefore(ctx, flowKey, "https://test.workflow.host:428/task-b", true)
		assert.NoError(err)
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		outcome, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(workflow.StatusInterrupted, outcomeStatus(outcome))

		// Resume (the interrupt path) must refuse a breakpoint pause
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"result": "wrong"})
		assert.Equal(http.StatusConflict, errors.StatusCode(err))

		// ResumeBreak injects state overrides; task-b appends " world" to the overridden result
		err = foremanClient.ResumeBreak(ctx, flowKey, map[string]any{"result": "HELLO"})
		assert.NoError(err)
		outcome, err = foremanClient.Await(ctx, flowKey)
		status, state := outcomeStatusState(outcome)
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				state["result"], "HELLO world",
			)
		}

		var flowStatus string
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(workflow.StatusCompleted, strings.TrimSpace(flowStatus))
	})

	t.Run("resume_breakpoint_rejects_interrupt", func(t *testing.T) {
		assert := testarossa.For(t)

		// A genuine flow.Interrupt pause must reject ResumeBreak
		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{
			"needInput": true,
		}, nil)
		assert.NoError(err)

		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		outcome, err := foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(workflow.StatusInterrupted, outcomeStatus(outcome))

		err = foremanClient.ResumeBreak(ctx, flowKey, map[string]any{"needInput": false})
		assert.Equal(http.StatusConflict, errors.StatusCode(err))

		// The interrupt path still works, completing the flow
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"needInput": false})
		assert.NoError(err)
		outcome, err = foremanClient.Await(ctx, flowKey)
		assert.NoError(err)
		assert.Equal(workflow.StatusCompleted, outcomeStatus(outcome))
	})

	t.Run("interrupt_and_resume", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create and start a flow that will interrupt at task-b
		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{
			"needInput": true,
		}, nil)

		assert.NoError(err)
		_, flowID, _, _ := parseFlowKey(flowKey)

		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		outcome, err := foremanClient.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Equal(workflow.StatusInterrupted, status)

		// Verify flow is interrupted
		var flowStatus string
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(workflow.StatusInterrupted, strings.TrimSpace(flowStatus))

		// Verify the interrupted step has the interrupt payload
		var stepStatus, interruptPayloadJSON string
		db.QueryRowContext(ctx,
			"SELECT status, interrupt_payload FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, workflow.StatusInterrupted,
		).Scan(&stepStatus, &interruptPayloadJSON)
		assert.Equal(workflow.StatusInterrupted, strings.TrimSpace(stepStatus))
		var payload map[string]any
		assert.NoError(json.Unmarshal([]byte(interruptPayloadJSON), &payload))
		assert.Equal("more data", payload["request"])

		// Resume with needInput=false so task-b completes normally
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"needInput": false})
		assert.NoError(err)
		outcome, err = foremanClient.Await(ctx, flowKey)

		status, state := outcomeStatusState(outcome)
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				state["result"], "hello world",
			)
		}

		// Verify flow is completed in the database
		db.QueryRowContext(ctx, "SELECT status FROM microbus_flows WHERE flow_id=?", flowID).Scan(&flowStatus)
		assert.Equal(workflow.StatusCompleted, strings.TrimSpace(flowStatus))
	})

	t.Run("continue", func(t *testing.T) {
		assert := testarossa.For(t)

		// Run to completion
		flowKey, err := foremanClient.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"input": "first"}, nil)
		assert.NoError(err)
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)
		outcome, err := foremanClient.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		assert.NoError(err)
		assert.Equal(workflow.StatusCompleted, status)

		_, firstFlowID, firstFlowToken, _ := parseFlowKey(flowKey)

		// Verify the first flow's thread_id is its own flow_id and thread_token matches flow_token
		var threadID int
		var threadToken string
		db.QueryRowContext(ctx, "SELECT thread_id, thread_token FROM microbus_flows WHERE flow_id=?", firstFlowID).Scan(&threadID, &threadToken)
		assert.Equal(firstFlowID, threadID)
		assert.Equal(firstFlowToken, strings.TrimSpace(threadToken))

		// Continue with additional state
		newFlowKey, err := foremanClient.Continue(ctx, flowKey, map[string]any{"extra": "data"}, nil)
		assert.NoError(err)
		_, newFlowID, _, _ := parseFlowKey(newFlowKey)

		// Verify new flow was created with merged state
		var newStatus, newWorkflowName string
		db.QueryRowContext(ctx, "SELECT status, workflow_name FROM microbus_flows WHERE flow_id=?", newFlowID).Scan(&newStatus, &newWorkflowName)
		assert.Expect(
			strings.TrimSpace(newStatus), workflow.StatusCreated,
			newWorkflowName, "https://test.workflow.host:428/my-workflow",
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

// newTestWorkflowSvc creates a minimal workflow service (taskA -> taskB -> END).
// taskA sets result="hello". taskB appends " world" unless needInput=true (then it interrupts).
func newTestWorkflowSvc() *connector.Connector {
	graphSvc := connector.New("test.workflow.host")
	graphSvc.Subscribe("MyWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/my-workflow")
			g.AddTask("taskA", "https://test.workflow.host:428/task-a")
			g.AddTask("taskB", "https://test.workflow.host:428/task-b")
			g.AddTransition("taskA", "taskB")
			g.AddTransition("taskB", workflow.END)
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
				_, yield, err := f.Interrupt(map[string]any{"request": "more data"})
				if err != nil {
					return err
				}
				if yield {
					return json.NewEncoder(w).Encode(&f)
				}
			}
			f.SetString("result", f.GetString("result")+" world")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-b"),
		sub.Web(),
	)
	// Dynamic-subgraph error delivery: ParentSub calls flow.Subgraph on a child whose only task fails.
	// The child error is delivered to ParentSub via flow.Subgraph's err return (not cascaded), and
	// ParentSub returns it so the parent flow fails with that error.
	graphSvc.Subscribe("ParentSubWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/parent-sub-workflow")
			g.AddTask("parentSub", "https://test.workflow.host:428/parent-sub")
			g.AddTransition("parentSub", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/parent-sub-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("ParentSub",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			out, yield, err := f.Subgraph("https://test.workflow.host:428/child-fail-workflow", nil)
			if yield {
				return json.NewEncoder(w).Encode(&f)
			}
			if err != nil {
				// Surface the child's error: the parent flow fails with it.
				return err
			}
			f.Set("childOut", out)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/parent-sub"),
		sub.Web(),
	)
	graphSvc.Subscribe("ChildFailWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/child-fail-workflow")
			g.AddTask("childFail", "https://test.workflow.host:428/child-fail")
			g.AddTransition("childFail", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/child-fail-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("ChildFail",
		func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("child boom")
		},
		sub.At("POST", ":428/child-fail"),
		sub.Web(),
	)
	// Single-park guard: doublePark arms an interrupt, and after Resume arms a subgraph on the same
	// step. The second park is illegal because the step's interrupt slot already resolved, so the
	// foreman fails the step (and the flow) instead of launching the child.
	graphSvc.Subscribe("DoubleParkWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/double-park-workflow")
			g.AddTask("doublePark", "https://test.workflow.host:428/double-park")
			g.AddTransition("doublePark", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/double-park-workflow"),
		sub.Web(),
	)
	// Retry clears the park slot: retrySub arms a subgraph whose child fails on its first invocation
	// and succeeds on the second. The parent's flow.Retry after the failed child clears subgraph_done,
	// so the re-dispatch re-arms the subgraph and runs a fresh child that succeeds.
	var retryChildAttempts atomic.Int32
	graphSvc.Subscribe("RetrySubWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/retry-sub-workflow")
			g.AddTask("retrySub", "https://test.workflow.host:428/retry-sub")
			g.AddTransition("retrySub", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/retry-sub-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("RetrySub",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			out, yield, err := f.Subgraph("https://test.workflow.host:428/retry-child-workflow", nil)
			if yield {
				return json.NewEncoder(w).Encode(&f)
			}
			if err != nil && f.Retry(5, 0, 0, 0) {
				return json.NewEncoder(w).Encode(&f)
			}
			if err != nil {
				return err
			}
			f.Set("childResult", out["childResult"])
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/retry-sub"),
		sub.Web(),
	)
	graphSvc.Subscribe("RetryChildWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.workflow.host:428/retry-child-workflow")
			g.AddTask("retryChild", "https://test.workflow.host:428/retry-child")
			g.AddTransition("retryChild", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/retry-child-workflow"),
		sub.Web(),
	)
	graphSvc.Subscribe("RetryChild",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			if retryChildAttempts.Add(1) == 1 {
				return errors.New("child fails on first attempt")
			}
			f.Set("childResult", "ok")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/retry-child"),
		sub.Web(),
	)
	graphSvc.Subscribe("DoublePark",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			_, yield, err := f.Interrupt(map[string]any{"request": "first"})
			if err != nil {
				return err
			}
			if yield {
				return json.NewEncoder(w).Encode(&f)
			}
			// Resumed: the interrupt slot is resolved. Arming a subgraph on the same step is illegal.
			_, yield, err = f.Subgraph("https://test.workflow.host:428/child-fail-workflow", nil)
			if err != nil {
				return err
			}
			if yield {
				return json.NewEncoder(w).Encode(&f)
			}
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/double-park"),
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"x": 1}, nil)
	if assert.NoError(err) {
		assert.True(flowKey != "")
	}

	// Invalid workflow name
	_, err = client.Create(ctx, "", nil, nil)
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCompleted, status)
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.StartNotify(ctx, flowKey, "my.notify.host")
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCompleted, status)
}

// TestForeman_DynamicSubgraphError verifies that a failing dynamic subgraph child delivers its error to
// the parent task via flow.Subgraph's err return (rather than cascading), and that the parent surfacing
// that error fails the parent flow with it.
func TestForeman_DynamicSubgraphError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/parent-sub-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(workflow.StatusFailed, outcomeStatus(outcome))
	assert.True(strings.Contains(outcome.Error, "child boom"))
}

// TestForeman_SingleParkGuard verifies that a task arming a second park of a different kind on a
// step whose slot already resolved is rejected: doublePark interrupts, and after Resume arms a
// subgraph on the same step, which fails the step (and the flow) with a clear message.
func TestForeman_SingleParkGuard(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/double-park-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(workflow.StatusInterrupted, outcomeStatus(outcome))

	// Resume: on re-entry the task arms a subgraph on the already-resolved interrupt slot.
	err = client.Resume(ctx, flowKey, map[string]any{})
	assert.NoError(err)
	outcome, err = client.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(workflow.StatusFailed, outcomeStatus(outcome))
	assert.True(strings.Contains(outcome.Error, "already parked for an interrupt"))
}

// TestForeman_RetryClearsSubgraphPark verifies that flow.Retry after a failed dynamic subgraph clears the
// step's park slot, so the re-dispatch re-arms the subgraph and runs a fresh child. The child fails on its
// first invocation and succeeds on the second; the parent flow completes with the child's result.
func TestForeman_RetryClearsSubgraphPark(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/retry-sub-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)
	assert.NoError(err)
	status, state := outcomeStatusState(outcome)
	assert.Equal(workflow.StatusCompleted, status)
	assert.Equal("ok", state["childResult"])
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Cancel(ctx, flowKey, "")
	assert.NoError(err)

	outcome, err := client.Snapshot(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCancelled, status)
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
	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"needInput": true}, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusInterrupted, status)

	// Resume
	err = client.Resume(ctx, flowKey, map[string]any{"needInput": false})
	assert.NoError(err)
	outcome, err = client.Await(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Expect(
			status, workflow.StatusCompleted,
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.BreakBefore(ctx, flowKey, "https://test.workflow.host:428/task-b", true)
	assert.NoError(err)

	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusInterrupted, status)

	// Resume past the breakpoint (the debug path; Resume is for interrupts and would reject this pause)
	err = client.ResumeBreak(ctx, flowKey, nil)
	assert.NoError(err)
	outcome, err = client.Await(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Expect(
			status, workflow.StatusCompleted,
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
	outcome, err := client.Await(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"input": "test"}, nil)
	assert := testarossa.For(t)
	if !assert.NoError(err) {
		return
	}

	// Snapshot of created flow
	outcome, err := client.Snapshot(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCreated, status)

	// Run to completion and snapshot again
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	client.Await(ctx, flowKey)
	outcome, err = client.Snapshot(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Expect(
			status, workflow.StatusCompleted,
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

	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
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
		assert.Equal("taskA", steps[0].TaskName)
		assert.Equal(workflow.StatusCompleted, steps[0].Status)
		assert.Equal("taskB", steps[1].TaskName)
		assert.Equal(workflow.StatusCompleted, steps[1].Status)
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
	flowKey1, _ := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	flowKey2, _ := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	assert := testarossa.For(t)

	// List created flows
	flows, _, err := client.List(ctx, foremanapi.Query{Status: workflow.StatusCreated})
	if assert.NoError(err) {
		assert.True(len(flows) >= 2)
	}

	// Complete one and list again
	client.Start(ctx, flowKey1)
	client.Await(ctx, flowKey1)

	flows, _, err = client.List(ctx, foremanapi.Query{Status: workflow.StatusCompleted})
	if assert.NoError(err) {
		assert.True(len(flows) >= 1)
	}

	// Cancel the other
	client.Cancel(ctx, flowKey2, "")
	flows, _, err = client.List(ctx, foremanapi.Query{Status: workflow.StatusCancelled})
	if assert.NoError(err) {
		assert.True(len(flows) >= 1)
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

	outcome, err := client.Run(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{"input": "test"}, nil)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Expect(
			status, workflow.StatusCompleted,
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
	flowKey, err := client.Create(ctx, "https://test.workflow.host:428/my-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status := outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCompleted, status)

	// The flowKey returned by Create is also the threadKey
	threadKey := flowKey

	// Continue using the threadKey (which is the first flow's key)
	newFlowKey, err := client.Continue(ctx, threadKey, map[string]any{"extra": "data"}, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, newFlowKey)
	assert.NoError(err)
	outcome, err = client.Await(ctx, newFlowKey)

	status, state := outcomeStatusState(outcome)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(workflow.StatusCompleted, status)
	assert.Equal("hello world", state["result"])

	// Continue again using the original threadKey (not the intermediate flowKey)
	thirdFlowKey, err := client.Continue(ctx, threadKey, map[string]any{"turn": 3}, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, thirdFlowKey)
	assert.NoError(err)
	outcome, err = client.Await(ctx, thirdFlowKey)

	status = outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCompleted, status)

	// Continue using an intermediate flowKey (should also work)
	fourthFlowKey, err := client.Continue(ctx, newFlowKey, map[string]any{"turn": 4}, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, fourthFlowKey)
	assert.NoError(err)
	outcome, err = client.Await(ctx, fourthFlowKey)

	status = outcomeStatus(outcome)
	assert.NoError(err)
	assert.Equal(workflow.StatusCompleted, status)

	// List by thread should show all 4 flows
	flows, _, err := client.List(ctx, foremanapi.Query{ThreadKey: threadKey})
	if assert.NoError(err) {
		assert.Equal(4, len(flows))
		// All should share the same ThreadKey
		for _, f := range flows {
			assert.Equal(threadKey, f.ThreadKey)
		}
	}
}

// newTestErrorWorkflowSvc creates a workflow where taskA fails if failTask=true,
// routing to an error handler that captures the error. Otherwise taskA succeeds to taskB.
// taskA -> taskB -> END (happy path)
// taskA -> errorHandler -> END (error path)
func newTestErrorWorkflowSvc() *connector.Connector {
	svc := connector.New("test.error.host")
	svc.Subscribe("ErrorWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.error.host:428/error-workflow")
			g.AddTask("taskA", "https://test.error.host:428/task-a")
			g.AddTask("taskB", "https://test.error.host:428/task-b")
			g.AddTask("errorHandler", "https://test.error.host:428/error-handler")
			g.AddTransition("taskA", "taskB")
			g.AddTransition("taskB", workflow.END)
			g.AddTransitionOnError("taskA", "errorHandler")
			g.AddTransition("errorHandler", workflow.END)
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
	flowKey, err := client.Create(ctx, "https://test.error.host:428/error-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err := client.Await(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
		assert.Equal("success via task-b", state["result"])
	}

	// Error path: taskA fails, routes to errorHandler
	flowKey, err = client.Create(ctx, "https://test.error.host:428/error-workflow", map[string]any{"failTask": true}, nil)
	if !assert.NoError(err) {
		return
	}
	err = client.Start(ctx, flowKey)
	assert.NoError(err)
	outcome, err = client.Await(ctx, flowKey)

	status, state = outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
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
		runSubTask     = "https://test.race.host:428/run-sub"
		subTask        = "https://test.race.host:428/sub-task"
		slowTaskSleep  = 100 * time.Millisecond
		expectedResult = "done"
	)

	graphSvc := connector.New(host)

	// Subgraph: a single instant task -> END.
	graphSvc.Subscribe("SubWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(subWorkflow)
			g.AddTask("subTask", subTask)
			g.AddTransition("subTask", workflow.END)
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
	// RunSub invokes the subgraph via flow.Subgraph; its parked step stays running at the fan-out
	// depth while slow-task runs, reproducing the completeSurgraphFlow race.
	graphSvc.Subscribe("RunSub",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			_, yield, err := f.Subgraph(subWorkflow, nil)
			if err != nil {
				return err
			}
			if yield {
				return json.NewEncoder(w).Encode(&f)
			}
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/run-sub"),
		sub.Web(),
	)

	// Main workflow: start fans out to {slow-task, sub-workflow}; both fan in to final-task.
	// slow-task is registered FIRST so it gets the lower step_id.
	graphSvc.Subscribe("MainWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(mainWorkflow)
			g.AddTask("startTask", startTask)
			g.AddTask("slowTask", slowTask)
			g.AddTask("subWorkflow", runSubTask)
			g.AddTask("finalTask", finalTask)
			g.SetFanIn("finalTask")
			g.AddTransition("startTask", "slowTask")
			g.AddTransition("startTask", "subWorkflow")
			g.AddTransition("slowTask", "finalTask")
			g.AddTransition("subWorkflow", "finalTask")
			g.AddTransition("finalTask", workflow.END)
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

	outcome, err := client.Run(runCtx, mainWorkflow, nil, nil)

	status, state := outcomeStatusState(outcome)
	if !assert.NoError(err) {
		return
	}
	assert.Expect(
		status, workflow.StatusCompleted,
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
		runA          = "https://test.parsubs.host:428/run-a"
		runB          = "https://test.parsubs.host:428/run-b"
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
	registerSub := func(name, workflowRoute, taskRoute, runRoute, workflowURL, taskURL, outputField, outputValue string, delay time.Duration) {
		graphSvc.Subscribe(name+"Workflow",
			func(w http.ResponseWriter, r *http.Request) error {
				g := workflow.NewGraph(workflowURL)
				g.AddTask(name, taskURL)
				g.AddTransition(name, workflow.END)
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
		// Caller task: invokes the subgraph via flow.Subgraph and adopts its output field. Two of these
		// run in parallel from the main fan-out, parking at the same step_depth with far-future leases -
		// the surgraph_step_id PK lookup is what keeps their completions from matching each other's step.
		graphSvc.Subscribe(name+"Run",
			func(w http.ResponseWriter, r *http.Request) error {
				var f workflow.Flow
				if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
					return err
				}
				out, yield, err := f.Subgraph(workflowURL, nil)
				if err != nil {
					return err
				}
				if yield {
					return json.NewEncoder(w).Encode(&f)
				}
				if v, ok := out[outputField].(string); ok {
					f.SetString(outputField, v)
				}
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(&f)
			},
			sub.At("POST", runRoute),
			sub.Web(),
		)
	}
	// SubA inserted first into the parent's fan-out (lower step_id) but slow to complete.
	// SubB inserted second (higher step_id) but completes immediately.
	registerSub("SubA", ":428/sub-a", ":428/task-a", ":428/run-a", subA, taskA, "outA", "from-A", 100*time.Millisecond)
	registerSub("SubB", ":428/sub-b", ":428/task-b", ":428/run-b", subB, taskB, "outB", "from-B", 0)

	// Main: start fans out to both subgraphs; both fan in to final-task.
	graphSvc.Subscribe("MainWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph(mainWorkflow)
			g.AddTask("startTask", startTask)
			g.AddTask("subA", runA)
			g.AddTask("subB", runB)
			g.AddTask("finalTask", finalTask)
			g.SetFanIn("finalTask")
			g.AddTransition("startTask", "subA")
			g.AddTransition("startTask", "subB")
			g.AddTransition("subA", "finalTask")
			g.AddTransition("subB", "finalTask")
			g.AddTransition("finalTask", workflow.END)
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

	outcome, err := client.Run(runCtx, mainWorkflow, nil, nil)

	status, state := outcomeStatusState(outcome)
	if !assert.NoError(err) {
		return
	}
	assert.Expect(
		status, workflow.StatusCompleted,
		state["result"], expectedValue,
		state["outA"], "from-A",
		state["outB"], "from-B",
	)
}

// newTestTimeoutWorkflowSvc creates a workflow where taskA fails with a
// configurable HTTP status code, routing through OnTimeout (408-coded) or
// OnError (catch-all) handlers. The workflow records which handler ran.
//
// taskA -> taskB -> END                  (happy path)
// taskA -> timeoutHandler -> END         (when status == 408)
// taskA -> errHandler -> END             (any other error)
func newTestTimeoutWorkflowSvc() *connector.Connector {
	svc := connector.New("test.timeout.host")
	svc.Subscribe("TimeoutWorkflow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.timeout.host:428/timeout-workflow")
			g.AddTask("taskA", "https://test.timeout.host:428/task-a")
			g.AddTask("taskB", "https://test.timeout.host:428/task-b")
			g.AddTask("timeoutHandler", "https://test.timeout.host:428/timeout-handler")
			g.AddTask("errHandler", "https://test.timeout.host:428/err-handler")
			g.AddTransition("taskA", "taskB")
			g.AddTransition("taskB", workflow.END)
			g.AddTransitionOnTimeout("taskA", "timeoutHandler")
			g.AddTransitionOnError("taskA", "errHandler")
			g.AddTransition("timeoutHandler", workflow.END)
			g.AddTransition("errHandler", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/timeout-workflow"),
		sub.Web(),
	)
	svc.Subscribe("TaskA",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			status := f.GetInt("failStatus")
			if status != 0 {
				return errors.New("task-a failed", status)
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
			f.SetString("handled", "taskB")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/task-b"),
		sub.Web(),
	)
	svc.Subscribe("TimeoutHandler",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("handled", "timeout")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/timeout-handler"),
		sub.Web(),
	)
	svc.Subscribe("ErrHandler",
		func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("handled", "err")
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		},
		sub.At("POST", ":428/err-handler"),
		sub.Web(),
	)
	return svc
}

func TestForeman_OnTimeoutTransition(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestTimeoutWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	// failStatus = 408 -> OnTimeout transition wins over OnError.
	flowKey, err := client.Create(ctx, "https://test.timeout.host:428/timeout-workflow", map[string]any{
		"failStatus": http.StatusRequestTimeout,
	}, nil)

	if !assert.NoError(err) {
		return
	}
	assert.NoError(client.Start(ctx, flowKey))
	outcome, err := client.Await(ctx, flowKey)

	status, state := outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
		assert.Equal("timeout", state["handled"])
	}

	// failStatus = 500 -> OnTimeout doesn't match, falls back to OnError.
	flowKey, err = client.Create(ctx, "https://test.timeout.host:428/timeout-workflow", map[string]any{
		"failStatus": http.StatusInternalServerError,
	}, nil)

	if !assert.NoError(err) {
		return
	}
	assert.NoError(client.Start(ctx, flowKey))
	outcome, err = client.Await(ctx, flowKey)

	status, state = outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
		assert.Equal("err", state["handled"])
	}

	// failStatus = 0 -> taskA succeeds, no error routing.
	flowKey, err = client.Create(ctx, "https://test.timeout.host:428/timeout-workflow", nil, nil)
	if !assert.NoError(err) {
		return
	}
	assert.NoError(client.Start(ctx, flowKey))
	outcome, err = client.Await(ctx, flowKey)

	status, state = outcomeStatusState(outcome)
	if assert.NoError(err) {
		assert.Equal(workflow.StatusCompleted, status)
		assert.Equal("taskB", state["handled"])
	}
}

// TestForeman_ShardInfo verifies that the ShardInfo endpoint returns one entry
// per shard with a 1-based index, a non-zero latency, and the correct row
// counts after seeding step/flow rows into each shard.
func TestForeman_ShardInfo(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const numShards = 3

	svc := NewService().Init(func(s *Service) error {
		if err := s.SetSQLDataSourceName("file:shardinfo%d?mode=memory&cache=shared"); err != nil {
			return err
		}
		return s.SetNumShards(numShards)
	})
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// Seed one step row on each shard so the row counts are non-trivial. Shards
	// are 1-based; svc.dbs is a 0-based slice.
	for i := 1; i <= numShards; i++ {
		db, err := svc.shard(i)
		if !assert.NoError(err) {
			return
		}
		_, err = db.ExecContext(ctx,
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lease_expires, priority, fairness_key, fairness_weight)"+
				" VALUES (?, 1, ?, ?, '{}', ?, 60000, DATE_ADD_MILLIS(NOW_UTC(), 60000), 1, '', 1)",
			1, "stok", "shard_info_task", workflow.StatusRunning,
		)
		if !assert.NoError(err) {
			return
		}
	}

	shards, err := fm.ShardInfo(ctx)
	if !assert.NoError(err) {
		return
	}
	if !assert.Equal(numShards, len(shards)) {
		return
	}
	for i, s := range shards {
		assert.Equal(i+1, s.Shard) // 1-based
		assert.Equal("", s.Error)
		assert.Equal(1, s.Steps)
		// LatencyMs and Flows are non-negative; we don't assert exact values to
		// avoid flakiness.
		assert.True(s.LatencyMs >= 0)
		assert.True(s.Flows >= 0)
	}
}

// TestForeman_List_QueryShard verifies that setting Query.Shard restricts
// List to a single shard's flows.
func TestForeman_List_QueryShard(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const numShards = 3

	svc := NewService().Init(func(s *Service) error {
		if err := s.SetSQLDataSourceName("file:listshard%d?mode=memory&cache=shared"); err != nil {
			return err
		}
		return s.SetNumShards(numShards)
	})
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// Create enough flows to cover several pages and exercise multiple shards.
	for range 24 {
		_, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
	}

	// Query each shard individually and assert every returned FlowKey carries
	// that shard's prefix.
	for s := 1; s <= numShards; s++ {
		flows, _, err := fm.List(ctx, foremanapi.Query{
			Status: workflow.StatusCreated,
			Shard:  s,
			Limit:  100,
		})
		if !assert.NoError(err) {
			return
		}
		prefix := strconv.Itoa(s) + "-"
		for _, f := range flows {
			assert.True(strings.HasPrefix(f.FlowKey, prefix))
		}
	}

	// Out-of-range shard is rejected.
	_, _, err := fm.List(ctx, foremanapi.Query{Shard: numShards + 1})
	assert.Error(err)
}

// TestForeman_List_CrossShardPagination verifies that List uses per-shard
// pagination (per-shard limit + per-shard flow_id cursors), so:
//   - all inserted flows are returned across pages with no duplicates and no
//     missing rows;
//   - a flow inserted on any shard mid-pagination does not appear on the next
//     page (its flow_id is above that shard's cursor).
//
// We deliberately do not assert a global newest-first order; presentation is
// shard-grouped because cross-shard time comparison is too noisy to depend on.
func TestForeman_List_CrossShardPagination(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const numShards = 2

	svc := NewService().Init(func(s *Service) error {
		if err := s.SetSQLDataSourceName("file:listpage%d?mode=memory&cache=shared"); err != nil {
			return err
		}
		return s.SetNumShards(numShards)
	})
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	mustCreate := func() string {
		fk, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) {
			return ""
		}
		return fk
	}
	const total = 12
	allKeys := map[string]bool{}
	for range total {
		allKeys[mustCreate()] = true
	}

	// Page through. With numShards=2 and limit=4, each page fetches up to 2
	// from each shard. Three pages of 4 should exhaust the 12; an empty
	// NextCursor signals end-of-results.
	const pageSize = 4
	seen := map[string]int{}
	cursor := ""
	for pages := 0; pages < total; pages++ { // generous upper bound
		flows, next, err := fm.List(ctx, foremanapi.Query{
			Status: workflow.StatusCreated,
			Limit:  pageSize,
			Cursor: cursor,
		})
		if !assert.NoError(err) {
			return
		}
		for _, f := range flows {
			seen[f.FlowKey]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	// Every inserted flow appears exactly once.
	assert.Equal(total, len(seen))
	for fk := range allKeys {
		assert.Equal(1, seen[fk])
	}
	for fk := range seen {
		assert.True(allKeys[fk])
	}

	// Mid-pagination insert: take page 1, insert a new flow, take page 2 with
	// the page-1 cursor. The fresh flow must not appear on page 2 because its
	// flow_id is above its shard's cursor in the encoded NextCursor.
	flows, firstCursor, err := fm.List(ctx, foremanapi.Query{
		Status: workflow.StatusCreated,
		Limit:  pageSize,
	})
	if !assert.NoError(err) {
		return
	}
	_ = flows
	freshKey := mustCreate()
	page2, _, err := fm.List(ctx, foremanapi.Query{
		Status: workflow.StatusCreated,
		Limit:  pageSize,
		Cursor: firstCursor,
	})
	if !assert.NoError(err) {
		return
	}
	for _, f := range page2 {
		assert.NotEqual(freshKey, f.FlowKey)
	}
}

func TestForeman_Delete(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	t.Run("delete_created_flow", func(t *testing.T) {
		assert := testarossa.For(t)
		flowKey, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
		err = fm.Delete(ctx, flowKey)
		assert.NoError(err)
		_, err = fm.Snapshot(ctx, flowKey)
		assert.Error(err) // Should be 404 not found after delete
	})

	t.Run("delete_terminal_flow", func(t *testing.T) {
		assert := testarossa.For(t)
		outcome, err := fm.Run(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) || !assert.NotNil(outcome) {
			return
		}
		assert.Equal(workflow.StatusCompleted, outcome.Status)
		err = fm.Delete(ctx, outcome.FlowKey)
		assert.NoError(err)
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		assert := testarossa.For(t)
		// Use a syntactically valid but nonexistent flow key on shard 1
		err := fm.Delete(ctx, "1-99999999-deadbeefdeadbeefdeadbeefdeadbeef")
		assert.Error(err)
	})
}

func TestForeman_Purge(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// Create 5 flows in created state and run 3 to completion.
	for range 5 {
		_, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
	}
	for range 3 {
		outcome, err := fm.Run(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		if !assert.NoError(err) || !assert.NotNil(outcome) {
			return
		}
		assert.Equal(workflow.StatusCompleted, outcome.Status)
	}

	t.Run("purge_by_status_completed", func(t *testing.T) {
		assert := testarossa.For(t)
		deleted, err := fm.Purge(ctx, foremanapi.Query{Status: workflow.StatusCompleted})
		assert.NoError(err)
		assert.Equal(3, deleted)
		// Verify all completed flows are gone.
		flows, _, err := fm.List(ctx, foremanapi.Query{Status: workflow.StatusCompleted, Limit: 100})
		assert.NoError(err)
		assert.Equal(0, len(flows))
	})

	t.Run("purge_remaining_created", func(t *testing.T) {
		assert := testarossa.For(t)
		deleted, err := fm.Purge(ctx, foremanapi.Query{Status: workflow.StatusCreated})
		assert.NoError(err)
		assert.Equal(5, deleted)
	})
}

func TestForeman_Query_TenantID(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// Create flows under three different tenants using actor claims with the tid claim.
	createWithTenant := func(tid int) string {
		fCtx := frame.CloneContext(ctx)
		frame.Of(fCtx).SetActor(map[string]any{"tid": tid})
		flowKey, err := fm.Create(fCtx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
		assert.NoError(err)
		return flowKey
	}
	for range 2 {
		createWithTenant(101)
	}
	for range 3 {
		createWithTenant(202)
	}
	// One flow without a tenant (no actor) - tenant_id defaults to 0.
	_, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
	assert.NoError(err)

	// Filter by tenant.
	t.Run("tenant_101", func(t *testing.T) {
		assert := testarossa.For(t)
		flows, _, err := fm.List(ctx, foremanapi.Query{TenantID: 101, Limit: 100})
		assert.NoError(err)
		assert.Equal(2, len(flows))
	})
	t.Run("tenant_202", func(t *testing.T) {
		assert := testarossa.For(t)
		flows, _, err := fm.List(ctx, foremanapi.Query{TenantID: 202, Limit: 100})
		assert.NoError(err)
		assert.Equal(3, len(flows))
	})
	t.Run("no_tenant_filter_returns_all", func(t *testing.T) {
		assert := testarossa.For(t)
		flows, _, err := fm.List(ctx, foremanapi.Query{Limit: 100})
		assert.NoError(err)
		assert.True(len(flows) >= 6)
	})
}

// newSwitchWorkflowSvc creates a workflow with a router task that uses Switch transitions
// to route to one of three handlers (A/B/default) based on the input "kind" state field.
// Each handler sets result="A"/"B"/"default". The router task is a no-op pass-through.
func newSwitchWorkflowSvc() *connector.Connector {
	svc := connector.New("test.switch.host")
	svc.Subscribe("SwitchFlow",
		func(w http.ResponseWriter, r *http.Request) error {
			g := workflow.NewGraph("https://test.switch.host:428/switch-flow")
			g.AddTask("router", "https://test.switch.host:428/router")
			g.AddTask("a", "https://test.switch.host:428/handler-a")
			g.AddTask("b", "https://test.switch.host:428/handler-b")
			g.AddTask("default", "https://test.switch.host:428/handler-default")
			g.AddTransitionSwitch("router", "a", `kind=="a"`)
			g.AddTransitionSwitch("router", "b", `kind=="b"`)
			g.AddTransitionSwitch("router", "default", "true")
			g.AddTransition("a", workflow.END)
			g.AddTransition("b", workflow.END)
			g.AddTransition("default", workflow.END)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(map[string]any{"graph": g})
		},
		sub.At("GET", ":428/switch-flow"),
		sub.Web(),
	)
	passthrough := func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	}
	setResult := func(value string) func(http.ResponseWriter, *http.Request) error {
		return func(w http.ResponseWriter, r *http.Request) error {
			var f workflow.Flow
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				return err
			}
			f.SetString("result", value)
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(&f)
		}
	}
	svc.Subscribe("Router", passthrough, sub.At("POST", ":428/router"), sub.Web())
	svc.Subscribe("HandlerA", setResult("A"), sub.At("POST", ":428/handler-a"), sub.Web())
	svc.Subscribe("HandlerB", setResult("B"), sub.At("POST", ":428/handler-b"), sub.Web())
	svc.Subscribe("HandlerDefault", setResult("default"), sub.At("POST", ":428/handler-default"), sub.Web())
	return svc
}

func TestForeman_SwitchFirstMatchWins(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	svc := NewService()
	tester := connector.New("tester.client")
	client := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newSwitchWorkflowSvc(), tester)
	app.RunInTest(t)
	assert := testarossa.For(t)

	cases := []struct {
		kind   string
		expect string
	}{
		{"a", "A"},
		{"b", "B"},
		{"c", "default"},
		{"", "default"},
	}
	for _, tc := range cases {
		outcome, err := client.Run(ctx,
			"https://test.switch.host:428/switch-flow",
			map[string]any{"kind": tc.kind},
			nil,
		)
		if !assert.NoError(err) {
			continue
		}
		assert.Expect(outcome.Status, "completed")
		assert.Expect(outcome.State["result"], tc.expect)
	}
}
