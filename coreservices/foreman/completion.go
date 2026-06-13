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
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// createSubgraphFlow creates a subgraph flow for a subgraph transition in "created" status.
// The subgraph flow's surgraph_flow_id and surgraph_step_depth link it back to
// the surgraph for completion propagation. The caller must call Start to begin execution.
func (svc *Service) createSubgraphFlow(ctx context.Context, shardNum int, surgraphFlowID int, surgraphStepDepth int, surgraphStepID int, subgraphWorkflowName string, subgraphGraph *workflow.Graph, surgraphState map[string]any, actorClaimsJSON string, traceParent string, breakpointsJSON string) (subgraphFlowKey string, err error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}

	// The subgraph flow inherits the surgraph flow's priority, fairness, and tenant.
	var inherited workflow.FlowOptions
	var parentTenantID int
	err = db.QueryRowContext(ctx,
		"SELECT priority, fairness_key, fairness_weight, tenant_id FROM microbus_flows WHERE flow_id=?",
		surgraphFlowID,
	).Scan(&inherited.Priority, &inherited.FairnessKey, &inherited.FairnessWeight, &parentTenantID)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Create the subgraph flow via createWithGraph (reuses flow+step INSERT logic). The parent
	// state passes through unfiltered; any adaptation is the workflow author's responsibility via
	// a small upstream task using flow.Transform/Delete/Clear ahead of the subgraph call.
	subgraphFlowKey, err = svc.createWithGraph(ctx, shardNum, subgraphWorkflowName, subgraphGraph, surgraphState, 0, "", &inherited)
	if err != nil {
		return "", errors.Trace(err)
	}
	_, subgraphFlowID, _, err := parseFlowKey(subgraphFlowKey)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Set surgraph linkage (including step_id for unambiguous lookup in completeSurgraphFlow),
	// override actor claims / trace parent, copy breakpoints, inherit tenant.
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_flows SET surgraph_flow_id=?, surgraph_step_depth=?, surgraph_step_id=?, actor_claims=?, trace_parent=?, breakpoints=?, tenant_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		surgraphFlowID, surgraphStepDepth, surgraphStepID, actorClaimsJSON, traceParent, breakpointsJSON, parentTenantID, subgraphFlowID,
	)
	if err != nil {
		// Edge case. The subgraph flow will remain orphaned and eventually purged
		return "", errors.Trace(err)
	}

	return subgraphFlowKey, nil
}

// allSubgraphFlows iteratively finds all active descendant subgraph flows of the given flow.
func (svc *Service) allSubgraphFlows(ctx context.Context, shardNum int, flowID int) (flowIDs []any, compositeFlowIDs []string, err error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	current := []any{flowID}
	for len(current) > 0 {
		placeholders := strings.Repeat("?,", len(current)-1) + "?"
		args := append([]any{}, current...)
		args = append(args, workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled)
		rows, err := db.QueryContext(ctx,
			"SELECT flow_id, flow_token FROM microbus_flows WHERE surgraph_flow_id IN ("+placeholders+") AND status NOT IN (?, ?, ?)",
			args...,
		)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		current = nil
		for rows.Next() {
			var id int
			var token string
			if err := rows.Scan(&id, &token); err != nil {
				rows.Close()
				return nil, nil, errors.Trace(err)
			}
			flowIDs = append(flowIDs, id)
			compositeFlowIDs = append(compositeFlowIDs, fmt.Sprintf("%d-%d-%s", shardNum, id, strings.TrimSpace(token)))
			current = append(current, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, errors.Trace(err)
		}
	}
	return flowIDs, compositeFlowIDs, nil
}

// completeFlowSequential marks the flow completed and the current step completed when no
// successor exists (terminal-END path, lineage mode).
func (svc *Service) completeFlowSequential(ctx context.Context, shardNum int, db *sequel.DB, flowID int, flowToken string, stepID int, notifyHostname, workflowName string) error {
	svc.LogDebug(ctx, "Flow completed", "flow", workflowName)
	_, err := svc.completeFlow(ctx, shardNum, flowID, flowToken, notifyHostname)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE step_id=?",
		workflow.StatusCompleted, stepID,
	)
	return errors.Trace(err)
}

