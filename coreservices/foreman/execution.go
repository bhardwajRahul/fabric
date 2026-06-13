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
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

	"go.opentelemetry.io/otel/propagation"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// fetchGraph retrieves a workflow graph definition by making a GET request to the workflow URL.
// workflowName is the full URL like "https://playground.fabric:428/multiply-and-check".
func (svc *Service) fetchGraph(ctx context.Context, workflowName string) (*workflow.Graph, error) {
	u := workflowName
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	httpRes, err := svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(u),
	)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var wrapper struct {
		Graph workflow.Graph `json:"graph"`
	}
	err = json.NewDecoder(httpRes.Body).Decode(&wrapper)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = wrapper.Graph.Validate()
	if err != nil {
		return nil, errors.Trace(err, http.StatusBadRequest)
	}
	return &wrapper.Graph, nil
}

// processStep acquires a step, executes its task, and enqueues the next step if applicable.
func (svc *Service) processStep(ctx context.Context, stepID int, shardNum int) (err error) {
	defer func() {
		if sequel.IsLockContentionError(err) {
			// The step row was not updated. If this worker had already leased it,
			// expire the lease and reset it to pending so the immediate poll can
			// re-enqueue it now. Without this the step keeps its full
			// time_budget+leaseMargin lease (minutes), so pollPendingSteps - which
			// only recovers running steps whose lease has expired - cannot act on
			// it, stalling the whole flow's fan-in until the lease lapses.
			if db, derr := svc.shard(shardNum); derr == nil {
				db.ExecContext(ctx,
					"UPDATE microbus_steps SET status=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
					workflow.StatusPending, stepID, workflow.StatusRunning,
				)
			}
			// Trigger an immediate poll cycle to recover it rather than waiting up
			// to maxPollInterval for the next scheduled poll.
			svc.shortenNextPoll(time.Now())
		}
	}()
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	// Lease is sized to the foreman's current TimeBudget config + leaseMargin
	// rather than the step's stored time_budget_ms, so the lease can be set
	// without an upfront SELECT. The per-step time_budget_ms is read in the
	// parallel block below for the dispatch timeout. Assumes the operator does
	// not decrease TimeBudget mid-flight below the largest in-flight step's
	// budget; if they do, in-flight leases may expire before completion and
	// trigger pollPendingSteps recovery.
	leaseMs := int(svc.TimeBudget().Milliseconds() + leaseMargin.Milliseconds())

	var n int64
	var stepDepth int
	var taskName string
	var stateJSON string
	var priorChangesJSON string
	var breakpointHit bool
	var attempt int
	var lineageID int
	var flowID int
	var timeBudgetMs int
	var interruptDone bool
	var resumeDataJSON string
	var subgraphDone bool
	var subgraphResultJSON string
	var subgraphErrorStr string
	switch db.DriverName() {
	case "pgx", "sqlite":
		// Single round-trip claim+read: UPDATE ... RETURNING. The CAS predicate
		// gates the claim, so an unmatched UPDATE yields ErrNoRows from Scan.
		err = db.QueryRowContext(ctx,
			"UPDATE microbus_steps SET status=?, lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC(),"+
				" started_at=CASE WHEN attempt>0 OR subgraph_done=1 OR interrupt_done=1 THEN started_at ELSE NOW_UTC() END"+
				" WHERE step_id=? AND status=? AND parked=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC()"+
				" RETURNING step_depth, task_name, state, changes, breakpoint_hit, attempt, lineage_id, flow_id, time_budget_ms, interrupt_done, resume_data, subgraph_done, subgraph_result, subgraph_error",
			workflow.StatusRunning, leaseMs, stepID, workflow.StatusPending, parkedNone,
		).Scan(&stepDepth, &taskName, &stateJSON, &priorChangesJSON, &breakpointHit, &attempt, &lineageID, &flowID, &timeBudgetMs, &interruptDone, &resumeDataJSON, &subgraphDone, &subgraphResultJSON, &subgraphErrorStr)
		if err == sql.ErrNoRows {
			n = 0
			err = nil
		} else if err == nil {
			n = 1
		}
	case "mssql":
		// SQL Server uses OUTPUT INSERTED.* between SET and WHERE; same single-
		// round-trip semantics as RETURNING.
		err = db.QueryRowContext(ctx,
			"UPDATE microbus_steps SET status=?, lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC(),"+
				" started_at=CASE WHEN attempt>0 OR subgraph_done=1 OR interrupt_done=1 THEN started_at ELSE NOW_UTC() END"+
				" OUTPUT INSERTED.step_depth, INSERTED.task_name, INSERTED.state, INSERTED.changes, INSERTED.breakpoint_hit, INSERTED.attempt, INSERTED.lineage_id, INSERTED.flow_id, INSERTED.time_budget_ms, INSERTED.interrupt_done, INSERTED.resume_data, INSERTED.subgraph_done, INSERTED.subgraph_result, INSERTED.subgraph_error"+
				" WHERE step_id=? AND status=? AND parked=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC()",
			workflow.StatusRunning, leaseMs, stepID, workflow.StatusPending, parkedNone,
		).Scan(&stepDepth, &taskName, &stateJSON, &priorChangesJSON, &breakpointHit, &attempt, &lineageID, &flowID, &timeBudgetMs, &interruptDone, &resumeDataJSON, &subgraphDone, &subgraphResultJSON, &subgraphErrorStr)
		if err == sql.ErrNoRows {
			n = 0
			err = nil
		} else if err == nil {
			n = 1
		}
	default:
		err = svc.Parallel(
			func() error {
				// Atomic claim. WHERE clause gates: only one worker wins.
				res, err := db.ExecContext(ctx,
					"UPDATE microbus_steps SET status=?, lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC(),"+
						" started_at=CASE WHEN attempt>0 OR subgraph_done=1 OR interrupt_done=1 THEN started_at ELSE NOW_UTC() END"+
						" WHERE step_id=? AND status=? AND parked=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC()",
					workflow.StatusRunning, leaseMs, stepID, workflow.StatusPending, parkedNone,
				)
				if err != nil {
					return errors.Trace(err)
				}
				n, _ = res.RowsAffected()
				return nil
			},
			func() error {
				// The UPDATE only mutates status, lease_expires, updated_at, started_at; the columns
				// read here are stable for the row's lifetime, so this race-reads safely.
				err := db.QueryRowContext(ctx,
					"SELECT step_depth, task_name, state, changes, breakpoint_hit, attempt, lineage_id, flow_id, time_budget_ms, interrupt_done, resume_data, subgraph_done, subgraph_result, subgraph_error FROM microbus_steps WHERE step_id=?",
					stepID,
				).Scan(&stepDepth, &taskName, &stateJSON, &priorChangesJSON, &breakpointHit, &attempt, &lineageID, &flowID, &timeBudgetMs, &interruptDone, &resumeDataJSON, &subgraphDone, &subgraphResultJSON, &subgraphErrorStr)
				if err == sql.ErrNoRows {
					return nil
				}
				return errors.Trace(err)
			},
		)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if n == 0 {
		return nil // Already claimed or not yet due
	}
	if flowID == 0 {
		return nil // Step row gone
	}

	var flowToken string
	var flowStatus string
	var workflowName string
	var graphJSON string
	var actorClaimsJSON string
	var traceParent string
	var notifyHostname string
	var breakpointsJSON string
	var flowCreatedAt time.Time
	var flowUpdatedAt time.Time
	var flowPriority int
	var flowFairnessKey string
	var flowFairnessWeight float64
	err = db.QueryRowContext(ctx,
		"SELECT flow_token, status, workflow_name, graph, actor_claims, trace_parent, notify_hostname, breakpoints, created_at, updated_at, priority, fairness_key, fairness_weight FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&flowToken, &flowStatus, &workflowName, &graphJSON, &actorClaimsJSON, &traceParent, &notifyHostname, &breakpointsJSON, &flowCreatedAt, &flowUpdatedAt, &flowPriority, &flowFairnessKey, &flowFairnessWeight)
	if err != nil {
		return errors.Trace(err)
	}

	// Terminal flow check: Cancel, failStep, and flow completion set the flow to a terminal status first, then update steps.
	// If this worker claimed the step before the step update, catch it here.
	flowStatus = strings.TrimSpace(flowStatus)
	flowToken = strings.TrimSpace(flowToken)
	if flowStatus == workflow.StatusCancelled || flowStatus == workflow.StatusFailed || flowStatus == workflow.StatusCompleted {
		// Reset parked alongside the terminal transition: the "step is parked" invariant only
		// holds while the step is actively waiting (parkedSubgraph or parkedBreaker). Once the
		// step is terminal, the park slot is by definition gone - leaving parked != 0 would
		// strand the row outside the selection index for any subsequent Restart/RestartFrom.
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, parked=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=?",
			flowStatus, parkedNone, stepID,
		)
		return errors.Trace(err)
	}

	// Deserialize graph (or reuse the cached parse - graphJSON is frozen at
	// flow creation, so every step of the same flow sees identical bytes).
	graphKey := graphCacheKey{shard: shardNum, flowID: flowID}
	graph, cached := svc.graphCache.Load(graphKey)
	if !cached {
		graph = &workflow.Graph{}
		err = json.Unmarshal([]byte(graphJSON), graph)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		svc.graphCache.Store(graphKey, graph)
	}

	// Build the Flow carrier
	var state map[string]any
	err = unmarshalJSONMap(stateJSON, &state)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}
	var priorChanges map[string]any
	err = unmarshalJSONMap(priorChangesJSON, &priorChanges)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}
	externalFlowID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, strings.TrimSpace(flowToken))
	// Merge state+priorChanges so the task sees the accumulated state from all prior executions.
	// The state column is invariant after step creation; all mutations accumulate in changes.
	mergedInputState, err := workflow.MergeState(state, priorChanges, nil)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}
	flow := workflow.NewRawFlow()
	flow.SetRawState(mergedInputState)
	flow.SetRawChanges(priorChanges)
	flow.SetAttempt(attempt)
	flow.SetTimestamps(flowCreatedAt, flowUpdatedAt)

	// Materialize an interrupt park's resolution from the step row so flow.Interrupt returns the
	// resume data (yield=false) on re-entry instead of re-arming. Only a resumed step has
	// interrupt_done set; an un-resumed step (the common case) needs no materialization.
	if interruptDone {
		var resumeData map[string]any
		err = unmarshalJSONMap(resumeDataJSON, &resumeData)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		flow.SetInterruptResolution(resumeData)
	}
	// Likewise materialize a dynamic subgraph park's resolution so flow.Subgraph returns the child's
	// result/error (yield=false) on re-entry. Only a step whose child has terminated has subgraph_done set.
	if subgraphDone {
		var subgraphResult map[string]any
		err = unmarshalJSONMap(subgraphResultJSON, &subgraphResult)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		flow.SetSubgraphResolution(subgraphResult, subgraphErrorStr)
	}

	// Check breakpoints: pause before executing the task if a breakpoint matches.
	// Skip if this step already hit a breakpoint (breakpoint_hit flag prevents re-triggering on Resume).
	if !breakpointHit {
		var breakpoints map[string]string
		err := unmarshalJSONMap(breakpointsJSON, &breakpoints)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		// URL fallback fires only when exactly one node uses that URL; aliases must
		// be addressed by node name.
		breakpointMatch := breakpoints[taskName] == "b"
		if !breakpointMatch {
			if u := graph.URLOf(taskName); u != "" && breakpoints[u] == "b" && len(graph.NamesForURL(u)) == 1 {
				breakpointMatch = true
			}
		}
		if len(breakpoints) > 0 && breakpointMatch {
			svc.LogDebug(ctx, "Breakpoint hit", "task", taskName, "step", stepDepth, "flow", workflowName)

			// Build the surgraph chain to interrupt all parent flows atomically
			chainFlowIDs, chainStepIDs, chainCompositeIDs, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}

			// Atomically interrupt all flows and steps in the chain
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return errors.Trace(err)
			}
			defer tx.Rollback()

			// Interrupt all flows in the chain
			flowPlaceholders := strings.Repeat("?,", len(chainFlowIDs)-1) + "?"
			flowArgs := append([]any{workflow.StatusInterrupted}, chainFlowIDs...)
			flowArgs = append(flowArgs, workflow.StatusRunning, workflow.StatusInterrupted)
			_, err = tx.ExecContext(ctx,
				"UPDATE microbus_flows SET status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status IN (?, ?)",
				flowArgs...,
			)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}

			// Interrupt all steps in the chain (current step + parked surgraph steps)
			allStepIDs := append([]any{stepID}, chainStepIDs...)
			stepPlaceholders := strings.Repeat("?,", len(allStepIDs)-1) + "?"
			stepArgs := append([]any{workflow.StatusInterrupted}, allStepIDs...)
			stepArgs = append(stepArgs, workflow.StatusRunning, workflow.StatusInterrupted)
			_, err = tx.ExecContext(ctx,
				"UPDATE microbus_steps SET status=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id IN ("+stepPlaceholders+") AND status IN (?, ?)",
				stepArgs...,
			)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}

			// Set breakpoint_hit on the current step (prevents re-triggering on Resume)
			_, err = tx.ExecContext(ctx,
				"UPDATE microbus_steps SET breakpoint_hit=1 WHERE step_id=?",
				stepID,
			)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}

			err = tx.Commit()
			if err != nil {
				return errors.Trace(err)
			}

			// Notify status change for all flows in the chain (outside the transaction)
			for _, compositeID := range chainCompositeIDs {
				foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusInterrupted)
			}
			// Fire OnFlowStopped on the root flow's notify hostname (if set)
			rootFlowID := chainFlowIDs[len(chainFlowIDs)-1]
			rootCompositeID := chainCompositeIDs[len(chainCompositeIDs)-1]
			var rootNotifyHostname string
			db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", rootFlowID).Scan(&rootNotifyHostname)
			rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
			if rootNotifyHostname != "" {
				foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, &workflow.FlowOutcome{
					FlowKey: rootCompositeID,
					Status:  workflow.StatusInterrupted,
				})
			}

			svc.IncrementStepsExecuted(ctx, 1, taskName, workflow.StatusInterrupted)
			return nil
		}
	}

	// Create a child span under the flow's trace
	taskCtx := injectTraceParent(ctx, traceParent)
	taskCtx, taskSpan := svc.StartSpan(taskCtx, fmt.Sprintf("step %d", stepDepth),
		trc.Internal(),
		trc.String("workflow.id", externalFlowID),
		trc.String("workflow.name", workflowName),
		trc.Int("workflow.step", stepDepth),
	)
	defer taskSpan.End()

	var resultFlow *workflow.RawFlow
	var actorClaims map[string]any
	errorRouted := false
	errStatusCode := 0
	var actorToken string
	var execErr error

	// Mint an access token from the original actor's claims
	err = unmarshalJSONMap(actorClaimsJSON, &actorClaims)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}
	if len(actorClaims) > 0 {
		iss, _ := actorClaims["iss"].(string)
		iss = stripProto(iss)
		actorClaims["iss"] = actorClaims["idp"]
		delete(actorClaims, "idp")
		actorToken, err = accesstokenapi.NewClient(svc).ForHost(iss).Mint(ctx, actorClaims)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
	}

	// Execute the task. taskName is the graph node name; the dispatch URL is resolved
	// via the graph so a node can address a different URL than its name (used when the
	// same task is reused at multiple positions in the graph). If the task's breaker is
	// tripped, this dispatch IS the probe (only probes are parked=0 for tripped tasks, so
	// selection couldn't have returned anything else) - advance the schedule before dispatch
	// so the cluster's nextProbeAt reflects "a probe is in flight now". Doing it here, once
	// per probe admission, matches the rate-limiting the old breakerAdmits gate provided;
	// late-arriving 404s from in-flight dispatches that pre-dated the trip do not (and must
	// not) advance the schedule again - they're not probes.
	svc.LogDebug(ctx, "Executing task", "task", taskName, "flow", workflowName)
	svc.breakerCommit(taskName)
	resultFlow, execErr = svc.executeTask(taskCtx, dispatchURLOf(graph, taskName), flow, actorToken, time.Duration(timeBudgetMs)*time.Millisecond)
	if execErr != nil {
		// 429 -> the step is bounced back to pending, not failed.
		// Runs before the OnError check: the task never saw this 429.
		if errors.StatusCode(execErr) == http.StatusTooManyRequests {
			err := svc.handleBackpressure(ctx, shardNum, stepID, taskName, workflowName)
			return errors.Trace(err)
		}
		// 404 ack-timeout / 503 Service Unavailable / 529 Site Overloaded (529 is
		// non-standard, used by Cloudflare and some third-party APIs) -> the step is
		// bounced; the breaker, not the step, gates the retry. All three want
		// exponential backoff because rate-cutting (the 429 path) keeps probing a
		// downstream that is genuinely unreachable, in maintenance, or
		// capacity-collapsed, drives the rate to zero, and then re-floods on the
		// recovery curve. The breaker's trip+exponential probe schedule lets the
		// downstream actually recover before we send the next request.
		// A handler-emitted 404 lacks the "ack timeout" prefix and falls through.
		var breakerCause string
		switch {
		case errors.StatusCode(execErr) == http.StatusNotFound && strings.HasPrefix(execErr.Error(), "ack timeout"):
			breakerCause = breakerCauseAckTimeout
		case errors.StatusCode(execErr) == http.StatusServiceUnavailable:
			breakerCause = breakerCauseUnavailable
		case errors.StatusCode(execErr) == 529:
			breakerCause = breakerCauseOverloaded
		}
		if breakerCause != "" {
			err := svc.handleBreakerTrip(ctx, shardNum, stepID, taskName, workflowName, breakerCause)
			return errors.Trace(err)
		}
		// Record the input state on the span. Use a distinct name so the
		// dispatch err (driving OnError/OnTimeout routing below) is not
		// shadowed by a nil from a successful MergeState.
		inputState, err := workflow.MergeState(state, priorChanges, nil)
		if err != nil {
			return errors.Trace(err)
		}
		for k, v := range inputState {
			taskSpan.SetAttributes("workflow.state."+k, v)
		}
		taskSpan.SetError(execErr)

		// Check for error transition before failing the flow
		if _, ok := graph.ErrorTransition(taskName); ok {
			svc.LogDebug(ctx, "Task error routed", "task", taskName, "flow", workflowName, "error", execErr)
			taskSpan.SetAttributes("workflow.command", "onError")
			errorRouted = true

			// Serialize the error as a TracedError into a synthetic result flow
			tracedErr := errors.Convert(execErr)
			errStatusCode = tracedErr.StatusCode
			resultFlow = workflow.NewRawFlow()
			resultFlow.SetRawState(state)
			resultFlow.SetRawChanges(nil)
			resultFlow.Set("onErr", tracedErr)
			goto postExecution
		}

		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, execErr, taskName)
		return errors.Trace(execErr)
	}

	// Close the breaker if this dispatch was a probe; no-op otherwise. Per-shard scope so
	// recovery is staggered across shards instead of a unison release.
	svc.breakerClose(ctx, taskName, shardNum)

	// Concurrent-cancel race is handled by two downstream guards instead of a
	// dedicated SELECT here: the step-complete UPDATE below gates on
	// status!=cancelled, so a Cancel that already cancelled this step is a
	// harmless no-op; and any next steps created by the transition evaluation
	// are caught by the terminal-flow check at the top of processStep on their
	// first execution. The step we just ran is recorded as completed (the task
	// did run), which is more faithful to history than overwriting it with
	// cancelled.

