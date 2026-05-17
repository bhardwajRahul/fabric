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
	"strings"
	"time"

	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// fireFanInDirect creates the FanIn step immediately for an empty-cohort case.
// Used when a fan-out source produces zero branches (e.g. forEach over an empty array).
func (svc *Service) fireFanInDirect(ctx context.Context, shardNum int, db *sequel.DB, flowID int, flowToken string, stepID int, stepDepth int, lineageID int, fanInTarget string, graph *workflow.Graph, sleepDur time.Duration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET updated_at=NOW_UTC() WHERE flow_id=?",
		flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Set our cohort_size to 0 explicitly (it's already the default but be explicit).
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET cohort_size=0 WHERE step_id=?",
		stepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Compute base state from our state+changes (no cohort children to merge).
	var ourStateJSON, ourChangesJSON string
	err = tx.QueryRowContext(ctx,
		"SELECT state, changes FROM microbus_steps WHERE step_id=?",
		stepID,
	).Scan(&ourStateJSON, &ourChangesJSON)
	if err != nil {
		return errors.Trace(err)
	}
	var ourState, ourChanges map[string]any
	json.Unmarshal([]byte(ourStateJSON), &ourState)
	json.Unmarshal([]byte(ourChangesJSON), &ourChanges)
	mergedState, err := workflow.MergeState(ourState, ourChanges, nil)
	if err != nil {
		return errors.Trace(err)
	}
	mergedJSON, _ := json.Marshal(mergedState)

	nextStepDepth := stepDepth + 1
	sleepMs := sleepDur.Milliseconds()
	nextTimeBudget := svc.taskTimeBudget()
	fanInStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lineage_id, predecessor_id, not_before, priority, fairness_key, fairness_weight)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), (SELECT priority FROM microbus_flows WHERE flow_id=?), (SELECT fairness_key FROM microbus_flows WHERE flow_id=?), (SELECT fairness_weight FROM microbus_flows WHERE flow_id=?))",
		flowID, nextStepDepth, utils.RandomIdentifier(16), fanInTarget, string(mergedJSON), foremanapi.StatusPending, nextTimeBudget.Milliseconds(), lineageID, stepID, sleepMs, flowID, flowID, flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Empty cohort: the single edge is spawn -> fan-in. Record the forward side too.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET successor_id=? WHERE step_id=?",
		int(fanInStepID), stepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		int(fanInStepID), flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}
	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	if sleepDur > 0 {
		svc.shortenNextPoll(time.Now().Add(sleepDur))
	} else {
		foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, int(fanInStepID))
	}
	return nil
}