// mergeTerminalSteps computes a flow's terminal state from the execution-DAG tail -
// steps with no recorded outgoing edge (successor_id = 0) - never from step_depth.
// A normally completed flow has completed tail steps (a well-formed graph has exactly
// one, the step that transitioned to END; several only for a fan-out with no fan-in);
// their merged state is the result. A flow force-terminated by Cancel/failStep before
// any step completed has only non-completed tail steps - the in-flight/entry steps
// whose immutable state snapshot is the flow's last-known input; merging those (their
// changes are empty) reproduces that snapshot without consulting step_depth. Returns
// an empty map for a flow with no steps. Reducers apply when tail branches converge.
func (svc *Service) mergeTerminalSteps(ctx context.Context, db sequel.Executor, flowID int, reducers map[string]workflow.Reducer) (map[string]any, error) {
	merge := func(query string, args ...any) (map[string]any, bool, error) {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, false, errors.Trace(err)
		}
		defer rows.Close()

		var baseState map[string]any
		var allChanges []map[string]any
		found := false
		for rows.Next() {
			found = true
			var stateJSON, changesJSON string
			err := rows.Scan(&stateJSON, &changesJSON)
			if err != nil {
				return nil, false, errors.Trace(err)
			}
			if baseState == nil {
				err = json.Unmarshal([]byte(stateJSON), &baseState)
				if err != nil {
					return nil, false, errors.Trace(err)
				}
			}
			var changes map[string]any
			err = json.Unmarshal([]byte(changesJSON), &changes)
			if err != nil {
				return nil, false, errors.Trace(err)
			}
			allChanges = append(allChanges, changes)
		}
		err = rows.Err()
		if err != nil {
			return nil, false, errors.Trace(err)
		}
		if !found {
			return nil, false, nil
		}

		merged := baseState
		for _, changes := range allChanges {
			merged, err = workflow.MergeState(merged, changes, reducers)
			if err != nil {
				return nil, false, errors.Trace(err)
			}
		}
		if merged == nil {
			merged = map[string]any{}
		}
		return merged, true, nil
	}

	// Primary: the completed tail of a normally finishing flow.
	merged, found, err := merge(
		"SELECT state, changes FROM microbus_steps WHERE flow_id=? AND successor_id=0 AND status=? ORDER BY step_id",
		flowID, workflow.StatusCompleted,
	)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if found {
		return merged, nil
	}

	// Force-terminated before any step completed: the non-completed tail (the
	// in-flight/entry steps). Their immutable state snapshot is the flow's
	// last-known input; this stays depth-free.
	merged, found, err = merge(
		"SELECT state, changes FROM microbus_steps WHERE flow_id=? AND successor_id=0 ORDER BY step_id",
		flowID,
	)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if found {
		return merged, nil
	}
	return map[string]any{}, nil
}

// completeFlow atomically transitions a flow to completed status. The UPDATE only
// succeeds if the flow is not already in a terminal status, guaranteeing exactly-once
// notification. It computes and stores the final_state.
// Returns true if this call was the one that completed the flow.
func (svc *Service) completeFlow(ctx context.Context, shardNum int, flowID int, flowToken string, notifyHostname string) (bool, error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return false, errors.Trace(err)
	}
	finalStateJSON, workflowName, err := svc.computeFinalState(ctx, db, flowID)
	if err != nil {
		return false, errors.Trace(err)
	}

	res, err := db.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, final_state=?, updated_at=NOW_UTC() WHERE flow_id=? AND status NOT IN (?, ?, ?)",
		workflow.StatusCompleted, finalStateJSON, flowID,
		workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled,
	)
	if err != nil {
		return false, errors.Trace(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Another goroutine or replica already terminated this flow
		return false, nil
	}
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "to", workflow.StatusCompleted)
	svc.IncrementFlowsTerminated(ctx, 1, workflowName, workflow.StatusCompleted)
	compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
	if notifyHostname != "" {
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStateJSON), &finalState); err == nil {
			foremanapi.NewMulticastTrigger(svc).ForHost(notifyHostname).OnFlowStopped(ctx, &workflow.FlowOutcome{
				FlowKey: compositeID,
				Status:  workflow.StatusCompleted,
				State:   finalState,
			})
		}
	}
	// Wake up all Await callers across all replicas
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusCompleted)

	// Propagate completion back to the surgraph
	var surgraphFlowID int
	var surgraphStepDepth int
	var surgraphStepID int
	err = db.QueryRowContext(ctx,
		"SELECT surgraph_flow_id, surgraph_step_depth, surgraph_step_id FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&surgraphFlowID, &surgraphStepDepth, &surgraphStepID)
	if err != nil {
		return true, errors.Trace(err)
	}
	if surgraphFlowID != 0 {
		if err := svc.completeSurgraphFlow(ctx, shardNum, surgraphFlowID, surgraphStepDepth, surgraphStepID, workflowName, finalStateJSON); err != nil {
			return true, errors.Trace(err)
		}
	}

	return true, nil
}