postExecution:
	// Accumulate this execution's changes on top of prior changes.
	// The state column is invariant; all mutations accumulate in the changes column.
	// Short-circuit when the task produced no new changes: the accumulated set is
	// just the prior set, and its JSON is already in priorChangesJSON.
	var accumulatedChanges map[string]any
	var changesJSON []byte
	rawChanges := resultFlow.RawChanges()
	if len(rawChanges) == 0 {
		accumulatedChanges = priorChanges
		changesJSON = []byte(priorChangesJSON)
	} else {
		accumulatedChanges, _ = workflow.MergeState(priorChanges, rawChanges, nil)
		changesJSON, _ = json.Marshal(accumulatedChanges)
	}

	// Fail the step if multiple competing control signals are set.
	// Sleep is excluded - it modifies timing, not control flow.
	{
		signalCount := 0
		if _, interrupted := resultFlow.InterruptRequested(); interrupted {
			signalCount++
		}
		if _, _, _, _, retryRequested := resultFlow.RetryRequested(); retryRequested {
			signalCount++
		}
		if resultFlow.GotoRequested() != "" {
			signalCount++
		}
		if _, _, ok := resultFlow.SubgraphRequested(); ok {
			signalCount++
		}
		if signalCount > 1 {
			err = errors.New("task '%s' set multiple competing control signals", taskName)
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
	}

	// Single-park guard: a step parks at most once, interrupt XOR subgraph, and never re-parks a slot
	// that already resolved. interruptDone/subgraphDone are this step's resolution flags materialized
	// from the row above. A resolved parker returns its cached value without re-arming, so a same-kind
	// re-arm cannot reach here; this catches a task that arms the OTHER kind of park on re-entry (a
	// determinism-contract violation). Arming both kinds in one dispatch is already caught above.
	{
		_, interruptArmed := resultFlow.InterruptRequested()
		_, _, subgraphArmed := resultFlow.SubgraphRequested()
		if (interruptArmed || subgraphArmed) && (interruptDone || subgraphDone) {
			err = errors.New("task '%s' armed a second park on an already-resolved step", taskName)
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
	}

	// Handle interrupt (sleep is irrelevant).
	// Set flow to interrupted first so that if we crash before updating the step,
	// the step's lease expires, pollPendingSteps resets it to pending,
	// and re-execution will produce the interrupt again.
	if interruptPayload, interrupted := resultFlow.InterruptRequested(); interrupted {
		svc.LogDebug(ctx, "Task interrupted", "task", taskName, "flow", workflowName)
		taskSpan.SetAttributes("workflow.command", "interrupt")

		// Build the surgraph chain to interrupt all parent flows atomically
		chainFlowIDs, chainStepIDs, chainCompositeIDs, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
		if err != nil {
			return errors.Trace(err)
		}

		// Atomically interrupt all flows and steps in the chain
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errors.Trace(err)
		}
		defer tx.Rollback()

		// Interrupt all flows in the chain
		flowPlaceholders := strings.Repeat("?,", len(chainFlowIDs)-1) + "?"
		flowArgs := append([]any{workflow.StatusInterrupted}, chainFlowIDs...)
		flowArgs = append(flowArgs, workflow.StatusRunning, workflow.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status IN (?, ?)",
			flowArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}

		// Interrupt all steps in the chain, persisting changes on the current step via CASE and
		// flagging interrupt_done on the current step (the one that armed flow.Interrupt) so its
		// re-dispatch after Resume returns the resume data instead of re-arming. Parent surgraph
		// steps in the chain are parked for a subgraph, not an interrupt, so they keep interrupt_done=0.
		// parked=0 on every step in the chain: surgraph parents were parked=1 (subgraph park) and
		// need to clear since the interrupt status is the discriminator now; the leaf was parked=0
		// already so this is a no-op for it. Resume re-parks the parents back to parked=1.
		allStepIDs := append([]any{stepID}, chainStepIDs...)
		stepPlaceholders := strings.Repeat("?,", len(allStepIDs)-1) + "?"
		stepArgs := []any{stepID, string(changesJSON), stepID, workflow.StatusInterrupted, parkedNone}
		stepArgs = append(stepArgs, allStepIDs...)
		stepArgs = append(stepArgs, workflow.StatusRunning, workflow.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET changes=CASE WHEN step_id=? THEN ? ELSE changes END, interrupt_done=CASE WHEN step_id=? THEN 1 ELSE interrupt_done END, status=?, parked=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id IN ("+stepPlaceholders+") AND status IN (?, ?)",
			stepArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}

		// Propagate interrupt payload to all steps in the chain.
		// The WHERE guard ensures only the first interrupt in a fan-out writes the payload.
		if len(interruptPayload) > 0 {
			payloadJSON, _ := json.Marshal(interruptPayload)
			payloadArgs := []any{string(payloadJSON)}
			payloadArgs = append(payloadArgs, allStepIDs...)
			_, err = tx.ExecContext(ctx,
				"UPDATE microbus_steps SET interrupt_payload=? WHERE step_id IN ("+stepPlaceholders+") AND interrupt_payload='{}'",
				payloadArgs...,
			)
			if err != nil {
				return errors.Trace(err)
			}
		}

		err = tx.Commit()
		if err != nil {
			return errors.Trace(err)
		}

		// Notify status change for all flows in the chain (outside the transaction)
		for _, compositeID := range chainCompositeIDs {
			foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusInterrupted)
		}
		// Fire OnFlowStopped on the root flow's notify hostname (if set)
		rootFlowID := chainFlowIDs[len(chainFlowIDs)-1]
		rootCompositeID := chainCompositeIDs[len(chainCompositeIDs)-1]
		var rootNotifyHostname string
		db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", rootFlowID).Scan(&rootNotifyHostname)
		rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
		if rootNotifyHostname != "" {
			foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, &workflow.FlowOutcome{
				FlowKey:          rootCompositeID,
				Status:           workflow.StatusInterrupted,
				InterruptPayload: interruptPayload,
			})
		}

		svc.IncrementStepsExecuted(ctx, 1, taskName, workflow.StatusInterrupted)
		return nil
	}

	// Handle dynamic subgraph signal.
	// Like Interrupt, the step is parked while the child workflow runs.
	// When the child completes, completeSurgraphFlow merges results into this step's
	// changes and sets it to PENDING. The foreman picks it up and re-executes the task -
	// the task sees the child's output in state and returns normally without signaling again.
	if subgraphWorkflow, subgraphInput, subgraphRequested := resultFlow.SubgraphRequested(); subgraphRequested {
		svc.LogDebug(ctx, "Task requested subgraph", "task", taskName, "flow", workflowName, "subgraph", subgraphWorkflow)
		taskSpan.SetAttributes("workflow.command", "subgraph")

		// Persist accumulated changes (state column is invariant).
		// On crash recovery before the child is created, the step will be re-executed
		// and the task will see its prior changes via the merged state built by the flow builder.
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET changes=?, updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			string(changesJSON), stepID, workflow.StatusRunning,
		)
		if err != nil {
			return errors.Trace(err)
		}

		// Fetch the child graph
		subgraphGraph, err := svc.fetchGraph(ctx, subgraphWorkflow)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}

		// Build the child's initial state: ONLY the explicit input passed to flow.Subgraph.
		// Subgraph is a function call - the parent's state does not cross the boundary by
		// default. A caller that wants the parent's full state can pass flow.Snapshot() (or
		// a derived map) as input. nil input means "no arguments" (empty state).
		childInputState := subgraphInput
		if childInputState == nil {
			childInputState = map[string]any{}
		}

		// Create and start the child flow
		subgraphFlowKey, err := svc.createSubgraphFlow(ctx, shardNum, flowID, stepDepth, stepID, subgraphWorkflow, subgraphGraph, childInputState, actorClaimsJSON, traceParent, breakpointsJSON)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		err = svc.Start(ctx, subgraphFlowKey)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}

		// Park the step. Status stays 'running' (the step IS running, logically, just waiting
		// on its child) but parked=1 takes it out of the selection index, the saturation count,
		// and pollPendingSteps' lease-expiry recovery. completeSurgraphFlow flips it back to
		// (status='pending', parked=0) when the child resolves.
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET parked=?, updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			parkedSubgraph, stepID, workflow.StatusRunning,
		)
		if err != nil {
			return errors.Trace(err)
		}
		svc.IncrementStepsExecuted(ctx, 1, taskName, "subgraph")
		return nil
	}

	// Extract sleep duration upfront - applies to both retry and normal advancement
	sleepDur := resultFlow.SleepRequested()
	if sleepDur > 0 {
		taskSpan.SetAttributes("workflow.sleep", sleepDur)
	}

	// Handle retry (with optional sleep for backoff)
	if maxAttempts, initialDelay, multiplier, maxDelay, retryRequested := resultFlow.RetryRequested(); retryRequested {
		taskSpan.SetAttributes("workflow.command", "retry")

		// Compute sleep delay: use backoff parameters if present, otherwise use flow.Sleep()
		retrySleepMs := sleepDur.Milliseconds()
		if maxAttempts > 0 {
			delay := float64(initialDelay)
			if multiplier > 0 {
				for range attempt {
					delay *= multiplier
				}
			}
			if maxDelay > 0 && time.Duration(delay) > maxDelay {
				delay = float64(maxDelay)
			}
			retrySleepMs = time.Duration(delay).Milliseconds()
		}
		svc.LogDebug(ctx, "Task retried", "task", taskName, "flow", workflowName, "step", stepID,
			"attempt", attempt, "maxAttempts", maxAttempts, "delayMs", retrySleepMs)

		// State column is invariant. Accumulated changes already include this execution's output.
		// On the next attempt, the flow builder merges state+changes so the task sees everything.
		// Clear the park slot so the re-dispatch re-arms it: a retry after a resolved Subgraph re-runs
		// the child, a retry after a resolved Interrupt re-interrupts. A no-op when the step never parked.
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, changes=?, attempt=?, not_before=DATE_ADD_MILLIS(NOW_UTC(), ?), lease_expires=NOW_UTC(), updated_at=NOW_UTC(), interrupt_done=0, resume_data='{}', subgraph_done=0, subgraph_result='{}', subgraph_error='' WHERE step_id=?",
			workflow.StatusPending, string(changesJSON), attempt+1, retrySleepMs, stepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
		svc.IncrementStepsExecuted(ctx, 1, taskName, "retried")
		if retrySleepMs > 0 {
			svc.shortenNextPoll(time.Now().Add(time.Duration(retrySleepMs) * time.Millisecond))
		} else {
			foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, stepID)
		}
		return nil
	}

	// Persist changes and mark step as completed.
	// Note: if the process crashes after this UPDATE but before the next-step transaction commits,
	// the step is completed but no successor exists. This is a narrow window (~microseconds) and
	// is acceptable for the simplification gained by removing the COMPLETING intermediate status.
	if errorRouted {
		svc.LogDebug(ctx, "Task error routed", "task", taskName, "flow", workflowName)
		svc.IncrementStepsExecuted(ctx, 1, taskName, "error_routed")
	} else {
		svc.LogDebug(ctx, "Task completed", "task", taskName, "flow", workflowName)
		svc.IncrementStepsExecuted(ctx, 1, taskName, workflow.StatusCompleted)
		taskSpan.SetAttributes("workflow.command", "next")
	}
	gotoTarget := resultFlow.GotoRequested()
	stepRes, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, changes=?, goto_next=?, updated_at=NOW_UTC() WHERE step_id=? AND status!=?",
		workflow.StatusCompleted, string(changesJSON), gotoTarget, stepID, workflow.StatusCancelled,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := stepRes.RowsAffected(); n == 0 {
		return nil // Step was cancelled concurrently
	}

	// Lineage-based advancement. See workflow/CLAUDE.md "Fan-in is explicit via SetFanIn".
	// OnError no longer cancels cohort siblings - they keep running, and the cohort's eventual
	// fan-in resolution (via cohort_failures) decides whether the flow fails.

	var nextTasks []nextStep
	if errorRouted {
		nextTasks, err = svc.evaluateErrorTransitions(graph, taskName, resultFlow, errStatusCode)
	} else {
		nextTasks, err = svc.evaluateTransitions(graph, taskName, resultFlow)
	}
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}

	var realTasks []nextStep
	for _, t := range nextTasks {
		if t.taskName != "" && t.taskName != workflow.END {
			realTasks = append(realTasks, t)
		}
	}

	// isPushTransition reflects whether THIS dispatch pushes a new lineage frame.
	// Static fan-out source + normal/forEach evaluation = push; Goto and OnError
	// transitions stay in the source's scope regardless.
	isPushTransition := graph.IsFanOutSource(taskName) && !errorRouted && resultFlow.GotoRequested() == ""
	cohortSize := len(realTasks)

	// Empty cohort: a fan-out source spawned no branches. Fire the FanIn directly.
	if isPushTransition && cohortSize == 0 {
		fanInTarget := graph.FanInFor(taskName)
		if fanInTarget == "" {
			return svc.completeFlowSequential(ctx, shardNum, db, flowID, flowToken, stepID, notifyHostname, workflowName)
		}
		return svc.fireFanInDirect(ctx, shardNum, db, flowID, stepID, stepDepth, lineageID, fanInTarget, sleepDur, flowPriority, flowFairnessKey, flowFairnessWeight)
	}

	if cohortSize == 0 {
		return svc.completeFlowSequential(ctx, shardNum, db, flowID, flowToken, stepID, notifyHostname, workflowName)
	}

	// Cohort spawn: ourselves if we're pushing a frame, our lineage_id otherwise.
	cohortSpawnID := lineageID
	childLineageID := lineageID
	if isPushTransition {
		cohortSpawnID = stepID
		childLineageID = stepID
	}

	// Partition into FanIn arrivals and normal next steps.
	var normalNexts []nextStep
	var fanInTaskName string
	fanInArrivals := 0
	for _, next := range realTasks {
		if graph.IsFanIn(next.taskName) {
			fanInTaskName = next.taskName
			fanInArrivals++
		} else {
			normalNexts = append(normalNexts, next)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Acquire write lock on the flow row to serialize concurrent fan-in arrivals.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET updated_at=NOW_UTC() WHERE flow_id=?",
		flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Compute the children's input state from our state+changes using in-memory
	// values: state is invariant after step creation (loaded in the parallel
	// block at the top of processStep), and changes is what we just wrote one
	// statement above via UPDATE microbus_steps SET changes=?. Re-reading them
	// from the row would just return these same values.
	childInputState, err := workflow.MergeState(state, accumulatedChanges, nil)
	if err != nil {
		return errors.Trace(err)
	}
	childInputJSON, _ := json.Marshal(childInputState)

	nextStepDepth := stepDepth + 1
	sleepMs := sleepDur.Milliseconds()
	var newStepIDs []int

	for i, next := range normalNexts {
		stepStateJSON := childInputJSON
		if next.item != nil {
			perStepState := make(map[string]any, len(childInputState)+3)
			maps.Copy(perStepState, childInputState)
			// Drop the source array from the branch's state so it doesn't get copied into
			// every branch (an N-element forEach would otherwise carry N copies forward).
			// The array stays in the spawn step's immutable state and is restored by the
			// fan-in merge. Guard against as == forEach so we don't strip what we're about
			// to set.
			if next.forEachKey != "" && next.forEachKey != next.itemKey {
				delete(perStepState, next.forEachKey)
			}
			perStepState[next.itemKey] = next.item
			if next.forEachKey != "" {
				perStepState[next.itemKey+"Index"] = next.cohortIndex
				perStepState[next.itemKey+"Count"] = next.cohortCount
			}
			stepStateJSON, _ = json.Marshal(perStepState)
		}
		nextTimeBudget := svc.taskTimeBudget()
		// fan_out_ordinal records this branch's position in the fan-out (forEach array
		// index, or static fan-out declaration order) so fan-in can merge in that order.
		// predecessor_id is the step that spawned this one (the current step), so the
		// execution-DAG edge is recorded on the child side (covers linear and fan-out).
		newStepID, err := tx.InsertReturnID(ctx, "step_id",
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, parked, time_budget_ms, lineage_id, fan_out_ordinal, predecessor_id, not_before, priority, fairness_key, fairness_weight)"+
				" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), ?, ?, ?)",
			flowID, nextStepDepth, utils.RandomIdentifier(16), next.taskName, string(stepStateJSON), workflow.StatusPending, svc.initialParkedFor(next.taskName), nextTimeBudget.Milliseconds(), childLineageID, i, stepID, sleepMs, flowPriority, flowFairnessKey, flowFairnessWeight,
		)
		if err != nil {
			return errors.Trace(err)
		}
		newStepIDs = append(newStepIDs, int(newStepID))
	}

	// Record the forward edge on the source step: successor_id points to the first
	// next step (linear: the single next; fan-out: the first child). The full fan-out
	// edge set is recovered from each child's predecessor_id.
	if len(newStepIDs) > 0 {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET successor_id=? WHERE step_id=?",
			newStepIDs[0], stepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Set cohort_size on the spawn step the first time we fan out from it.
	if isPushTransition {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET cohort_size=? WHERE step_id=?",
			cohortSize, stepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
		svc.LogDebug(ctx, "Fan-out cohort spawned", "flow", flowID, "spawnStep", stepID, "task", taskName, "cohortSize", cohortSize, "childLineage", childLineageID)
	}

	// Increment cohort_arrivals for each direct FanIn arrival from this transition. If the
	// cohort is now fully resolved, fire the fan-in step normally when no failures occurred,
	// otherwise propagate the failure up the cohort chain (and fail the flow if it reaches root).
	flowFailed := false
	flowFailedErr := ""
	flowFailedFinalState := ""
	if fanInArrivals > 0 {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET cohort_arrivals = cohort_arrivals + ? WHERE step_id=?",
			fanInArrivals, cohortSpawnID,
		)
		if err != nil {
			return errors.Trace(err)
		}
		var arrivals, size, failures, spawnLineageID int
		err = tx.QueryRowContext(ctx,
			"SELECT cohort_arrivals, cohort_size, cohort_failures, lineage_id FROM microbus_steps WHERE step_id=?",
			cohortSpawnID,
		).Scan(&arrivals, &size, &failures, &spawnLineageID)
		if err != nil {
			return errors.Trace(err)
		}
		fullyResolved := size > 0 && arrivals >= size
		fire := fullyResolved && failures == 0
		svc.LogDebug(ctx, "Fan-in arrival", "flow", flowID, "spawnCohort", cohortSpawnID, "delta", fanInArrivals,
			"arrivals", arrivals, "size", size, "failures", failures, "fire", fire, "fanInTask", fanInTaskName,
			"byStep", stepID, "task", taskName, "lineage", lineageID, "push", isPushTransition)
		if fire {
			fanInStepID, err := svc.insertFanInStep(ctx, tx, flowID, nextStepDepth, cohortSpawnID, stepID, fanInTaskName, graph, sleepMs, flowPriority, flowFairnessKey, flowFairnessWeight)
			if err != nil {
				return errors.Trace(err)
			}
			newStepIDs = append(newStepIDs, fanInStepID)
		} else if fullyResolved && failures > 0 {
			// Cohort closed with failures. Walk up the cohort chain from this spawn's parent.
			var failFlow bool
			if spawnLineageID == 0 {
				failFlow = true
			} else {
				failFlow, err = svc.propagateCohortFailure(ctx, tx, spawnLineageID)
				if err != nil {
					return errors.Trace(err)
				}
			}
			if failFlow {
				var sampleErr string
				tx.QueryRowContext(ctx,
					"SELECT error FROM microbus_steps WHERE flow_id=? AND status=? AND error!='' LIMIT 1",
					flowID, workflow.StatusFailed,
				).Scan(&sampleErr)
				sampleErr = strings.TrimSpace(sampleErr)
				if sampleErr == "" {
					sampleErr = "cohort failed"
				}
				finalStateJSON, _, err := svc.computeFinalState(ctx, tx, flowID)
				if err != nil {
					return errors.Trace(err)
				}
				_, err = tx.ExecContext(ctx,
					"UPDATE microbus_flows SET final_state=?, status=?, error=?, updated_at=NOW_UTC() WHERE flow_id=? AND status NOT IN (?, ?, ?)",
					finalStateJSON, workflow.StatusFailed, sampleErr, flowID,
					workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled,
				)
				if err != nil {
					return errors.Trace(err)
				}
				flowFailed = true
				flowFailedErr = sampleErr
				flowFailedFinalState = finalStateJSON
			}
		}
	}

	nextFlowStepID := 0
	if len(newStepIDs) == 1 {
		nextFlowStepID = newStepIDs[0]
	}
	if !flowFailed {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
			nextFlowStepID, flowID,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	if flowFailed {
		compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
		notifyHostnameTrimmed := strings.TrimSpace(notifyHostname)
		if notifyHostnameTrimmed != "" {
			var finalState map[string]any
			if err := json.Unmarshal([]byte(flowFailedFinalState), &finalState); err == nil {
				foremanapi.NewMulticastTrigger(svc).ForHost(notifyHostnameTrimmed).OnFlowStopped(ctx, &workflow.FlowOutcome{
					FlowKey: compositeID,
					Status:  workflow.StatusFailed,
					State:   finalState,
					Error:   flowFailedErr,
				})
			}
		}
		svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "to", workflow.StatusFailed)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusFailed)
		return nil
	}

	if sleepDur > 0 {
		svc.shortenNextPoll(time.Now().Add(sleepDur))
	} else if len(newStepIDs) > 0 {
		foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, newStepIDs[0])
	}
	return nil
}