// insertFanInStep creates the fan-in step inside the caller's tx after the cohort completes.
// Only cohort members (steps with lineage_id = cohortSpawnID) in status 'completed' contribute
// their changes, merged in fan_out_ordinal order (the forEach array / spawn order) so list/append/
// sum reducers are deterministic; step_id breaks ties. Non-completed members ('failed',
// 'cancelled', 'retried', 'pending', 'running') contribute no state and the fan-in does NOT
// escalate on them: a 'cancelled' member is the normal OnError sibling-cancel case and the flow
// must still recover via the handler path; genuine failures/cancels are already terminal via
// failStep / the Cancel API / the terminal-flow check. The new step's lineage_id is the spawn's
// lineage_id (one frame popped). Its predecessor_id is predecessorStepID (the last cohort member
// to finish); each cohort exit step's successor_id is set to the fan-in step so the many-to-one
// fan-in edges are recorded.
func (svc *Service) insertFanInStep(ctx context.Context, tx sequel.Executor, flowID, nextStepDepth, cohortSpawnID, predecessorStepID int, fanInTaskName string, graph *workflow.Graph, sleepMs int64) (stepID int, err error) {
	var spawnStateJSON, spawnChangesJSON string
	var spawnLineageID int
	err = tx.QueryRowContext(ctx,
		"SELECT state, changes, lineage_id FROM microbus_steps WHERE step_id=?",
		cohortSpawnID,
	).Scan(&spawnStateJSON, &spawnChangesJSON, &spawnLineageID)
	if err != nil {
		return 0, errors.Trace(err)
	}
	var spawnState, spawnChanges map[string]any
	json.Unmarshal([]byte(spawnStateJSON), &spawnState)
	json.Unmarshal([]byte(spawnChangesJSON), &spawnChanges)
	merged, err := workflow.MergeState(spawnState, spawnChanges, graph.Reducers())
	if err != nil {
		return 0, errors.Trace(err)
	}

	rows, err := tx.QueryContext(ctx,
		"SELECT status, changes FROM microbus_steps WHERE flow_id=? AND lineage_id=? ORDER BY fan_out_ordinal, step_id",
		flowID, cohortSpawnID,
	)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer rows.Close()
	for rows.Next() {
		var status, changesJSON string
		err = rows.Scan(&status, &changesJSON)
		if err != nil {
			return 0, errors.Trace(err)
		}
		// Only completed members contribute state. failed/cancelled/retried/pending/
		// running are skipped - the fan-in does not escalate on them.
		if status != foremanapi.StatusCompleted {
			continue
		}
		var changes map[string]any
		json.Unmarshal([]byte(changesJSON), &changes)
		merged, err = workflow.MergeState(merged, changes, graph.Reducers())
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	err = rows.Err()
	if err != nil {
		return 0, errors.Trace(err)
	}

	mergedJSON, _ := json.Marshal(merged)
	nextTimeBudget := svc.taskTimeBudget()
	fanInStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lineage_id, predecessor_id, not_before, priority, fairness_key, fairness_weight)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), (SELECT priority FROM microbus_flows WHERE flow_id=?), (SELECT fairness_key FROM microbus_flows WHERE flow_id=?), (SELECT fairness_weight FROM microbus_flows WHERE flow_id=?))",
		flowID, nextStepDepth, utils.RandomIdentifier(16), fanInTaskName, string(mergedJSON), foremanapi.StatusPending, nextTimeBudget.Milliseconds(), spawnLineageID, predecessorStepID, sleepMs, flowID, flowID, flowID,
	)
	if err != nil {
		return 0, errors.Trace(err)
	}

	// Record the fan-in edges on the cohort exit steps: those whose task transitions
	// into the fan-in (per the graph), scoped to this cohort by lineage_id. This is
	// the exit set, not the whole lineage (e.g. in S->forEach->{A->B->C}->J only the
	// C's, not A/B, point at J).
	exitTasks := fanInPredecessorTasks(graph, fanInTaskName)
	if len(exitTasks) > 0 {
		placeholders := strings.Repeat("?,", len(exitTasks)-1) + "?"
		args := []any{int(fanInStepID), flowID, cohortSpawnID}
		for _, t := range exitTasks {
			args = append(args, t)
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET successor_id=? WHERE flow_id=? AND lineage_id=? AND task_name IN ("+placeholders+")",
			args...,
		)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	return int(fanInStepID), nil
}

// fanInPredecessorTasks returns the distinct task node names that transition into
// fanInTask via a normal (non-goto, non-error) transition. These are the cohort's
// exit tasks; their step instances are the direct predecessors of the fan-in step.
func fanInPredecessorTasks(graph *workflow.Graph, fanInTask string) []string {
	seen := map[string]bool{}
	var names []string
	for _, tr := range graph.Transitions() {
		if tr.To == fanInTask && !tr.WithGoto && !tr.OnError && !seen[tr.From] {
			seen[tr.From] = true
			names = append(names, tr.From)
		}
	}
	return names
}

// nextStep describes a step to be created during transition evaluation.
type nextStep struct {
	taskName string
	item     any    // non-nil for forEach fan-out
	itemKey  string // state key for the item (the "as" alias or forEach field name)
}