// completeSurgraphFlow merges a completed subgraph flow's final state into the surgraph step's
// changes and re-enqueues the surgraph step for transition evaluation.
// surgraphStepID identifies the parked surgraph step by primary key; pass 0 for legacy subgraph
// flows created before the surgraph_step_id column was added (then a fallback search is used).
func (svc *Service) completeSurgraphFlow(ctx context.Context, shardNum int, surgraphFlowID int, surgraphStepDepth int, surgraphStepID int, subgraphWorkflowName string, subgraphFinalStateJSON string) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	// Resolve the parked surgraph step by primary key, or a legacy lease-threshold fallback for
	// subgraph flows created before the surgraph_step_id column existed. The threshold (>= 1 hour;
	// far longer than any normal task lease) ensures the SELECT cannot match a sibling step that
	// happens to be momentarily running.
	if surgraphStepID != 0 {
		var existing int
		err = db.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps WHERE step_id=? AND status=?",
			surgraphStepID, workflow.StatusRunning,
		).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Already handled (e.g. cancelled or failed)
		}
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		const surgraphParkedLeaseThresholdMs = 60 * 60 * 1000 // 1 hour
		err = db.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps WHERE flow_id=? AND step_depth=? AND status=? AND lease_expires > DATE_ADD_MILLIS(NOW_UTC(), ?)",
			surgraphFlowID, surgraphStepDepth, workflow.StatusRunning, surgraphParkedLeaseThresholdMs,
		).Scan(&surgraphStepID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Another worker already handled this surgraph step
		}
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Deliver the child's final_state to the parent task via the subgraph_result column. The task
	// re-runs, calls flow.Subgraph again, and receives the result (yield=false) to adopt explicitly.
	resultJSON := subgraphFinalStateJSON
	if strings.TrimSpace(resultJSON) == "" {
		resultJSON = "{}"
	}
	// Guard the revive on the exact park state (running + parkedSubgraph), mirroring deliverSubgraphError.
	// The SELECT above and this UPDATE are separate statements, so a Cancel that cascaded to this caller
	// step in between would otherwise be resurrected to pending - keying on step_id alone overwrites the
	// just-cancelled row. The rows-affected gate then keeps Enqueue off a no-op.
	res, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, subgraph_done=1, subgraph_result=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=? AND parked=?",
		workflow.StatusPending, parkedNone, resultJSON, surgraphStepID, workflow.StatusRunning, parkedSubgraph,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // the surgraph step was cancelled/failed/handled concurrently; nothing to re-dispatch
	}
	svc.LogDebug(ctx, "Resuming surgraph task after subgraph flow completion",
		"surgraphFlow", surgraphFlowID, "surgraphStep", surgraphStepDepth, "subgraph", subgraphWorkflowName)
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, surgraphStepID)
	return nil
}

