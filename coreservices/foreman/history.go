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
	"fmt"
	"strings"

	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)


// historyBeforeStep returns steps for a flow, up to (but not including) beforeStepDepth.
// If beforeStepDepth is 0, all steps are returned.
func (svc *Service) historyBeforeStep(ctx context.Context, shardNum int, flowID int, beforeStepDepth int) ([]foremanapi.FlowStep, error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var rows *sql.Rows
	if beforeStepDepth > 0 {
		rows, err = db.QueryContext(ctx,
			"SELECT step_id, step_token, step_depth, task_name, attempt, status, error, created_at, started_at, updated_at, predecessor_id, successor_id FROM microbus_steps WHERE flow_id=? AND step_depth<? ORDER BY step_depth, step_id",
			flowID, beforeStepDepth,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			"SELECT step_id, step_token, step_depth, task_name, attempt, status, error, created_at, started_at, updated_at, predecessor_id, successor_id FROM microbus_steps WHERE flow_id=? ORDER BY step_depth, step_id",
			flowID,
		)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	return svc.scanHistorySteps(ctx, shardNum, rows)
}

// scanHistorySteps reads FlowStep records from a sql.Rows cursor.
// For subgraph steps, it recursively fetches the subgraph flow's history.
func (svc *Service) scanHistorySteps(ctx context.Context, shardNum int, rows *sql.Rows) ([]foremanapi.FlowStep, error) {
	var steps []foremanapi.FlowStep
	for rows.Next() {
		var step foremanapi.FlowStep
		var stepID int
		var stepToken string
		var errMsg string
		err := rows.Scan(&stepID, &stepToken, &step.StepDepth, &step.TaskName, &step.Attempt, &step.Status, &errMsg, &step.CreatedAt, &step.StartedAt, &step.UpdatedAt, &step.PredecessorID, &step.SuccessorID)
		if err != nil {
			return nil, errors.Trace(err)
		}
		step.StepID = stepID
		step.StepKey = fmt.Sprintf("%d-%d-%s", shardNum, stepID, strings.TrimSpace(stepToken))
		step.Status = strings.TrimSpace(step.Status)
		step.Error = strings.TrimSpace(errMsg)
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Trace(err)
	}

	// A step is a subgraph step when a child flow.Subgraph flow exists for it. There is no static
	// subgraph node type anymore, so detection is by child-flow existence: subgraphHistory returns
	// the child's steps, or nil when none exists. Fetch nested histories after closing the cursor.
	for i := range steps {
		subWorkflowName, subHistory, err := svc.subgraphHistory(ctx, shardNum, steps[i].StepID)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(subHistory) > 0 {
			steps[i].Subgraph = true
			steps[i].SubWorkflowName = subWorkflowName
			steps[i].SubHistory = subHistory
		}
	}
	return steps, nil
}

// subgraphHistory returns the workflow URL and execution history of the subgraph flow
// attached to the given parked surgraph step. Looking up by surgraph_step_id (the parent
// step's PK) is the only unambiguous identifier when a fan-out spawns multiple parallel
// subgraphs at the same step_depth - see "Surgraph Step Identification" in CLAUDE.md.
func (svc *Service) subgraphHistory(ctx context.Context, shardNum int, surgraphStepID int) (subWorkflowName string, steps []foremanapi.FlowStep, err error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	var subFlowID int
	err = db.QueryRowContext(ctx,
		"SELECT flow_id, workflow_name FROM microbus_flows WHERE surgraph_step_id=?",
		surgraphStepID,
	).Scan(&subFlowID, &subWorkflowName)
	if err == sql.ErrNoRows {
		return "", nil, nil // Subgraph flow not yet created or already purged
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	subWorkflowName = strings.TrimSpace(subWorkflowName)
	steps, err = svc.historyBeforeStep(ctx, shardNum, subFlowID, 0)
	return subWorkflowName, steps, errors.Trace(err)
}