// taskTimeBudget returns the hard ceiling applied to every task step's execution,
// taken from the foreman's TimeBudget config. A task endpoint may declare a
// shorter budget of its own via sub.TimeBudget, enforced by the connector when
// the task handler runs.
func (svc *Service) taskTimeBudget() time.Duration {
	return svc.TimeBudget()
}

// unmarshalJSONMap decodes a JSON-object column into a map, fast-pathing the
// "{}" sentinel that is the schema default for empty JSON columns. For an
// empty or "{}" input the destination is left at its zero value (nil),
// which len/range/maps.Copy and workflow.MergeState all treat as empty.
func unmarshalJSONMap[V any](s string, dst *map[string]V) error {
	if s == "" || s == "{}" {
		return nil
	}
	return json.Unmarshal([]byte(s), dst)
}

// executeTask sends the flow to a task endpoint and returns the resulting flow.
// dispatchURLOf resolves a graph node name to its dispatch URL. Falls back to the name
// itself if the node isn't registered (legacy graphs persisted before the name/URL split,
// where task_name in the DB was the URL). END passes through.
func dispatchURLOf(graph *workflow.Graph, name string) string {
	if name == workflow.END {
		return name
	}
	if u := graph.URLOf(name); u != "" {
		return u
	}
	return name
}