// interruptedSubgraphChain walks down from the given flow through interrupted subgraph steps to find the
// leaf interrupted step. It returns three parallel lists matching the signature of surgraphChain:
//   - flowIDs: all flow IDs in the chain (starting flow first, leaf flow last)
//   - stepIDs: all interrupted step IDs in the chain (intermediate surgraph steps first, leaf step last)
//   - compositeFlowIDs: external composite IDs (shard-flowID-token) for NotifyStatusChange
//
// The last element of stepIDs is the leaf step. All preceding elements are surgraph steps to re-park.
func (svc *Service) interruptedSubgraphChain(ctx context.Context, shardNum int, flowID int, flowToken string) (flowIDs []any, stepIDs []any, compositeFlowIDs []string, err error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}
	flowIDs = []any{flowID}
	compositeFlowIDs = []string{fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)}

	currentFlowID := flowID
	for {
		// Find the earliest interrupted step in the current flow
		var stepID int
		var stepDepth int
		err = db.QueryRowContext(ctx,
			"SELECT step_id, step_depth FROM microbus_steps WHERE flow_id=? AND status=? ORDER BY updated_at LIMIT_OFFSET(1, 0)",
			currentFlowID, workflow.StatusInterrupted,
		).Scan(&stepID, &stepDepth)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
		stepIDs = append(stepIDs, stepID)

		// Check if this step has an interrupted child subgraph flow
		var subFlowID int
		var subFlowToken string
		err = db.QueryRowContext(ctx,
			"SELECT flow_id, flow_token FROM microbus_flows WHERE surgraph_flow_id=? AND surgraph_step_depth=? AND status=?",
			currentFlowID, stepDepth, workflow.StatusInterrupted,
		).Scan(&subFlowID, &subFlowToken)
		if err == sql.ErrNoRows {
			// No child subgraph - this is the leaf step
			return flowIDs, stepIDs, compositeFlowIDs, nil
		}
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}

		// Descend into the subgraph
		subFlowToken = strings.TrimSpace(subFlowToken)
		flowIDs = append(flowIDs, subFlowID)
		compositeFlowIDs = append(compositeFlowIDs, fmt.Sprintf("%d-%d-%s", shardNum, subFlowID, subFlowToken))
		currentFlowID = subFlowID
	}
}

// surgraphChain walks from the given flow up to the root surgraph, returning three parallel lists:
//   - flowIDs: all flow IDs in the chain (starting flow first, root last)
//   - stepIDs: step IDs of the surgraph steps that launched each subgraph (excludes the starting flow's step)
//   - compositeFlowIDs: external composite IDs (shard-flowID-token) for NotifyStatusChange
func (svc *Service) surgraphChain(ctx context.Context, shardNum int, flowID int, flowToken string) (flowIDs []any, stepIDs []any, compositeFlowIDs []string, err error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}
	flowIDs = []any{flowID}
	compositeFlowIDs = []string{fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)}

	currentFlowID := flowID
	for {
		var surgraphFlowID int
		var surgraphStepDepth int
		var surgraphStepID int
		var workflowName string
		err = db.QueryRowContext(ctx,
			"SELECT surgraph_flow_id, surgraph_step_depth, surgraph_step_id, workflow_name FROM microbus_flows WHERE flow_id=?",
			currentFlowID,
		).Scan(&surgraphFlowID, &surgraphStepDepth, &surgraphStepID, &workflowName)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
		if surgraphFlowID == 0 {
			break // Reached the root flow
		}
		workflowName = strings.TrimSpace(workflowName)
		var surgraphFlowToken string
		err = db.QueryRowContext(ctx,
			"SELECT flow_token FROM microbus_flows WHERE flow_id=?",
			surgraphFlowID,
		).Scan(&surgraphFlowToken)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
		// Legacy fallback for subgraph flows created before surgraph_step_id existed.
		if surgraphStepID == 0 {
			err = db.QueryRowContext(ctx,
				"SELECT step_id FROM microbus_steps WHERE flow_id=? AND step_depth=? AND task_name=?",
				surgraphFlowID, surgraphStepDepth, workflowName,
			).Scan(&surgraphStepID)
			if err != nil {
				return nil, nil, nil, errors.Trace(err)
			}
		}
		flowIDs = append(flowIDs, surgraphFlowID)
		stepIDs = append(stepIDs, surgraphStepID)
		compositeFlowIDs = append(compositeFlowIDs, fmt.Sprintf("%d-%d-%s", shardNum, surgraphFlowID, strings.TrimSpace(surgraphFlowToken)))
		currentFlowID = surgraphFlowID
	}
	return flowIDs, stepIDs, compositeFlowIDs, nil
}

