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

	// The subgraph flow inherits the surgraph flow's priority and fairness.
	var inherited flowSchedule
	err = db.QueryRowContext(ctx,
		"SELECT priority, fairness_key, fairness_weight FROM microbus_flows WHERE flow_id=?",
		surgraphFlowID,
	).Scan(&inherited.priority, &inherited.fairnessKey, &inherited.fairnessWeight)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Create the subgraph flow via createWithGraph (reuses flow+step INSERT logic)
	// Filter the parent state through the subgraph's declared inputs.
	subgraphState := workflow.FilterState(surgraphState, subgraphGraph.Inputs())
	subgraphFlowKey, err = svc.createWithGraph(ctx, shardNum, subgraphWorkflowName, subgraphGraph, subgraphState, 0, "", inherited)
	if err != nil {
		return "", errors.Trace(err)
	}
	_, subgraphFlowID, _, err := parseFlowKey(subgraphFlowKey)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Set surgraph linkage (including step_id for unambiguous lookup in completeSurgraphFlow),
	// override actor claims / trace parent, and copy breakpoints from surgraph.
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_flows SET surgraph_flow_id=?, surgraph_step_depth=?, surgraph_step_id=?, actor_claims=?, trace_parent=?, breakpoints=?, updated_at=NOW_UTC() WHERE flow_id=?",
		surgraphFlowID, surgraphStepDepth, surgraphStepID, actorClaimsJSON, traceParent, breakpointsJSON, subgraphFlowID,
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
		args = append(args, foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled)
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
		foremanapi.StatusCompleted, stepID,
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
		flowID, foremanapi.StatusCompleted,
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

	// Filter the final state through the graph's declared outputs
	var graphJSON string
	err = db.QueryRowContext(ctx,
		"SELECT graph FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&graphJSON)
	if err != nil {
		return false, errors.Trace(err)
	}
	var graph workflow.Graph
	if err = json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		return false, errors.Trace(err)
	}
	var finalState map[string]any
	if err = json.Unmarshal([]byte(finalStateJSON), &finalState); err != nil {
		return false, errors.Trace(err)
	}
	finalState = workflow.FilterState(finalState, graph.Outputs())
	filteredJSON, _ := json.Marshal(finalState)
	finalStateJSON = string(filteredJSON)

	res, err := db.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, final_state=?, updated_at=NOW_UTC() WHERE flow_id=? AND status NOT IN (?, ?, ?)",
		foremanapi.StatusCompleted, finalStateJSON, flowID,
		foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled,
	)
	if err != nil {
		return false, errors.Trace(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Another goroutine or replica already terminated this flow
		return false, nil
	}
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "to", foremanapi.StatusCompleted)
	svc.IncrementFlowsTerminated(ctx, 1, workflowName, foremanapi.StatusCompleted)
	compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
	if notifyHostname != "" {
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStateJSON), &finalState); err == nil {
			raw := workflow.NewRawFlow()
			raw.SetRawState(finalState)
			foremanapi.NewMulticastTrigger(svc).ForHost(notifyHostname).OnFlowStopped(ctx, compositeID, foremanapi.StatusCompleted, raw.RawState())
		}
	}
	// Wake up all Await callers across all replicas
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, foremanapi.StatusCompleted)

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
	// Look up the surgraph step (by PK or legacy fallback), parent flow graph (for reducers),
	// and subgraph graph (for output filtering) in parallel.
	var surgraphStepChangesJSON string
	var surgraphGraphJSON string
	var subgraphGraphJSON string
	err = svc.Parallel(
		func() error {
			if surgraphStepID != 0 {
				// Deterministic lookup by primary key.
				err := db.QueryRowContext(ctx,
					"SELECT changes FROM microbus_steps WHERE step_id=? AND status=?",
					surgraphStepID, foremanapi.StatusRunning,
				).Scan(&surgraphStepChangesJSON)
				if errors.Is(err, sql.ErrNoRows) {
					surgraphStepID = 0 // Already handled (e.g. cancelled or failed)
					return nil
				}
				return errors.Trace(err)
			}
			// Legacy fallback for subgraph flows created before surgraph_step_id existed.
			// Filter on a parked lease (>= 1 day; far longer than any normal task lease) so
			// the SELECT cannot match a sibling step that happens to be momentarily running.
			const surgraphParkedLeaseThresholdMs = 60 * 60 * 1000 // 1 hour
			err := db.QueryRowContext(ctx,
				"SELECT step_id, changes FROM microbus_steps WHERE flow_id=? AND step_depth=? AND status=? AND lease_expires > DATE_ADD_MILLIS(NOW_UTC(), ?)",
				surgraphFlowID, surgraphStepDepth, foremanapi.StatusRunning, surgraphParkedLeaseThresholdMs,
			).Scan(&surgraphStepID, &surgraphStepChangesJSON)
			if errors.Is(err, sql.ErrNoRows) {
				return nil // Another worker already handled this surgraph step
			}
			return errors.Trace(err)
		},
		func() error {
			err := db.QueryRowContext(ctx,
				"SELECT graph FROM microbus_flows WHERE flow_id=?",
				surgraphFlowID,
			).Scan(&surgraphGraphJSON)
			return errors.Trace(err)
		},
		func() error {
			err := db.QueryRowContext(ctx,
				"SELECT graph FROM microbus_flows WHERE surgraph_flow_id=? AND surgraph_step_depth=? AND workflow_name=? AND status=?",
				surgraphFlowID, surgraphStepDepth, subgraphWorkflowName, foremanapi.StatusCompleted,
			).Scan(&subgraphGraphJSON)
			return errors.Trace(err)
		},
	)
	if err != nil {
		return errors.Trace(err)
	}
	if surgraphStepID == 0 {
		return nil // Another worker already advanced the surgraph step
	}
	var surgraphGraph workflow.Graph
	if err = json.Unmarshal([]byte(surgraphGraphJSON), &surgraphGraph); err != nil {
		return errors.Trace(err)
	}

	// Filter the subgraph's final_state through its declared outputs
	var subgraphGraph workflow.Graph
	if err = json.Unmarshal([]byte(subgraphGraphJSON), &subgraphGraph); err != nil {
		return errors.Trace(err)
	}
	var subgraphFinalState map[string]any
	if err := json.Unmarshal([]byte(subgraphFinalStateJSON), &subgraphFinalState); err != nil {
		return errors.Trace(err)
	}
	subgraphFinalState = workflow.FilterState(subgraphFinalState, subgraphGraph.Outputs())

	// Merge filtered subgraph state into the surgraph step's changes using reducers
	var surgraphChanges map[string]any
	if err := json.Unmarshal([]byte(surgraphStepChangesJSON), &surgraphChanges); err != nil {
		surgraphChanges = make(map[string]any)
	}
	surgraphChanges, err = workflow.MergeState(surgraphChanges, subgraphFinalState, surgraphGraph.Reducers())
	if err != nil {
		return errors.Trace(err)
	}
	mergedChangesJSON, _ := json.Marshal(surgraphChanges)

	// Merge state+changes so that if the step is re-executed as a task (dynamic subgraph),
	// the task sees the accumulated state including the child's output.
	// For static subgraphs this is harmless - the step skips to postExecution without re-running.
	var surgraphStateJSON string
	err = db.QueryRowContext(ctx,
		"SELECT state FROM microbus_steps WHERE step_id=?",
		surgraphStepID,
	).Scan(&surgraphStateJSON)
	if err != nil {
		return errors.Trace(err)
	}
	var surgraphState map[string]any
	if err := json.Unmarshal([]byte(surgraphStateJSON), &surgraphState); err != nil {
		surgraphState = make(map[string]any)
	}
	mergedState, err := workflow.MergeState(surgraphState, surgraphChanges, nil)
	if err != nil {
		return errors.Trace(err)
	}
	mergedStateJSON, _ := json.Marshal(mergedState)

	// Set surgraph step to PENDING with expired lease for immediate pickup.
	// For static subgraphs, processStep detects the completed child and skips to postExecution.
	// For dynamic subgraphs, the task re-runs and sees the merged state.
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, state=?, changes=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=?",
		foremanapi.StatusPending, string(mergedStateJSON), string(mergedChangesJSON), surgraphStepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Enqueue the surgraph step for transition evaluation
	svc.LogDebug(ctx, "Resuming surgraph after subgraph flow completion",
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
			currentFlowID, foremanapi.StatusInterrupted,
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
			currentFlowID, stepDepth, foremanapi.StatusInterrupted,
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

// failStep marks a step, its flow, and all surgraph flows up the chain as failed.
// This is analogous to Cancel - it handles the full upward chain in bulk.
func (svc *Service) failStep(ctx context.Context, shardNum int, stepID int, flowID int, flowToken string, taskErr error, taskName string) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// Build the surgraph chain (current flow + all parent flows up to root)
	chainFlowIDs, chainStepIDs, chainCompositeIDs, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
	if err != nil {
		return errors.Trace(err)
	}

	// Atomically fail all steps, compute final states, and fail all flows
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Fail the current step
	errMsg := taskErr.Error()
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, error=?, updated_at=NOW_UTC() WHERE step_id=?",
		foremanapi.StatusFailed, errMsg, stepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Fail surgraph steps in the chain (with subgraph failure error)
	if len(chainStepIDs) > 0 {
		stepPlaceholders := strings.Repeat("?,", len(chainStepIDs)-1) + "?"
		subgraphErr := "subgraph failed"
		stepArgs := append([]any{foremanapi.StatusFailed, subgraphErr}, chainStepIDs...)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, error=?, updated_at=NOW_UTC() WHERE step_id IN ("+stepPlaceholders+")",
			stepArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Compute final_state for each flow (inside transaction so it reflects the failed steps)
	finalStates := make([]string, len(chainFlowIDs))
	for i, fid := range chainFlowIDs {
		fs, _, err := svc.computeFinalState(ctx, tx, fid.(int))
		if err != nil {
			return errors.Trace(err)
		}
		finalStates[i] = fs
	}

	// Fail all flows in the chain with their computed final_state via CASE
	flowPlaceholders := strings.Repeat("?,", len(chainFlowIDs)-1) + "?"
	caseClause := "CASE"
	flowArgs := []any{}
	for i, fid := range chainFlowIDs {
		caseClause += " WHEN flow_id=? THEN ?"
		flowArgs = append(flowArgs, fid, finalStates[i])
	}
	caseClause += " END"
	flowArgs = append(flowArgs, foremanapi.StatusFailed)
	flowArgs = append(flowArgs, chainFlowIDs...)
	flowArgs = append(flowArgs, foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled)
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET final_state="+caseClause+", status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status NOT IN (?, ?, ?)",
		flowArgs...,
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	// Notifications (outside the transaction)
	// Use the root flow's notify_hostname (last element of the surgraph chain)
	rootIdx := len(chainFlowIDs) - 1
	rootCompositeID := chainCompositeIDs[rootIdx]
	var rootNotifyHostname string
	db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", chainFlowIDs[rootIdx]).Scan(&rootNotifyHostname)
	rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
	if rootNotifyHostname != "" {
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStates[rootIdx]), &finalState); err == nil {
			raw := workflow.NewRawFlow()
			raw.SetRawState(finalState)
			foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, rootCompositeID, foremanapi.StatusFailed, raw.RawState())
		}
	}
	for i, cid := range chainCompositeIDs {
		svc.LogInfo(ctx, "Flow status transition", "flow", chainFlowIDs[i], "to", foremanapi.StatusFailed)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, cid, foremanapi.StatusFailed)
	}

	svc.IncrementStepsExecuted(ctx, 1, taskName, foremanapi.StatusFailed)
	return nil
}