// evaluateTransitions determines the next task(s) to execute based on the graph transitions
// and the current flow state. Returns multiple candidates for fan-out.
func (svc *Service) evaluateTransitions(graph *workflow.Graph, currentTask string, flow *workflow.RawFlow) ([]nextStep, error) {
	// Check for goto override. The user passes the target as a URL or node name to flow.Goto;
	// match a withGoto transition whose To either equals the target literally (node name match
	// or URL-as-name back-compat) or whose target node has that URL.
	if gotoTarget := flow.GotoRequested(); gotoTarget != "" {
		for _, tr := range graph.Transitions() {
			if tr.From != currentTask || !tr.WithGoto {
				continue
			}
			if tr.To == gotoTarget || graph.URLOf(tr.To) == gotoTarget {
				return []nextStep{{taskName: tr.To}}, nil
			}
		}
		return nil, errors.New("task '%s' requested goto to '%s' but no WithGoto transition exists from this task", stripProto(currentTask), stripProto(gotoTarget))
	}

	// Build state map for condition evaluation.
	// RawState() includes the task's output because the intermediate layer's handler calls
	// flow.SetChanges(), which writes changed fields to both the state and changes maps.
	// This means When conditions on outgoing transitions see the task's output values.
	// Values may be json.RawMessage (from internal storage) - unmarshal them for boolexp evaluation.
	stateMap := make(map[string]any, len(flow.RawState()))
	for k, v := range flow.RawState() {
		if raw, ok := v.(json.RawMessage); ok {
			var val any
			json.Unmarshal(raw, &val)
			stateMap[k] = val
		} else {
			stateMap[k] = v
		}
	}

	// Evaluate transitions from the current task
	var candidates []nextStep
	for _, tr := range graph.Transitions() {
		if tr.From != currentTask {
			continue
		}
		if tr.WithGoto {
			continue // Goto transitions are only followed when explicitly requested
		}
		if tr.OnError {
			continue // Error transitions are only followed when the task returns an error
		}
		taken := false
		if tr.When == "" {
			taken = true
		} else {
			match, err := boolexp.Eval(tr.When, stateMap)
			if err != nil {
				return nil, errors.Trace(err)
			}
			taken = match
		}
		if !taken {
			continue
		}

		if tr.ForEach != "" {
			// Dynamic fan-out: expand one step per element in the state array
			val, ok := flow.RawState()[tr.ForEach]
			if !ok {
				continue // field not in state, skip this transition
			}
			raw, err := json.Marshal(val)
			if err != nil {
				return nil, errors.Trace(err)
			}
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, errors.New("forEach field '%s' is not an array", tr.ForEach, err)
			}
			itemKey := tr.As
			if itemKey == "" {
				itemKey = "item"
			}
			for _, item := range items {
				candidates = append(candidates, nextStep{
					taskName: tr.To,
					item:     item,
					itemKey:  itemKey,
				})
			}
		} else {
			candidates = append(candidates, nextStep{taskName: tr.To})
		}
	}

	return candidates, nil
}

// evaluateErrorTransitions determines the error handler task to route to when a task fails.
// It evaluates only OnError transitions from the current task, respecting When conditions.
// Status-coded transitions (StatusCode != 0) win on a match against errStatusCode; otherwise
// the first catch-all OnError transition wins. Returns at most one candidate.
func (svc *Service) evaluateErrorTransitions(graph *workflow.Graph, currentTask string, flow *workflow.RawFlow, errStatusCode int) ([]nextStep, error) {
	// Build state map for condition evaluation
	stateMap := make(map[string]any, len(flow.RawState()))
	for k, v := range flow.RawState() {
		if raw, ok := v.(json.RawMessage); ok {
			var val any
			json.Unmarshal(raw, &val)
			stateMap[k] = val
		} else {
			stateMap[k] = v
		}
	}

	// Two passes: status-coded matches first, catch-all second.
	for _, statusCoded := range []bool{true, false} {
		for _, tr := range graph.Transitions() {
			if tr.From != currentTask || !tr.OnError {
				continue
			}
			if statusCoded {
				if tr.StatusCode == 0 || tr.StatusCode != errStatusCode {
					continue
				}
			} else {
				if tr.StatusCode != 0 {
					continue
				}
			}
			taken := true
			if tr.When != "" {
				match, err := boolexp.Eval(tr.When, stateMap)
				if err != nil {
					return nil, errors.Trace(err)
				}
				taken = match
			}
			if taken {
				return []nextStep{{taskName: tr.To}}, nil
			}
		}
	}
	return nil, nil
}