func (svc *Service) executeTask(ctx context.Context, taskName string, flow *workflow.RawFlow, actorToken string, timeBudget time.Duration) (*workflow.RawFlow, error) {
	body, err := json.Marshal(flow)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !strings.Contains(taskName, "://") {
		taskName = "https://" + taskName
	}
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(taskName),
		pub.Body(body),
		pub.ContentType("application/json"),
		pub.Timeout(timeBudget),
	}
	if actorToken != "" {
		opts = append(opts, pub.Token(actorToken))
	}
	httpRes, err := svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var resultFlow workflow.RawFlow
	err = json.NewDecoder(httpRes.Body).Decode(&resultFlow)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &resultFlow, nil
}

// extractTraceParent serializes the trace context from ctx into a W3C traceparent string.
func extractTraceParent(ctx context.Context) string {
	carrier := make(propagation.HeaderCarrier)
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("Traceparent")
}

// injectTraceParent deserializes a W3C traceparent string into the context
// so that subsequent spans are created as children of the stored trace.
func injectTraceParent(ctx context.Context, traceParent string) context.Context {
	if traceParent == "" {
		return ctx
	}
	carrier := make(propagation.HeaderCarrier)
	carrier.Set("Traceparent", traceParent)
	return propagation.TraceContext{}.Extract(ctx, carrier)
}
