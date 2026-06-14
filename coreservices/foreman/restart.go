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

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"
)

// undoCohortBumps applies (arrivalsDelta, failuresDelta) as a SUBTRACT at the given spawn step,
// then walks up the lineage chain undoing any failure-propagation that resulted from this spawn
// having previously been fully resolved with failures. At each ancestor it reads the spawn's prior
// state; if (arrivals == size && failures > 0) BEFORE this decrement, the cohort had propagated up
// — so the ancestor needs the same (-1, -1) we'd have applied via propagateCohortFailure, in
// reverse. The walk stops at the first level whose prior state was NOT propagated, or at the root.
func (svc *Service) undoCohortBumps(ctx context.Context, tx sequel.Executor, spawnID int, arrivalsDelta int, failuresDelta int) error {
	if spawnID == 0 || (arrivalsDelta == 0 && failuresDelta == 0) {
		return nil
	}
	var priorArrivals, priorFailures, size, lineageID int
	err := tx.QueryRowContext(ctx,
		"SELECT cohort_arrivals, cohort_size, cohort_failures, lineage_id FROM microbus_steps WHERE step_id=?",
		spawnID,
	).Scan(&priorArrivals, &size, &priorFailures, &lineageID)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET cohort_arrivals = cohort_arrivals - ?, cohort_failures = cohort_failures - ? WHERE step_id=?",
		arrivalsDelta, failuresDelta, spawnID,
	)
	if err != nil {
		return errors.Trace(err)
	}
	for priorArrivals >= size && priorFailures > 0 && lineageID != 0 {
		parent := lineageID
		err := tx.QueryRowContext(ctx,
			"SELECT cohort_arrivals, cohort_size, cohort_failures, lineage_id FROM microbus_steps WHERE step_id=?",
			parent,
		).Scan(&priorArrivals, &size, &priorFailures, &lineageID)
		if err != nil {
			return errors.Trace(err)
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET cohort_arrivals = cohort_arrivals - 1, cohort_failures = cohort_failures - 1 WHERE step_id=?",
			parent,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// isRestartable reports whether a flow status allows whole-flow Restart.
func isRestartable(status string) bool {
	switch status {
	case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled, workflow.StatusInterrupted:
		return true
	}
	return false
}

// isRestartableStep reports whether a step status allows RestartFrom on it.
func isRestartableStep(status string) bool {
	switch status {
	case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled, workflow.StatusInterrupted:
		return true
	}
	return false
}

// mergeWithOverrides applies a shallow top-level merge of stateOverrides onto an existing JSON
// state. Keys present in overrides replace the original; keys mapped to JSON null are deleted.
func mergeWithOverrides(originalJSON string, overrides any) (string, error) {
	var state map[string]any
	if originalJSON != "" && originalJSON != "{}" {
		err := json.Unmarshal([]byte(originalJSON), &state)
		if err != nil {
			return "", errors.Trace(err)
		}
	}
	if state == nil {
		state = map[string]any{}
	}
	if overrides == nil {
		out, _ := json.Marshal(state)
		return string(out), nil
	}
	overridesJSON, err := json.Marshal(overrides)
	if err != nil {
		return "", errors.Trace(err)
	}
	var ov map[string]any
	err = json.Unmarshal(overridesJSON, &ov)
	if err != nil {
		return "", errors.Trace(err)
	}
	for k, v := range ov {
		if v == nil {
			delete(state, k)
		} else {
			state[k] = v
		}
	}
	out, _ := json.Marshal(state)
	return string(out), nil
}

// allDescendantSubgraphFlows returns every descendant subgraph flow_id reachable from this flow
// via surgraph_flow_id, regardless of status.
func (svc *Service) allDescendantSubgraphFlows(ctx context.Context, db sequel.Executor, flowID int) ([]int, error) {
	var collected []int
	current := []any{flowID}
	for len(current) > 0 {
		ph := strings.Repeat("?,", len(current)-1) + "?"
		rows, err := db.QueryContext(ctx,
			"SELECT flow_id FROM microbus_flows WHERE surgraph_flow_id IN ("+ph+")",
			current...,
		)
		if err != nil {
			return nil, errors.Trace(err)
		}
		current = nil
		for rows.Next() {
			var id int
			err = rows.Scan(&id)
			if err != nil {
				rows.Close()
				return nil, errors.Trace(err)
			}
			collected = append(collected, id)
			current = append(current, id)
		}
		rows.Close()
	}
	return collected, nil
}

// deleteSubgraphFlowsRootedAt removes any subgraph flows (and their steps and any nested
// descendants) launched from the given surgraph step.
func (svc *Service) deleteSubgraphFlowsRootedAt(ctx context.Context, tx sequel.Executor, surgraphStepID int) error {
	var rootChildren []int
	rows, err := tx.QueryContext(ctx,
		"SELECT flow_id FROM microbus_flows WHERE surgraph_step_id=?",
		surgraphStepID,
	)
	if err != nil {
		return errors.Trace(err)
	}
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			rows.Close()
			return errors.Trace(err)
		}
		rootChildren = append(rootChildren, id)
	}
	rows.Close()
	if len(rootChildren) == 0 {
		return nil
	}
	allIDs := make([]int, 0, len(rootChildren))
	allIDs = append(allIDs, rootChildren...)
	current := make([]any, 0, len(rootChildren))
	for _, id := range rootChildren {
		current = append(current, id)
	}
	for len(current) > 0 {
		ph := strings.Repeat("?,", len(current)-1) + "?"
		nestedRows, err := tx.QueryContext(ctx,
			"SELECT flow_id FROM microbus_flows WHERE surgraph_flow_id IN ("+ph+")",
			current...,
		)
		if err != nil {
			return errors.Trace(err)
		}
		current = nil
		for nestedRows.Next() {
			var id int
			err = nestedRows.Scan(&id)
			if err != nil {
				nestedRows.Close()
				return errors.Trace(err)
			}
			allIDs = append(allIDs, id)
			current = append(current, id)
		}
		nestedRows.Close()
	}
	args := make([]any, 0, len(allIDs))
	for _, id := range allIDs {
		args = append(args, id)
	}
	ph := strings.Repeat("?,", len(allIDs)-1) + "?"
	_, err = tx.ExecContext(ctx,
		"DELETE FROM microbus_steps WHERE flow_id IN ("+ph+")",
		args...,
	)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = tx.ExecContext(ctx,
		"DELETE FROM microbus_flows WHERE flow_id IN ("+ph+")",
		args...,
	)
	return errors.Trace(err)
}