// computeFinalState computes the merged state for a flow.
// It also returns the workflow name from the flows table.
// The db parameter allows this to run inside a transaction when needed.
func (svc *Service) computeFinalState(ctx context.Context, db sequel.Executor, flowID int) (finalStateJSON string, workflowName string, err error) {
	// Get the graph and workflow name to check for declared outputs
	var graphJSON string
	err = db.QueryRowContext(ctx,
		"SELECT graph, workflow_name FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&graphJSON, &workflowName)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	var graph workflow.Graph
	if err := json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		return "", "", errors.Trace(err)
	}

	// The flow's terminal state is the merged state of the execution-DAG tail (the
	// steps with no recorded outgoing edge), never MAX(step_depth) - an intra-thread
	// Goto loop inside a fan-out drives a non-terminal branch to a deeper step_depth
	// than the fan-in/terminal step.
	merged, err := svc.mergeTerminalSteps(ctx, db, flowID, graph.Reducers())
	if err != nil {
		return "", "", errors.Trace(err)
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	return string(data), workflowName, nil
}

// failStep handles a task failure. For a subgraph child it delivers the error to the parent's
// surgraph step. For a cohort member (lineage_id != 0) it marks the step failed and propagates
// the failure up the cohort chain via propagateCohortFailure; the flow row is only failed when
// the propagation reaches the root cohort. For a non-cohort step the flow is failed immediately.
func (svc *Service) failStep(ctx context.Context, shardNum int, stepID int, flowID int, flowToken string, taskErr error, taskName string) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// If the failing flow is a subgraph child (its surgraph parent step called flow.Subgraph), deliver
	// the error to that parent task via subgraph_error instead of cascading the failure up the chain.
	// The parent task receives it from flow.Subgraph's err return and decides what to do (return it -
	// which lands back here from the parent - Retry, or route via OnError).
	dynamicParentStepID, isDynamicChild, err := svc.dynamicSubgraphParent(ctx, db, flowID)
	if err != nil {
		return errors.Trace(err)
	}
	if isDynamicChild {
		return svc.deliverSubgraphError(ctx, shardNum, stepID, flowID, dynamicParentStepID, taskErr)
	}

	// Determine whether this step is a cohort member. A non-cohort step's failure cascades the
	// flow immediately; a cohort member's failure defers to the cohort's fan-in resolution.
	var stepLineageID int
	err = db.QueryRowContext(ctx,
		"SELECT lineage_id FROM microbus_steps WHERE step_id=?",
		stepID,
	).Scan(&stepLineageID)
	if err != nil {
		return errors.Trace(err)
	}

	errMsg := taskErr.Error()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Mark the failed step (common to both paths). parked=parkedNone keeps the terminal-implies-
	// unparked invariant intact even if the failing step happens to have been parked (an unusual
	// case but worth defending - e.g. a hypothetical future code path that fails a parked row).
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, error=?, updated_at=NOW_UTC() WHERE step_id=?",
		workflow.StatusFailed, parkedNone, errMsg, stepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	failFlow := stepLineageID == 0
	if !failFlow {
		failFlow, err = svc.propagateCohortFailure(ctx, tx, stepLineageID)
		if err != nil {
			return errors.Trace(err)
		}
	}

	var finalStateJSON string
	if failFlow {
		finalStateJSON, _, err = svc.computeFinalState(ctx, tx, flowID)
		if err != nil {
			return errors.Trace(err)
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET final_state=?, status=?, error=?, updated_at=NOW_UTC() WHERE flow_id=? AND status NOT IN (?, ?, ?)",
			finalStateJSON, workflow.StatusFailed, errMsg, flowID,
			workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	svc.IncrementStepsExecuted(ctx, 1, taskName, workflow.StatusFailed)

	if !failFlow {
		return nil
	}

	// Notifications (post-commit).
	compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, strings.TrimSpace(flowToken))
	var notifyHostname string
	db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", flowID).Scan(&notifyHostname)
	notifyHostname = strings.TrimSpace(notifyHostname)
	if notifyHostname != "" {
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStateJSON), &finalState); err == nil {
			foremanapi.NewMulticastTrigger(svc).ForHost(notifyHostname).OnFlowStopped(ctx, &workflow.FlowOutcome{
				FlowKey: compositeID,
				Status:  workflow.StatusFailed,
				State:   finalState,
				Error:   errMsg,
			})
		}
	}
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "to", workflow.StatusFailed)
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusFailed)
	return nil
}

