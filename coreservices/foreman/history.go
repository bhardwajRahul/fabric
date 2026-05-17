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
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// renderMermaidSteps writes Mermaid flowchart nodes and edges for a list of steps.
// It returns the head node IDs (no incoming edge) and tail node IDs (no outgoing edge)
// so the caller can wire them to the surrounding start/end markers.
//
// The execution DAG is reconstructed purely from the recorded edges - each step's
// PredecessorID and SuccessorID - and step_depth is deliberately ignored. Every edge
// is recorded on at least one endpoint: fan-out edges via each child's PredecessorID,
// fan-in edges via each cohort exit step's SuccessorID, linear edges on both; the
// rendered edge set is their deduped union, which is exact for arbitrary nesting.
// Subgraph steps are expanded inline: edges into/out of the subgraph step attach to
// the subgraph's enter/exit markers. The startDate is the timestamp of the flow's
// first step, used to compute relative deltas.
func renderMermaidSteps(buf *strings.Builder, prefix string, steps []foremanapi.FlowStep, startDate time.Time) (heads []string, tails []string) {
	if len(steps) == 0 {
		return nil, nil
	}

	type renderNode struct {
		entry string // node ID an incoming edge should point at
		exit  string // node ID an outgoing edge should originate from
	}
	byID := make(map[int]*renderNode, len(steps))
	stepByID := make(map[int]foremanapi.FlowStep, len(steps))
	order := make([]int, 0, len(steps))

	// Render each step's node (or inline-expanded subgraph) and record its entry/exit.
	for i := range steps {
		s := steps[i]
		if s.StepID == 0 {
			continue
		}
		if s.Subgraph && len(s.SubHistory) > 0 {
			subPrefix := fmt.Sprintf("%ss%d_sub", prefix, s.StepID)
			subStartID := subPrefix + "_enter"
			subEndID := subPrefix + "_exit"
			fmt.Fprintf(buf, "    %s((\" \")):::term\n", subStartID)
			fmt.Fprintf(buf, "    %s((\" \")):::term\n", subEndID)
			subHeads, subTails := renderMermaidSteps(buf, subPrefix, s.SubHistory, startDate)
			for _, h := range subHeads {
				fmt.Fprintf(buf, "    %s --> %s\n", subStartID, h)
			}
			for _, t := range subTails {
				fmt.Fprintf(buf, "    %s --> %s\n", t, subEndID)
			}
			byID[s.StepID] = &renderNode{entry: subStartID, exit: subEndID}
		} else {
			nodeID := fmt.Sprintf("%ss%d", prefix, s.StepID)
			label := stripProto(s.TaskName)
			if !s.UpdatedAt.IsZero() && !startDate.IsZero() {
				label += "\n" + formatDeltaDuration(s.UpdatedAt.Sub(startDate))
			}
			statusClass := s.Status
			if statusClass == "" {
				statusClass = "pending"
			}
			fmt.Fprintf(buf, "    %s[\"%s\"]:::%s\n", nodeID, label, statusClass)
			byID[s.StepID] = &renderNode{entry: nodeID, exit: nodeID}
		}
		stepByID[s.StepID] = s
		order = append(order, s.StepID)
	}

	// Edges = deduped union of {predecessor -> step} and {step -> successor}.
	emitted := map[string]bool{}
	hasIncoming := map[int]bool{}
	hasOutgoing := map[int]bool{}
	addEdge := func(fromID, toID int) {
		from := byID[fromID]
		to := byID[toID]
		if from == nil || to == nil || from == to {
			return
		}
		key := from.exit + "\x00" + to.entry
		if emitted[key] {
			return
		}
		emitted[key] = true
		fmt.Fprintf(buf, "    %s --> %s\n", from.exit, to.entry)
		hasOutgoing[fromID] = true
		hasIncoming[toID] = true
	}
	for _, id := range order {
		s := stepByID[id]
		if s.PredecessorID != 0 {
			addEdge(s.PredecessorID, id)
		}
		if s.SuccessorID != 0 {
			addEdge(id, s.SuccessorID)
		}
	}

	// Heads have no incoming edge; tails have no outgoing edge.
	for _, id := range order {
		if !hasIncoming[id] {
			heads = append(heads, byID[id].entry)
		}
		if !hasOutgoing[id] {
			tails = append(tails, byID[id].exit)
		}
	}
	return heads, tails
}

// formatDeltaDuration formats a duration as a human-readable relative offset like "+0.211s" or "+2m30s".
func formatDeltaDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Second:
		return fmt.Sprintf("+%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("+%.3gs", d.Seconds())
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("+%dm", m)
		}
		return fmt.Sprintf("+%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("+%dh", h)
		}
		return fmt.Sprintf("+%dh%dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("+%dd", days)
		}
		return fmt.Sprintf("+%dd%dh", days, h)
	}
}