// sweptMember describes one step swept by a RestartFrom subtree walk.
type sweptMember struct {
	stepID    int
	lineageID int
	status    string
}

// collectDAGSubtree walks forward from a starting step via successor_id and returns every
// reachable step (excluding the starting step itself). Each entry carries enough metadata to
// drive the cohort-counter decrement and cascade-delete passes.
func (svc *Service) collectDAGSubtree(ctx context.Context, db sequel.Executor, flowID, startStepID int) ([]sweptMember, error) {
	visited := map[int]bool{startStepID: true}
	var collected []sweptMember
	frontier := []any{startStepID}
	for len(frontier) > 0 {
		ph := strings.Repeat("?,", len(frontier)-1) + "?"
		args := append([]any{flowID}, frontier...)
		query := "SELECT step_id, lineage_id, status FROM microbus_steps WHERE flow_id=? AND (" +
			"step_id IN (SELECT successor_id FROM microbus_steps WHERE step_id IN (" + ph + ") AND successor_id<>0)" +
			" OR predecessor_id IN (" + ph + "))"
		args = append(args, frontier...)
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, errors.Trace(err)
		}
		var nextFrontier []any
		for rows.Next() {
			var sid, lid int
			var status string
			err = rows.Scan(&sid, &lid, &status)
			if err != nil {
				rows.Close()
				return nil, errors.Trace(err)
			}
			if visited[sid] {
				continue
			}
			visited[sid] = true
			collected = append(collected, sweptMember{stepID: sid, lineageID: lid, status: strings.TrimSpace(status)})
			nextFrontier = append(nextFrontier, sid)
		}
		rows.Close()
		frontier = nextFrontier
	}
	return collected, nil
}