// propagateCohortFailure bumps a spawn step's cohort_arrivals and cohort_failures by 1 and, if the
// cohort fully resolves (arrivals == size), walks up to the parent spawn via lineage_id and repeats.
// Returns true when the propagation reaches the root cohort (lineage_id = 0), meaning the caller
// should fail the flow row. Returns false when an ancestor cohort is still waiting for other branches.
func (svc *Service) propagateCohortFailure(ctx context.Context, tx sequel.Executor, spawnStepID int) (failFlow bool, err error) {
	current := spawnStepID
	for {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET cohort_arrivals = cohort_arrivals + 1, cohort_failures = cohort_failures + 1 WHERE step_id=?",
			current,
		)
		if err != nil {
			return false, errors.Trace(err)
		}
		var arrivals, size, lineageID int
		err = tx.QueryRowContext(ctx,
			"SELECT cohort_arrivals, cohort_size, lineage_id FROM microbus_steps WHERE step_id=?",
			current,
		).Scan(&arrivals, &size, &lineageID)
		if err != nil {
			return false, errors.Trace(err)
		}
		if arrivals < size {
			return false, nil
		}
		if lineageID == 0 {
			return true, nil
		}
		current = lineageID
	}
}

// dynamicSubgraphParent reports whether the given flow is a subgraph child - i.e. it has a surgraph parent
// step launched by flow.Subgraph. It returns the parent step's ID when so. A root flow or a legacy row
// without surgraph_step_id returns ok=false (the caller cascades the failure as before).
func (svc *Service) dynamicSubgraphParent(ctx context.Context, db *sequel.DB, flowID int) (parentStepID int, ok bool, err error) {
	var surgraphFlowID, surgraphStepID int
	err = db.QueryRowContext(ctx,
		"SELECT surgraph_flow_id, surgraph_step_id FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&surgraphFlowID, &surgraphStepID)
	if err != nil {
		return 0, false, errors.Trace(err)
	}
	if surgraphFlowID == 0 || surgraphStepID == 0 {
		return 0, false, nil
	}
	return surgraphStepID, true, nil
}

// deliverSubgraphError fails a dynamic subgraph child flow (and its leaf step) and re-dispatches the
// parent's parked flow.Subgraph step with subgraph_error set, so the parent task receives the error from
// flow.Subgraph's err return on re-entry. The parent flow stays running.
func (svc *Service) deliverSubgraphError(ctx context.Context, shardNum int, childStepID int, childFlowID int, parentStepID int, taskErr error) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	errMsg := taskErr.Error()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Fail the child's leaf step. parked=parkedNone keeps the terminal-implies-unparked invariant.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, error=?, updated_at=NOW_UTC() WHERE step_id=?",
		workflow.StatusFailed, parkedNone, errMsg, childStepID,
	)
	if err != nil {
		return errors.Trace(err)
	}
	// Fail the child subgraph flow (terminal) with its computed final_state; its other steps
	// self-terminate via the terminal-flow check in processStep.
	childFinalState, _, err := svc.computeFinalState(ctx, tx, childFlowID)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, error=?, final_state=?, updated_at=NOW_UTC() WHERE flow_id=? AND status NOT IN (?, ?, ?)",
		workflow.StatusFailed, errMsg, childFinalState, childFlowID, workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled,
	)
	if err != nil {
		return errors.Trace(err)
	}
	// Deliver the error to the parent's parked dynamic-subgraph step: set subgraph_error + subgraph_done,
	// clear parked, and reset to pending so the parent task re-runs and receives it from flow.Subgraph.
	// Guard on (parked=parkedSubgraph AND status='running') so a concurrently cancelled/failed parent is
	// not revived; the surgraph park is the only legitimate (running, parked=1) state.
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, subgraph_done=1, subgraph_error=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=? AND parked=?",
		workflow.StatusPending, parkedNone, errMsg, parentStepID, workflow.StatusRunning, parkedSubgraph,
	)
	if err != nil {
		return errors.Trace(err)
	}
	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, parentStepID)
	}
	return nil
}