// historyBeforeStep returns steps for a flow, up to (but not including) beforeStepDepth.
// If beforeStepDepth is 0, all steps are returned. For forked flows, it recurses up the
// fork chain to reconstruct the full lineage.
func (svc *Service) historyBeforeStep(ctx context.Context, shardNum int, flowID int, beforeStepDepth int) ([]foremanapi.FlowStep, error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Load the flow's graph and fork lineage
	var forkedFlowID int
	var forkedStepDepth int
	var graphJSON string
	err = db.QueryRowContext(ctx,
		"SELECT forked_flow_id, forked_step_depth, graph FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&forkedFlowID, &forkedStepDepth, &graphJSON)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var graph workflow.Graph
	err = json.Unmarshal([]byte(graphJSON), &graph)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Recurse to fork ancestor first (if any) to prepend earlier steps
	var steps []foremanapi.FlowStep
	if forkedFlowID != 0 {
		steps, err = svc.historyBeforeStep(ctx, shardNum, forkedFlowID, forkedStepDepth)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	// Query this flow's steps
	var rows *sql.Rows
	if beforeStepDepth > 0 {
		rows, err = db.QueryContext(ctx,
			"SELECT step_id, step_token, step_depth, task_name, state, changes, status, error, updated_at, predecessor_id, successor_id FROM microbus_steps WHERE flow_id=? AND step_depth<? ORDER BY step_depth, step_id",
			flowID, beforeStepDepth,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			"SELECT step_id, step_token, step_depth, task_name, state, changes, status, error, updated_at, predecessor_id, successor_id FROM microbus_steps WHERE flow_id=? ORDER BY step_depth, step_id",
			flowID,
		)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	ownSteps, err := svc.scanHistorySteps(ctx, shardNum, rows, &graph, flowID)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return append(steps, ownSteps...), nil
}

// scanHistorySteps reads FlowStep records from a sql.Rows cursor.
// For subgraph steps, it recursively fetches the subgraph flow's history.
func (svc *Service) scanHistorySteps(ctx context.Context, shardNum int, rows *sql.Rows, graph *workflow.Graph, flowID int) ([]foremanapi.FlowStep, error) {
	var steps []foremanapi.FlowStep
	for rows.Next() {
		var step foremanapi.FlowStep
		var stepID int
		var stepToken string
		var stateJSON, changesJSON, errMsg string
		err := rows.Scan(&stepID, &stepToken, &step.StepDepth, &step.TaskName, &stateJSON, &changesJSON, &step.Status, &errMsg, &step.UpdatedAt, &step.PredecessorID, &step.SuccessorID)
		if err != nil {
			return nil, errors.Trace(err)
		}
		step.StepID = stepID
		step.StepKey = fmt.Sprintf("%d-%d-%s", shardNum, stepID, strings.TrimSpace(stepToken))
		step.Status = strings.TrimSpace(step.Status)
		step.Subgraph = graph.IsSubgraph(step.TaskName)
		step.Error = strings.TrimSpace(errMsg)

		// Deserialize state
		err = json.Unmarshal([]byte(stateJSON), &step.State)
		if err != nil {
			return nil, errors.Trace(err)
		}

		// Deserialize changes
		err = json.Unmarshal([]byte(changesJSON), &step.Changes)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(step.Changes) == 0 {
			step.Changes = nil
		}

		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Trace(err)
	}

	// Fetch nested subgraph histories after closing the rows cursor
	for i := range steps {
		if steps[i].Subgraph {
			subHistory, err := svc.subgraphHistory(ctx, shardNum, flowID, steps[i].StepDepth)
			if err != nil {
				return nil, errors.Trace(err)
			}
			steps[i].SubHistory = subHistory
		}
	}
	return steps, nil
}

// subgraphHistory returns the execution history of a subgraph flow spawned by the given
// parent flow at the given step number.
func (svc *Service) subgraphHistory(ctx context.Context, shardNum int, surgraphFlowID int, surgraphStepDepth int) ([]foremanapi.FlowStep, error) {
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var subFlowID int
	err = db.QueryRowContext(ctx,
		"SELECT flow_id FROM microbus_flows WHERE surgraph_flow_id=? AND surgraph_step_depth=?",
		surgraphFlowID, surgraphStepDepth,
	).Scan(&subFlowID)
	if err == sql.ErrNoRows {
		return nil, nil // Subgraph flow not yet created or already purged
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	return svc.historyBeforeStep(ctx, shardNum, subFlowID, 0)
}
