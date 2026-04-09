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
	"io/fs"
	"maps"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

var (
	_ foremanapi.Client
)

const (
	sequenceName = "foreman@2026-03-10" // Do not change

	maxPollInterval = 5 * time.Minute
	leaseMargin     = 30 * time.Second // margin on top of time_budget for lease duration
)

/*
Service implements the foreman.core microservice.

The foreman orchestrates agentic workflow execution. It fetches workflow graph definitions, executes tasks sequentially,
evaluates transition conditions, and persists state after each step.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	dbs     []*sequel.DB
	dbsLock sync.RWMutex

	// FIFO work queue and worker pool
	queue   jobQueue
	workers sync.WaitGroup

	// Timer goroutine for polling delayed steps
	nextPoll     time.Time
	nextPollLock sync.Mutex
	wakeTimer    chan struct{}

	// Wait registry for Await (keyed by flowKey)
	waitersLock sync.Mutex
	waiters     map[string][]chan string // flowKey -> list of waiting channels
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	err = svc.openDatabase(ctx)
	if err != nil {
		return errors.Trace(err)
	}

	// Initialize the queue and start workers.
	// Workers use raw goroutines (not svc.Go) so they don't count as pending operations during the drain phase of shutdown.
	// Their lifecycle is managed by svc.workers.Wait() in OnShutdown, which runs after queue.close() wakes them up.
	svc.queue.init()
	svc.wakeTimer = make(chan struct{}, 1)
	svc.nextPoll = time.Now().Add(5 * time.Minute)
	numWorkers := svc.Workers()
	for range numWorkers {
		svc.workers.Add(1)
		go func() {
			defer svc.workers.Done()
			svc.workerLoop(ctx)
		}()
	}
	// Timer goroutine for polling delayed steps
	svc.workers.Add(1)
	go func() {
		defer svc.workers.Done()
		svc.timerLoop(ctx)
	}()
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	svc.queue.close() // Terminate workerLoop
	if svc.wakeTimer != nil {
		close(svc.wakeTimer) // Terminate timerLoop
	}
	svc.workers.Wait()
	// Wake all Await callers so they don't hang
	svc.waitersLock.Lock()
	for _, chans := range svc.waiters {
		for _, ch := range chans {
			select {
			case ch <- "":
			default:
			}
		}
	}
	svc.waitersLock.Unlock()
	svc.closeDatabase(ctx)
	return nil
}

// OnObserveQueueDepth records the current depth of the local worker queue.
func (svc *Service) OnObserveQueueDepth(ctx context.Context) (err error) { // MARKER: QueueDepth
	err = svc.RecordQueueDepth(ctx, svc.queue.len())
	return errors.Trace(err)
}

/*
HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.
*/
func (svc *Service) HistoryMermaid(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HistoryMermaid
	flowKey := r.URL.Query().Get("flowKey")
	if flowKey == "" {
		return errors.New("flowKey is required", http.StatusBadRequest)
	}

	steps, err := svc.History(r.Context(), flowKey)
	if err != nil {
		return errors.Trace(err)
	}

	var buf strings.Builder
	buf.WriteString("flowchart TD\n")
	buf.WriteString("    classDef completed fill:#32a7c1,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef failed fill:#f15922,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef interrupted fill:#ed2e92,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef retried fill:#c1884a,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef pending fill:#c1cccc,color:#434343,stroke:#434343\n")
	buf.WriteString("    classDef running fill:#2e8a9e,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef cancelled fill:#7a8a8a,color:#f4f2ef,stroke:#434343\n")
	buf.WriteString("    classDef term fill:#e5f4f3,color:#434343,stroke:#434343\n")

	startID := "_start"
	endID := "_end"
	fmt.Fprintf(&buf, "    %s((\" \")):::term\n", startID)
	fmt.Fprintf(&buf, "    %s((\" \")):::term\n", endID)
	var startDate time.Time
	if len(steps) > 0 {
		startDate = steps[0].UpdatedAt.UTC()
	}
	heads, tails := renderMermaidSteps(&buf, "", steps, startDate)
	for _, h := range heads {
		fmt.Fprintf(&buf, "    %s --> %s\n", startID, h)
	}
	for _, t := range tails {
		fmt.Fprintf(&buf, "    %s --> %s\n", t, endID)
	}

	if r.URL.Query().Get("format") == "raw" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, buf.String())
		return nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Flow History - %s</title>
<script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
<style>
body { font-family: sans-serif; margin: 2em; background: #fafafa; }
.mermaid { background: #fff; padding: 1em; border-radius: 8px; border: 1px solid #ddd; }
</style>
</head>
<body>
<pre class="mermaid">
%s
</pre>
<script>mermaid.initialize({startOnLoad:true, securityLevel:'loose'});</script>
</body>
</html>`, flowKey, buf.String())
	return nil
}

// renderMermaidSteps writes Mermaid flowchart nodes and edges for a list of steps.
// It returns the head node IDs (entry points) and tail node IDs (exit points) so the
// caller can wire them to surrounding nodes. Subgraph steps are expanded inline: the
// subgraph's own heads and tails replace the parent node in the flow.
// The startDate is the timestamp of the flow's first step, used to compute relative deltas.
func renderMermaidSteps(buf *strings.Builder, prefix string, steps []foremanapi.FlowStep, startDate time.Time) (heads []string, tails []string) {
	if len(steps) == 0 {
		return nil, nil
	}

	// Group steps by stepDepth to handle fan-out
	type stepEntry struct {
		step  foremanapi.FlowStep
		heads []string // entry node IDs for this step
		tails []string // exit node IDs for this step
	}
	type stepGroup struct {
		stepDepth int
		entries   []stepEntry
	}
	var groups []stepGroup
	groupIdx := map[int]int{}
	for _, s := range steps {
		if idx, ok := groupIdx[s.StepDepth]; ok {
			groups[idx].entries = append(groups[idx].entries, stepEntry{step: s})
		} else {
			groupIdx[s.StepDepth] = len(groups)
			groups = append(groups, stepGroup{
				stepDepth: s.StepDepth,
				entries:   []stepEntry{{step: s}},
			})
		}
	}

	// Render nodes and determine heads/tails for each entry
	for gi := range groups {
		for ei := range groups[gi].entries {
			e := &groups[gi].entries[ei]
			s := e.step

			if s.Subgraph && len(s.SubHistory) > 0 {
				// Expand subgraph inline with start/end markers
				subPrefix := fmt.Sprintf("%ss%d_%d_sub", prefix, groups[gi].stepDepth, ei)
				subLabel := " "
				subStartID := subPrefix + "_enter"
				subEndID := subPrefix + "_exit"
				fmt.Fprintf(buf, "    %s((\"%s\")):::term\n", subStartID, subLabel)
				fmt.Fprintf(buf, "    %s((\"%s\")):::term\n", subEndID, subLabel)
				subHeads, subTails := renderMermaidSteps(buf, subPrefix, s.SubHistory, startDate)
				for _, h := range subHeads {
					fmt.Fprintf(buf, "    %s --> %s\n", subStartID, h)
				}
				for _, t := range subTails {
					fmt.Fprintf(buf, "    %s --> %s\n", t, subEndID)
				}
				e.heads = []string{subStartID}
				e.tails = []string{subEndID}
			} else {
				// Regular node
				nodeID := fmt.Sprintf("%ss%d_%d", prefix, groups[gi].stepDepth, ei)
				label := stripProto(s.TaskName)
				if !s.UpdatedAt.IsZero() && !startDate.IsZero() {
					label += "\n" + formatDeltaDuration(s.UpdatedAt.Sub(startDate))
				}
				statusClass := s.Status
				if statusClass == "" {
					statusClass = "pending"
				}
				fmt.Fprintf(buf, "    %s[\"%s\"]:::%s\n", nodeID, label, statusClass)
				e.heads = []string{nodeID}
				e.tails = []string{nodeID}
			}
		}
	}

	// Connect edges between consecutive groups
	for gi := 1; gi < len(groups); gi++ {
		prevGroup := groups[gi-1]
		curGroup := groups[gi]
		for _, prev := range prevGroup.entries {
			for _, cur := range curGroup.entries {
				for _, t := range prev.tails {
					for _, h := range cur.heads {
						fmt.Fprintf(buf, "    %s --> %s\n", t, h)
					}
				}
			}
		}
	}

	// Collect overall heads from first group and tails from last group
	for _, e := range groups[0].entries {
		heads = append(heads, e.heads...)
	}
	for _, e := range groups[len(groups)-1].entries {
		tails = append(tails, e.tails...)
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

// shard returns the database connection for the given shard index.
func (svc *Service) shard(n int) (*sequel.DB, error) {
	svc.dbsLock.RLock()
	defer svc.dbsLock.RUnlock()
	if n < 0 || n >= len(svc.dbs) {
		return nil, errors.New("flow not found", http.StatusNotFound)
	}
	return svc.dbs[n], nil
}

// numDBShards returns the current number of database shards.
func (svc *Service) numDBShards() int {
	svc.dbsLock.RLock()
	n := len(svc.dbs)
	svc.dbsLock.RUnlock()
	return n
}

// openDatabase opens connections to all database shards and runs migrations.
func (svc *Service) openDatabase(ctx context.Context) (err error) {
	numShards := svc.NumShards()
	dataSourceName := svc.SQLDataSourceName()
	if numShards > 1 && !strings.Contains(dataSourceName, "%d") {
		return errors.New("SQLDataSourceName must contain %%d when NumShards > 1")
	}
	for i := range numShards {
		db, err := svc.openDatabaseShard(ctx, dataSourceName, i)
		if err != nil {
			return errors.Trace(err)
		}
		svc.dbs = append(svc.dbs, db)
	}
	return nil
}

// closeDatabase closes all database shard connections.
func (svc *Service) closeDatabase(ctx context.Context) {
	_ = ctx
	for _, db := range svc.dbs {
		db.Close()
	}
	svc.dbs = nil
}

// openDatabaseShard opens a single database shard connection and runs migrations.
func (svc *Service) openDatabaseShard(ctx context.Context, dataSourceName string, shardIndex int) (db *sequel.DB, err error) {
	_ = ctx
	const driverName = "" // Inferred from the data source name
	dsn := dataSourceName
	if strings.Contains(dataSourceName, "%d") {
		dsn = fmt.Sprintf(dataSourceName, shardIndex)
	}
	if dsn == "" && svc.Deployment() == connector.LOCAL {
		dsn = "file:local.sqlite"
	}
	if svc.Deployment() == connector.TESTING {
		db, err = sequel.OpenTesting(driverName, dsn, fmt.Sprintf("%s_shard%d", svc.Plane(), shardIndex))
	} else {
		db, err = sequel.Open(driverName, dsn)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	dirFS, err := fs.Sub(svc.ResFS(), "sql")
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = db.Migrate(sequenceName, dirFS)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return db, nil
}

// OnChangedNumShards opens connections to any newly added shards.
func (svc *Service) OnChangedNumShards(ctx context.Context) (err error) {
	newCount := svc.NumShards()
	oldCount := svc.numDBShards()
	if newCount < oldCount {
		return errors.New("cannot reduce NumShards from %d to %d", oldCount, newCount)
	}
	if newCount == oldCount {
		return nil
	}
	dataSourceName := svc.SQLDataSourceName()
	if !strings.Contains(dataSourceName, "%d") {
		return errors.New("SQLDataSourceName must contain %%d when NumShards > 1")
	}
	for i := oldCount; i < newCount; i++ {
		db, err := svc.openDatabaseShard(ctx, dataSourceName, i)
		if err != nil {
			return errors.Trace(err)
		}
		svc.dbsLock.Lock()
		svc.dbs = append(svc.dbs, db)
		svc.dbsLock.Unlock()
	}
	svc.LogDebug(ctx, "Shards expanded", "from", oldCount, "to", newCount)
	return nil
}

/*
Create creates a new flow for a workflow without starting it.
*/
func (svc *Service) Create(ctx context.Context, workflowName string, initialState any) (flowKey string, err error) { // MARKER: Create
	if workflowName == "" {
		return "", errors.New("workflow name is required", http.StatusBadRequest)
	}
	graph, err := svc.fetchGraph(ctx, workflowName)
	if err != nil {
		return "", errors.Trace(err)
	}
	shardNum := rand.IntN(svc.numDBShards())
	flowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, graph, initialState, 0, "")
	return flowKey, errors.Trace(err)
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers.
The threadKey can be any flowKey that belongs to the thread (including the original one).
The new flow uses the same workflow graph, belongs to the same thread, and is returned in created status.
It is intended for multi-turn workflows where outputs feed back as inputs.
*/
func (svc *Service) Continue(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error) { // MARKER: Continue
	shardNum, flowID, flowToken, err := parseFlowKey(threadKey)
	if err != nil {
		return "", errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Look up the thread_id and thread_token from the provided flowKey
	var threadID int
	var threadToken string
	err = db.QueryRowContext(ctx,
		"SELECT thread_id, thread_token FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&threadID, &threadToken)
	if err != nil {
		return "", errors.New("flow not found", http.StatusNotFound)
	}
	threadToken = strings.TrimSpace(threadToken)

	// Find the latest flow in the thread
	var latestFlowID int
	var flowStatus, finalStateJSON, graphJSON, workflowName string
	err = db.QueryRowContext(ctx,
		"SELECT flow_id, status, final_state, graph, workflow_name FROM microbus_flows WHERE thread_id=? ORDER BY flow_id DESC LIMIT_OFFSET(1, 0)",
		threadID,
	).Scan(&latestFlowID, &flowStatus, &finalStateJSON, &graphJSON, &workflowName)
	if err != nil {
		return "", errors.New("no flows found in thread", http.StatusNotFound)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	if flowStatus != foremanapi.StatusCompleted {
		return "", errors.New("latest flow in thread is not completed (status: %s)", flowStatus, http.StatusConflict)
	}

	// Deserialize final state and graph
	var finalState map[string]any
	if err = json.Unmarshal([]byte(finalStateJSON), &finalState); err != nil {
		return "", errors.Trace(err)
	}
	var graph workflow.Graph
	if err = json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		return "", errors.Trace(err)
	}

	// Merge additional state on top of final state using the graph's reducers
	mergedState, err := workflow.MergeState(finalState, additionalState, graph.Reducers())
	if err != nil {
		return "", errors.Trace(err)
	}

	// Create a new flow with the same graph and merged state, in the same thread
	newFlowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, &graph, mergedState, threadID, threadToken)
	return newFlowKey, errors.Trace(err)
}

/*
CreateTask creates a flow that executes a single task and then terminates, without starting it.
*/
func (svc *Service) CreateTask(ctx context.Context, taskName string, initialState any) (flowKey string, err error) { // MARKER: CreateTask
	if taskName == "" {
		return "", errors.New("task name is required", http.StatusBadRequest)
	}
	graph := workflow.NewGraph(taskName)
	graph.DeclareInputs("*")
	graph.DeclareOutputs("*")
	graph.AddTransition(taskName, workflow.END)
	shardNum := rand.IntN(svc.numDBShards())
	flowKey, err = svc.createWithGraph(ctx, shardNum, taskName, graph, initialState, 0, "")
	return flowKey, errors.Trace(err)
}

// createWithGraph is the shared implementation for Create, CreateTask, and Continue.
// It creates a new flow from a pre-built graph in "created" status without starting it.
// If threadID is 0, the new flow starts its own thread (thread_id = flow_id).
// If threadID is non-zero, the new flow joins the specified thread.
func (svc *Service) createWithGraph(ctx context.Context, shardNum int, workflowName string, graph *workflow.Graph, initialState any, threadID int, threadToken string) (flowKey string, err error) {
	// Validate entry point
	entryPoint := graph.EntryPoint()
	if entryPoint == "" {
		return "", errors.New("workflow has no entry point", http.StatusBadRequest)
	}

	// Extract actor claims from the calling context
	var actorClaims map[string]any
	frame.Of(ctx).ParseActor(&actorClaims)
	actorClaimsJSON, err := json.Marshal(actorClaims)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Serialize graph and initial state
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return "", errors.Trace(err)
	}
	stateJSON, err := json.Marshal(initialState)
	if err != nil {
		return "", errors.Trace(err)
	}

	flowToken := utils.RandomIdentifier(16)

	// Create a root span for the flow and serialize its trace context
	flowCtx, flowSpan := svc.StartSpan(trace.ContextWithSpan(ctx, nil), "workflow",
		trc.Internal(),
		trc.String("workflow.name", workflowName),
	)
	flowSpan.End()
	traceParent := extractTraceParent(flowCtx)

	// Atomically create the flow and its first step within a transaction
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer tx.Rollback()

	newFlowID, err := tx.InsertReturnID(ctx, "flow_id",
		"INSERT INTO microbus_flows (flow_token, workflow_name, graph, actor_claims, status, forked_flow_id, forked_step_depth, trace_parent)"+
			" VALUES (?, ?, ?, ?, ?, 0, 0, ?)",
		flowToken, workflowName, string(graphJSON), string(actorClaimsJSON), foremanapi.StatusCreated, traceParent,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Set thread_id: use provided threadID if joining an existing thread, otherwise self-reference
	if threadID == 0 {
		threadID = int(newFlowID)
		threadToken = flowToken
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET thread_id=?, thread_token=? WHERE flow_id=?",
		threadID, threadToken, newFlowID,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	timeBudget := svc.taskTimeBudget(graph, entryPoint)
	newStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lease_expires)"+
			" VALUES (?, 1, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?))",
		newFlowID, utils.RandomIdentifier(16), entryPoint, string(stateJSON), foremanapi.StatusCreated, timeBudget.Milliseconds(), leaseMargin.Milliseconds(),
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		newStepID, newFlowID,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return "", errors.Trace(err)
	}
	svc.LogDebug(ctx, "Flow created", "flow", workflowName, "task", entryPoint)

	return fmt.Sprintf("%d-%d-%s", shardNum, newFlowID, flowToken), nil
}

/*
Start transitions a created flow to running and enqueues it for execution.
*/
func (svc *Service) Start(ctx context.Context, flowKey string) (err error) { // MARKER: Start
	err = svc.StartNotify(ctx, flowKey, "")
	return errors.Trace(err)
}

/*
StartNotify transitions a created flow to running with status change notifications sent to the given hostname.
The caller receives an OnFlowStopped event at the notifyHostname when the flow stops
(completed, failed, cancelled, or interrupted). If notifyHostname is empty, no notification is sent.
*/
func (svc *Service) StartNotify(ctx context.Context, flowKey string, notifyHostname string) (err error) { // MARKER: StartNotify
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// Validate that the flow is in created status
	var flowStatus string
	var stepID int
	var workflowName string
	err = db.QueryRowContext(ctx,
		"SELECT status, step_id, workflow_name FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus, &stepID, &workflowName)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	if flowStatus != foremanapi.StatusCreated {
		return errors.New("flow is not in created status (status: %s)", flowStatus, http.StatusConflict)
	}

	// Atomically transition steps and flow within a transaction (steps first, then flow)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE flow_id=? AND status=?",
		foremanapi.StatusPending, flowID, foremanapi.StatusCreated,
	)
	if err != nil {
		return errors.Trace(err)
	}

	notifyHostname = strings.TrimSpace(notifyHostname)
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, notify_hostname=?, updated_at=NOW_UTC() WHERE flow_id=? AND status=?",
		foremanapi.StatusRunning, notifyHostname, flowID, foremanapi.StatusCreated,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Another replica started the flow
		return errors.New("flow is already started", http.StatusConflict)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "from", foremanapi.StatusCreated, "to", foremanapi.StatusRunning)
	svc.IncrementFlowsStarted(ctx, 1, workflowName)
	compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, foremanapi.StatusRunning)

	// Enqueue the current step for processing (outside the transaction)
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, int(stepID))
	return nil
}

/*
Snapshot returns the current status and state of a flow.
*/
func (svc *Service) Snapshot(ctx context.Context, flowKey string) (status string, state map[string]any, err error) { // MARKER: Snapshot
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	// Query the flow
	retried := false
queryFlow:
	var flowStatus string
	var flowStepID int
	var workflowName string
	var finalStateJSON string
	var graphJSON string
	err = db.QueryRowContext(ctx,
		"SELECT status, step_id, workflow_name, final_state, graph FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus, &flowStepID, &workflowName, &finalStateJSON, &graphJSON)
	if err == sql.ErrNoRows {
		return "", nil, errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	fanOut := flowStepID == 0

	// For terminated flows, return the pre-computed final_state
	if flowStatus == foremanapi.StatusCompleted || flowStatus == foremanapi.StatusFailed || flowStatus == foremanapi.StatusCancelled {
		var finalState map[string]any
		if err = json.Unmarshal([]byte(finalStateJSON), &finalState); err != nil {
			return "", nil, errors.Trace(err)
		}
		return flowStatus, finalState, nil
	}

	// Query the current step
	var stateJSON, changesJSON, interruptPayloadJSON, taskName string
	var stepDepth int
	if fanOut {
		// Pick the most recently active step
		err = db.QueryRowContext(ctx,
			"SELECT state, changes, interrupt_payload, task_name, step_depth FROM microbus_steps WHERE flow_id=? AND status IN (?, ?, ?, ?) ORDER BY updated_at DESC LIMIT_OFFSET(1, 0)",
			flowID,
			foremanapi.StatusCreated, foremanapi.StatusPending, foremanapi.StatusRunning, foremanapi.StatusInterrupted,
		).Scan(&stateJSON, &changesJSON, &interruptPayloadJSON, &taskName, &stepDepth)
	} else {
		err = db.QueryRowContext(ctx,
			"SELECT state, changes, interrupt_payload, task_name, step_depth FROM microbus_steps WHERE step_id=? AND status IN (?, ?, ?, ?)",
			flowStepID,
			foremanapi.StatusCreated, foremanapi.StatusPending, foremanapi.StatusRunning, foremanapi.StatusInterrupted,
		).Scan(&stateJSON, &changesJSON, &interruptPayloadJSON, &taskName, &stepDepth)
	}
	if err == sql.ErrNoRows {
		// Race: the step terminated between the flow query and the step query.
		// Re-read the flow - it should now be terminal with final_state populated.
		if !retried {
			retried = true
			goto queryFlow
		}
		// The flow is still running but no active step exists momentarily (e.g. between fan-out steps).
		// Return the flow's current status so Await can continue waiting.
		return flowStatus, nil, nil
	}
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	// Merge state and changes
	var rawState map[string]any
	err = json.Unmarshal([]byte(stateJSON), &rawState)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	var rawChanges map[string]any
	err = json.Unmarshal([]byte(changesJSON), &rawChanges)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	rawMerged, err := workflow.MergeState(rawState, rawChanges, nil)
	if err != nil {
		return "", nil, errors.Trace(err)
	}

	// Merge interrupt payload into the returned state using the flow's reducers
	if interruptPayloadJSON != "" && interruptPayloadJSON != "{}" {
		var payload map[string]any
		if err = json.Unmarshal([]byte(interruptPayloadJSON), &payload); err != nil {
			return "", nil, errors.Trace(err)
		}
		if len(payload) > 0 {
			var graph workflow.Graph
			if err = json.Unmarshal([]byte(graphJSON), &graph); err != nil {
				return "", nil, errors.Trace(err)
			}
			rawMerged, err = workflow.MergeState(rawMerged, payload, graph.Reducers())
			if err != nil {
				return "", nil, errors.Trace(err)
			}
		}
	}

	return flowStatus, rawMerged, nil
}

/*
Resume resumes an interrupted flow by merging resumeData into the leaf step's state and re-enqueuing it for execution.
*/
func (svc *Service) Resume(ctx context.Context, flowKey string, resumeData any) (err error) { // MARKER: Resume
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// Validate that the flow is interrupted
	var flowStatus string
	err = db.QueryRowContext(ctx,
		"SELECT status FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	if flowStatus != foremanapi.StatusInterrupted {
		return errors.New("flow is not interrupted (status: %s)", flowStatus, http.StatusConflict)
	}

	// Walk up the surgraph chain to collect parent flows/steps (for re-parking and flow transitions)
	upFlowIDs, upStepIDs, upCompositeIDs, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
	if err != nil {
		return errors.Trace(err)
	}

	// Walk down the subgraph chain to find the leaf interrupted step
	downFlowIDs, downStepIDs, downCompositeIDs, err := svc.interruptedSubgraphChain(ctx, shardNum, flowID, flowToken)
	if err != nil {
		return errors.Trace(err)
	}

	// Combine: all flow IDs and composite IDs (up[1:] reversed + down, dedup the starting flow)
	// upFlowIDs[0] == downFlowIDs[0] == flowID, so skip upFlowIDs[0]
	chainFlowIDs := append([]any{}, upFlowIDs...)
	chainCompositeIDs := append([]string{}, upCompositeIDs...)
	chainFlowIDs = append(chainFlowIDs, downFlowIDs[1:]...)
	chainCompositeIDs = append(chainCompositeIDs, downCompositeIDs[1:]...)

	// All steps to re-park: upStepIDs (parent surgraph steps) + downStepIDs except the leaf
	leafStepID := downStepIDs[len(downStepIDs)-1]
	parkStepIDs := append([]any{}, upStepIDs...)
	parkStepIDs = append(parkStepIDs, downStepIDs[:len(downStepIDs)-1]...)

	// Load the leaf step's changes and its flow's graph (for reducers).
	// State column is invariant - resumeData is merged into changes so it
	// accumulates alongside the task's prior output and is visible during fan-in.
	var leafChangesJSON string
	var leafGraphJSON string
	err = db.QueryRowContext(ctx,
		"SELECT s.changes, f.graph FROM microbus_steps s JOIN microbus_flows f ON s.flow_id=f.flow_id WHERE s.step_id=? AND s.status=?",
		leafStepID, foremanapi.StatusInterrupted,
	).Scan(&leafChangesJSON, &leafGraphJSON)
	if err == sql.ErrNoRows {
		return errors.New("step is already being resumed", http.StatusConflict)
	}
	if err != nil {
		return errors.Trace(err)
	}

	// Merge resumeData into the leaf step's changes (not state) so it accumulates
	// with the task's prior output and is visible during fan-in.
	newChangesJSON := leafChangesJSON
	if resumeData != nil {
		resumeDataJSON, err := json.Marshal(resumeData)
		if err != nil {
			return errors.Trace(err)
		}
		var resumeMap map[string]any
		if err = json.Unmarshal(resumeDataJSON, &resumeMap); err != nil {
			return errors.Trace(err)
		}
		if len(resumeMap) > 0 {
			var leafGraph workflow.Graph
			if err = json.Unmarshal([]byte(leafGraphJSON), &leafGraph); err != nil {
				return errors.Trace(err)
			}
			var leafChanges map[string]any
			if err = json.Unmarshal([]byte(leafChangesJSON), &leafChanges); err != nil {
				return errors.Trace(err)
			}
			merged, err := workflow.MergeState(leafChanges, resumeMap, leafGraph.Reducers())
			if err != nil {
				return errors.Trace(err)
			}
			mergedJSON, err := json.Marshal(merged)
			if err != nil {
				return errors.Trace(err)
			}
			newChangesJSON = string(mergedJSON)
		}
	}

	// Atomically re-park surgraph steps, reset the leaf step, and transition flows
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Clear interrupt_payload on all steps in the chain
	allStepIDs := append([]any{leafStepID}, parkStepIDs...)
	clearPlaceholders := strings.Repeat("?,", len(allStepIDs)-1) + "?"
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET interrupt_payload='{}' WHERE step_id IN ("+clearPlaceholders+")",
		allStepIDs...,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Re-park surgraph steps (restore to running with far-future lease)
	if len(parkStepIDs) > 0 {
		parkMs := max(svc.RetentionDays(), 1) * 24 * 60 * 60 * 1000
		parkPlaceholders := strings.Repeat("?,", len(parkStepIDs)-1) + "?"
		parkArgs := append([]any{foremanapi.StatusRunning, parkMs}, parkStepIDs...)
		parkArgs = append(parkArgs, foremanapi.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC() WHERE step_id IN ("+parkPlaceholders+") AND status=?",
			parkArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Reset the leaf step to pending with updated changes (resume data merged into changes)
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, changes=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
		foremanapi.StatusPending, newChangesJSON, leafStepID, foremanapi.StatusInterrupted,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("step is already being resumed", http.StatusConflict)
	}

	// Transition all flows in the chain to running.
	// Each flow only transitions if it has no more interrupted steps.
	var resumedFlows []int // Indices of flows that transitioned to running
	for i, chainFlowID := range chainFlowIDs {
		flowRes, err := tx.ExecContext(ctx,
			"UPDATE microbus_flows SET status=?, updated_at=NOW_UTC() WHERE flow_id=? AND status=? AND (SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND status=?)=0",
			foremanapi.StatusRunning, chainFlowID, foremanapi.StatusInterrupted,
			chainFlowID, foremanapi.StatusInterrupted,
		)
		if err != nil {
			return errors.Trace(err)
		}
		if n, _ := flowRes.RowsAffected(); n > 0 {
			resumedFlows = append(resumedFlows, i)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	// Notifications (outside the transaction)
	for _, i := range resumedFlows {
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, chainCompositeIDs[i], foremanapi.StatusRunning)
	}

	// If another step anywhere in the chain is still interrupted with a payload,
	// propagate it up so the caller sees it on the next State()/Await() call.
	// The next interrupt can be at any level (fan-out sibling at any depth).
	if len(parkStepIDs) > 0 {
		flowPlaceholders := strings.Repeat("?,", len(chainFlowIDs)-1) + "?"
		findArgs := append([]any{foremanapi.StatusInterrupted}, chainFlowIDs...)
		var nextPayloadJSON string
		err = db.QueryRowContext(ctx,
			"SELECT interrupt_payload FROM microbus_steps WHERE status=? AND flow_id IN ("+flowPlaceholders+") AND interrupt_payload!='{}' ORDER BY updated_at LIMIT_OFFSET(1, 0)",
			findArgs...,
		).Scan(&nextPayloadJSON)
		if err == nil && nextPayloadJSON != "" && nextPayloadJSON != "{}" {
			// Propagate the next interrupt's payload to all surgraph steps in the chain
			parkPlaceholders := strings.Repeat("?,", len(parkStepIDs)-1) + "?"
			payloadArgs := []any{nextPayloadJSON}
			payloadArgs = append(payloadArgs, parkStepIDs...)
			db.ExecContext(ctx,
				"UPDATE microbus_steps SET interrupt_payload=? WHERE step_id IN ("+parkPlaceholders+") AND interrupt_payload='{}'",
				payloadArgs...,
			)
		}
	}

	// Enqueue the leaf step for processing
	foremanapi.NewClient(svc).Enqueue(ctx, shardNum, leafStepID.(int))
	return nil
}

/*
Fork creates a new flow from an existing step's checkpoint. The step identified by stepKey is re-executed in the
new flow with the (optionally overridden) state.
*/
func (svc *Service) Fork(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error) { // MARKER: Fork
	shardNum, stepID, stepToken, err := parseStepKey(stepKey)
	if err != nil {
		return "", errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Look up the step by step key
	var flowID, stepDepth int
	var taskName, stateStr string
	err = db.QueryRowContext(ctx,
		"SELECT flow_id, step_depth, task_name, state FROM microbus_steps WHERE step_id=? AND step_token=?",
		stepID, stepToken,
	).Scan(&flowID, &stepDepth, &taskName, &stateStr)
	if err == sql.ErrNoRows {
		return "", errors.New("step not found", http.StatusNotFound)
	}
	if err != nil {
		return "", errors.Trace(err)
	}

	// Look up the parent flow
	var workflowName, graphJSON, actorClaimsJSON, traceParent, parentBreakpointsJSON string
	err = db.QueryRowContext(ctx,
		"SELECT workflow_name, graph, actor_claims, trace_parent, breakpoints FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&workflowName, &graphJSON, &actorClaimsJSON, &traceParent, &parentBreakpointsJSON)
	if err == sql.ErrNoRows {
		return "", errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return "", errors.Trace(err)
	}

	// Parse the graph for reducers and time budgets
	var graph workflow.Graph
	if err = json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		return "", errors.Trace(err)
	}

	// Start from the step's immutable state, then merge overrides using reducers
	var rawState map[string]any
	if err = json.Unmarshal([]byte(stateStr), &rawState); err != nil {
		return "", errors.Trace(err)
	}
	if stateOverrides != nil {
		overridesJSON, err := json.Marshal(stateOverrides)
		if err != nil {
			return "", errors.Trace(err)
		}
		var overridesMap map[string]any
		if err = json.Unmarshal(overridesJSON, &overridesMap); err != nil {
			return "", errors.Trace(err)
		}
		if len(overridesMap) > 0 {
			mergedState, err := workflow.MergeState(rawState, overridesMap, graph.Reducers())
			if err != nil {
				return "", errors.Trace(err)
			}
			rawState = mergedState
		}
	}
	mergedStateJSON, err := json.Marshal(rawState)
	if err != nil {
		return "", errors.Trace(err)
	}

	newFlowToken := utils.RandomIdentifier(16)

	// Atomically create the forked flow and its step within a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer tx.Rollback()

	newFlowID, err := tx.InsertReturnID(ctx, "flow_id",
		"INSERT INTO microbus_flows (flow_token, workflow_name, graph, actor_claims, status, forked_flow_id, forked_step_depth, trace_parent, breakpoints)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		newFlowToken, workflowName, graphJSON, actorClaimsJSON, foremanapi.StatusCreated, flowID, stepDepth, traceParent, parentBreakpointsJSON,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	forkTimeBudget := svc.taskTimeBudget(&graph, taskName)
	newStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lease_expires)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?))",
		newFlowID, stepDepth, utils.RandomIdentifier(16), taskName, string(mergedStateJSON), foremanapi.StatusCreated, forkTimeBudget.Milliseconds(), leaseMargin.Milliseconds(),
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		newStepID, newFlowID,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return "", errors.Trace(err)
	}
	svc.LogDebug(ctx, "Forked flow", "stepKey", stepKey, "workflow", workflowName)

	return fmt.Sprintf("%d-%d-%s", shardNum, newFlowID, newFlowToken), nil
}

// createSubgraphFlow creates a subgraph flow for a subgraph transition in "created" status.
// The subgraph flow's surgraph_flow_id and surgraph_step_depth link it back to
// the surgraph for completion propagation. The caller must call Start to begin execution.
func (svc *Service) createSubgraphFlow(ctx context.Context, shardNum int, surgraphFlowID int, surgraphStepDepth int, subgraphWorkflowName string, subgraphGraph *workflow.Graph, surgraphState map[string]any, actorClaimsJSON string, traceParent string, breakpointsJSON string) (subgraphFlowKey string, err error) {
	// Create the subgraph flow via createWithGraph (reuses flow+step INSERT logic)
	// Filter the parent state through the subgraph's declared inputs.
	subgraphState := workflow.FilterState(surgraphState, subgraphGraph.Inputs())
	subgraphFlowKey, err = svc.createWithGraph(ctx, shardNum, subgraphWorkflowName, subgraphGraph, subgraphState, 0, "")
	if err != nil {
		return "", errors.Trace(err)
	}
	_, subgraphFlowID, _, err := parseFlowKey(subgraphFlowKey)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Set surgraph linkage, override actor claims / trace parent, and copy breakpoints from surgraph
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_flows SET surgraph_flow_id=?, surgraph_step_depth=?, actor_claims=?, trace_parent=?, breakpoints=?, updated_at=NOW_UTC() WHERE flow_id=?",
		surgraphFlowID, surgraphStepDepth, actorClaimsJSON, traceParent, breakpointsJSON, subgraphFlowID,
	)
	if err != nil {
		// Edge case. The subgraph flow will remain orphaned and eventually purged
		return "", errors.Trace(err)
	}

	return subgraphFlowKey, nil
}

/*
Cancel cancels a flow that is not yet in a terminal status.
*/
/*
BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
*/
func (svc *Service) BreakBefore(ctx context.Context, flowKey string, taskName string, enabled bool) (err error) { // MARKER: BreakBefore
	return svc.setBreakpoint(ctx, flowKey, taskName, enabled)
}

// setBreakpoint adds or removes a breakpoint key in the flow's breakpoints JSON column.
func (svc *Service) setBreakpoint(ctx context.Context, flowKey string, key string, enabled bool) error {
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	var breakpointsJSON string
	err = db.QueryRowContext(ctx,
		"SELECT breakpoints FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&breakpointsJSON)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}

	var breakpoints map[string]string
	if err := json.Unmarshal([]byte(breakpointsJSON), &breakpoints); err != nil {
		breakpoints = make(map[string]string)
	}

	if enabled {
		breakpoints[key] = "b"
	} else {
		delete(breakpoints, key)
	}

	updatedJSON, err := json.Marshal(breakpoints)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = db.ExecContext(ctx,
		"UPDATE microbus_flows SET breakpoints=?, updated_at=NOW_UTC() WHERE flow_id=? AND flow_token=?",
		string(updatedJSON), flowID, flowToken,
	)
	return errors.Trace(err)
}

/*
Cancel cancels a flow that is not yet in a terminal status.
It cancels the entire chain: all parent surgraph flows (upward), the flow itself,
and all descendant subgraph flows (downward) - atomically in a single transaction.
*/
func (svc *Service) Cancel(ctx context.Context, flowKey string) (err error) { // MARKER: Cancel
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// Validate the flow exists and is not already terminal
	var flowStatus string
	var notifyHostname string
	err = db.QueryRowContext(ctx,
		"SELECT status, notify_hostname FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus, &notifyHostname)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	if flowStatus == foremanapi.StatusCompleted || flowStatus == foremanapi.StatusFailed || flowStatus == foremanapi.StatusCancelled {
		return errors.New("flow is already in terminal status", http.StatusConflict)
	}
	notifyHostname = strings.TrimSpace(notifyHostname)

	// Build the full surgraph chain (current flow + all parent flows up to root)
	surgraphFlowIDs, surgraphStepIDs, surgraphCompositeIDs, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
	if err != nil {
		return errors.Trace(err)
	}

	// Find all descendant subgraph flows (downward, iteratively)
	descendantFlowIDs, descendantCompositeIDs, err := svc.allSubgraphFlows(ctx, shardNum, flowID)
	if err != nil {
		return errors.Trace(err)
	}

	// Combine: all flows to cancel = surgraph chain + descendants
	allFlowIDs := append([]any{}, surgraphFlowIDs...)
	allFlowIDs = append(allFlowIDs, descendantFlowIDs...)
	allCompositeIDs := append([]string{}, surgraphCompositeIDs...)
	allCompositeIDs = append(allCompositeIDs, descendantCompositeIDs...)

	// Atomically cancel all steps, compute final states, and cancel all flows
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Cancel all active steps across all flows in one UPDATE per table
	flowPlaceholders := strings.Repeat("?,", len(allFlowIDs)-1) + "?"
	stepArgs := append([]any{foremanapi.StatusCancelled}, allFlowIDs...)
	stepArgs = append(stepArgs, foremanapi.StatusCreated, foremanapi.StatusPending, foremanapi.StatusInterrupted, foremanapi.StatusRunning)
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status IN (?, ?, ?, ?)",
		stepArgs...,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Cancel surgraph steps in the chain (if any)
	if len(surgraphStepIDs) > 0 {
		surgraphStepPlaceholders := strings.Repeat("?,", len(surgraphStepIDs)-1) + "?"
		surgraphStepArgs := append([]any{foremanapi.StatusCancelled}, surgraphStepIDs...)
		surgraphStepArgs = append(surgraphStepArgs, foremanapi.StatusCreated, foremanapi.StatusPending, foremanapi.StatusInterrupted, foremanapi.StatusRunning)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE step_id IN ("+surgraphStepPlaceholders+") AND status IN (?, ?, ?, ?)",
			surgraphStepArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Compute final_state for each flow (inside transaction so it reflects the cancelled steps)
	finalStates := make([]string, len(allFlowIDs))
	for i, fid := range allFlowIDs {
		fs, _, err := svc.computeFinalState(ctx, tx, fid.(int))
		if err != nil {
			return errors.Trace(err)
		}
		finalStates[i] = fs
	}

	// Cancel all flows with their computed final_state via CASE
	caseClause := "CASE"
	flowArgs := []any{}
	for i, fid := range allFlowIDs {
		caseClause += " WHEN flow_id=? THEN ?"
		flowArgs = append(flowArgs, fid, finalStates[i])
	}
	caseClause += " END"
	flowArgs = append(flowArgs, foremanapi.StatusCancelled)
	flowArgs = append(flowArgs, allFlowIDs...)
	flowArgs = append(flowArgs, foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled)
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_flows SET final_state="+caseClause+", status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status NOT IN (?, ?, ?)",
		flowArgs...,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("flow is already in terminal status", http.StatusConflict)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	// Notifications (outside the transaction)
	// Use the root flow's notify_hostname (last element of the surgraph chain)
	rootIdx := len(surgraphFlowIDs) - 1
	rootCompositeID := surgraphCompositeIDs[rootIdx]
	var rootNotifyHostname string
	db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", surgraphFlowIDs[rootIdx]).Scan(&rootNotifyHostname)
	rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
	if rootNotifyHostname != "" {
		// Use the root flow's final_state for the notification
		var finalState map[string]any
		if err := json.Unmarshal([]byte(finalStates[rootIdx]), &finalState); err == nil {
			raw := workflow.NewRawFlow()
			raw.SetRawState(finalState)
			foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, rootCompositeID, foremanapi.StatusCancelled, raw.RawState())
		}
	}
	for i, cid := range allCompositeIDs {
		svc.LogInfo(ctx, "Flow status transition", "flow", allFlowIDs[i], "to", foremanapi.StatusCancelled)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, cid, foremanapi.StatusCancelled)
	}
	return nil
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

/*
History returns the step-by-step execution history of a flow.
*/
func (svc *Service) History(ctx context.Context, flowKey string) (steps []foremanapi.FlowStep, err error) { // MARKER: History
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Validate the flow exists
	var exists int
	err = db.QueryRowContext(ctx,
		"SELECT 1 FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Recursively collect steps from the flow and its fork ancestors
	return svc.historyBeforeStep(ctx, shardNum, flowID, 0)
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
			"SELECT step_id, step_token, step_depth, task_name, state, changes, status, error, updated_at FROM microbus_steps WHERE flow_id=? AND step_depth<? ORDER BY step_depth, step_id",
			flowID, beforeStepDepth,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			"SELECT step_id, step_token, step_depth, task_name, state, changes, status, error, updated_at FROM microbus_steps WHERE flow_id=? ORDER BY step_depth, step_id",
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
		err := rows.Scan(&stepID, &stepToken, &step.StepDepth, &step.TaskName, &stateJSON, &changesJSON, &step.Status, &errMsg, &step.UpdatedAt)
		if err != nil {
			return nil, errors.Trace(err)
		}
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

/*
Retry re-executes the last failed step of a flow.
*/
func (svc *Service) Retry(ctx context.Context, flowKey string) (err error) { // MARKER: Retry
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	// Validate that the flow is failed and get the current step
	var flowStatus string
	var flowStepID int
	err = db.QueryRowContext(ctx,
		"SELECT status, step_id FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus, &flowStepID)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	if flowStatus != foremanapi.StatusFailed {
		return errors.New("flow is not failed (status: %s)", flowStatus, http.StatusConflict)
	}
	fanOut := flowStepID == 0

	// Load failed step(s) for duplication
	type failedStep struct {
		stepID       int
		stepDepth    int
		taskName     string
		state        string
		timeBudgetMs int
	}
	var failedSteps []failedStep
	if fanOut {
		// Potentially multiple failed steps
		rows, err := db.QueryContext(ctx,
			"SELECT step_id, step_depth, task_name, state, time_budget_ms FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, foremanapi.StatusFailed,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		for rows.Next() {
			var fs failedStep
			err := rows.Scan(&fs.stepID, &fs.stepDepth, &fs.taskName, &fs.state, &fs.timeBudgetMs)
			if err != nil {
				return errors.Trace(err)
			}
			failedSteps = append(failedSteps, fs)
		}
		if err := rows.Err(); err != nil {
			return errors.Trace(err)
		}
	} else {
		var fs failedStep
		err = db.QueryRowContext(ctx,
			"SELECT step_id, step_depth, task_name, state, time_budget_ms FROM microbus_steps WHERE step_id=? AND status=?",
			flowStepID, foremanapi.StatusFailed,
		).Scan(&fs.stepID, &fs.stepDepth, &fs.taskName, &fs.state, &fs.timeBudgetMs)
		if err != nil && err != sql.ErrNoRows {
			return errors.Trace(err)
		}
		if err == nil {
			failedSteps = []failedStep{fs}
		}
	}
	if len(failedSteps) == 0 {
		return errors.New("flow is already being retried", http.StatusConflict)
	}

	// Atomically mark failed steps as retried, insert new copies, and update flow within a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Mark failed steps as retried (preserves error and history)
	placeholders := strings.Repeat("?,", len(failedSteps)-1) + "?"
	args := []any{foremanapi.StatusRetried}
	for _, fs := range failedSteps {
		args = append(args, fs.stepID)
	}
	args = append(args, foremanapi.StatusFailed)
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE step_id IN ("+placeholders+") AND status=?",
		args...,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("flow is already being retried", http.StatusConflict)
	}

	// Insert new copies of the failed steps with pending status
	var newStepIDs []int
	for _, fs := range failedSteps {
		newStepID, err := tx.InsertReturnID(ctx, "step_id",
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms)"+
				" VALUES (?, ?, ?, ?, ?, ?, ?)",
			flowID, fs.stepDepth, utils.RandomIdentifier(16), fs.taskName, fs.state, foremanapi.StatusPending, fs.timeBudgetMs,
		)
		if err != nil {
			return errors.Trace(err)
		}
		newStepIDs = append(newStepIDs, int(newStepID))
	}

	// Update flow's step_id to point to the new step(s)
	nextFlowStepID := newStepIDs[0]
	if len(newStepIDs) > 1 {
		nextFlowStepID = 0 // Fan out
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, step_id=?, updated_at=NOW_UTC() WHERE flow_id=? AND status=?",
		foremanapi.StatusRunning, nextFlowStepID, flowID, foremanapi.StatusFailed,
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "from", foremanapi.StatusFailed, "to", foremanapi.StatusRunning)
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, flowKey, foremanapi.StatusRunning)

	// Enqueue all new steps for processing (outside the transaction)
	for _, stepID := range newStepIDs {
		foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, int(stepID))
	}
	return nil
}

/*
List queries flows by status or workflow name. Results are ordered by flow ID descending (newest first).
Set CursorFlowID in the query to the last result's flow ID to paginate. Limit defaults to 100 if not set.
*/
func (svc *Service) List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error) { // MARKER: List
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	var cursorNum int
	if query.CursorFlowID != "" {
		_, cursorNum, _, err = parseFlowKey(query.CursorFlowID)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	// Parse thread key if provided
	var threadShardNum, threadFlowID int
	if query.ThreadKey != "" {
		var threadFlowToken string
		threadShardNum, threadFlowID, threadFlowToken, err = parseFlowKey(query.ThreadKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// Look up the thread_id from the provided flow
		db, err := svc.shard(threadShardNum)
		if err != nil {
			return nil, errors.Trace(err)
		}
		var resolvedThreadID int
		err = db.QueryRowContext(ctx,
			"SELECT thread_id FROM microbus_flows WHERE flow_id=? AND flow_token=?",
			threadFlowID, threadFlowToken,
		).Scan(&resolvedThreadID)
		if err != nil {
			return nil, errors.New("flow not found", http.StatusNotFound)
		}
		threadFlowID = resolvedThreadID
	}

	conditions := []string{"f.surgraph_flow_id=0"}
	var args []any
	if query.Status != "" {
		conditions = append(conditions, "f.status=?")
		args = append(args, query.Status)
	}
	if query.WorkflowName != "" {
		conditions = append(conditions, "f.workflow_name=?")
		args = append(args, query.WorkflowName)
	}
	if query.ThreadKey != "" {
		conditions = append(conditions, "f.thread_id=?")
		args = append(args, threadFlowID)
	}
	if cursorNum > 0 {
		conditions = append(conditions, "f.flow_id<?")
		args = append(args, cursorNum)
	}
	where := " WHERE " + strings.Join(conditions, " AND ")
	args = append(args, limit)
	stmt := "SELECT f.flow_id, f.flow_token, f.thread_id, f.thread_token, f.workflow_name, f.status, s.task_name" +
		" FROM microbus_flows f LEFT JOIN microbus_steps s ON f.step_id = s.step_id" +
		where +
		" ORDER BY f.flow_id DESC LIMIT_OFFSET(?, 0)"

	var mu sync.Mutex
	queryShard := func(shardIdx int) func() error {
		// When filtering by thread, only query the shard that contains the thread
		if query.ThreadKey != "" && shardIdx != threadShardNum {
			return func() error { return nil }
		}
		return func() (err error) {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}
			rows, err := db.QueryContext(ctx, stmt, args...)
			if err != nil {
				return errors.Trace(err)
			}
			defer rows.Close()

			var shardFlows []foremanapi.FlowSummary
			for rows.Next() {
				var summary foremanapi.FlowSummary
				var flowID, threadID int
				var flowToken, threadToken string
				var taskName sql.NullString
				err = rows.Scan(&flowID, &flowToken, &threadID, &threadToken, &summary.WorkflowName, &summary.Status, &taskName)
				if err != nil {
					return errors.Trace(err)
				}
				summary.FlowID = fmt.Sprintf("%d-%d-%s", shardIdx, flowID, strings.TrimSpace(flowToken))
				summary.ThreadKey = fmt.Sprintf("%d-%d-%s", shardIdx, threadID, strings.TrimSpace(threadToken))
				summary.Status = strings.TrimSpace(summary.Status)
				summary.TaskName = taskName.String
				shardFlows = append(shardFlows, summary)
			}
			if err := rows.Err(); err != nil {
				return errors.Trace(err)
			}

			mu.Lock()
			flows = append(flows, shardFlows...)
			mu.Unlock()
			return nil
		}
	}
	jobs := make([]func() error, svc.numDBShards())
	for i := range jobs {
		jobs[i] = queryShard(i)
	}
	err = svc.Parallel(jobs...)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Sort by flow ID descending across shards and apply the limit
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].FlowID > flows[j].FlowID
	})
	if len(flows) > limit {
		flows = flows[:limit]
	}
	return flows, nil
}

/*
Enqueue adds a step to the local work queue for processing. This endpoint is called by foreman replicas to distribute
work across the cluster.
*/
func (svc *Service) Enqueue(ctx context.Context, shard int, stepID int) (err error) { // MARKER: Enqueue
	if frame.Of(ctx).FromHost() != Hostname {
		return errors.New("enqueue is restricted to foreman replicas", http.StatusForbidden)
	}
	svc.queue.push(job{stepID: stepID, shard: shard})
	return nil
}

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

// workerLoop is the main loop for a worker goroutine. It pops step IDs from the queue and executes one task per iteration.
func (svc *Service) workerLoop(ctx context.Context) {
	for {
		j, ok := svc.queue.pop()
		if !ok {
			return // Queue closed
		}
		err := errors.CatchPanic(func() error {
			return svc.processStep(ctx, j.stepID, j.shard)
		})
		if err != nil {
			svc.LogError(ctx, "Failed to process step",
				"stepID", j.stepID,
				"error", err,
			)
		}
	}
}

// processStep acquires a step, executes its task, and enqueues the next step if applicable.
func (svc *Service) processStep(ctx context.Context, stepID int, shardNum int) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	// Read the step's time budget and flow_id to compute lease duration and enable parallel loading
	var timeBudgetMs int
	var flowID int
	err = db.QueryRowContext(ctx,
		"SELECT time_budget_ms, flow_id FROM microbus_steps WHERE step_id=?",
		stepID,
	).Scan(&timeBudgetMs, &flowID)
	if err != nil {
		return nil // Step doesn't exist or was deleted
	}
	leaseMs := timeBudgetMs + int(leaseMargin.Milliseconds())

	// Try to acquire the step by atomically transitioning it from pending to running
	res, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC()"+
			" WHERE step_id=? AND status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC()",
		foremanapi.StatusRunning, leaseMs, stepID, foremanapi.StatusPending,
	)
	if err != nil {
		return errors.Trace(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil // Already claimed or not yet due
	}

	// Load step data and flow data in parallel
	var stepDepth int
	var taskName string
	var stateJSON string
	var priorChangesJSON string
	var breakpointHit bool
	var attempt int
	var flowToken string
	var flowStatus string
	var workflowName string
	var graphJSON string
	var actorClaimsJSON string
	var traceParent string
	var notifyHostname string
	var breakpointsJSON string
	err = svc.Parallel(
		func() error {
			err := db.QueryRowContext(ctx,
				"SELECT step_depth, task_name, state, changes, breakpoint_hit, attempt FROM microbus_steps WHERE step_id=?",
				stepID,
			).Scan(&stepDepth, &taskName, &stateJSON, &priorChangesJSON, &breakpointHit, &attempt)
			return errors.Trace(err)
		},
		func() error {
			err := db.QueryRowContext(ctx,
				"SELECT flow_token, status, workflow_name, graph, actor_claims, trace_parent, notify_hostname, breakpoints FROM microbus_flows WHERE flow_id=?",
				flowID,
			).Scan(&flowToken, &flowStatus, &workflowName, &graphJSON, &actorClaimsJSON, &traceParent, &notifyHostname, &breakpointsJSON)
			return errors.Trace(err)
		},
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Terminal flow check: Cancel, failStep, and flow completion set the flow to a terminal status first, then update steps.
	// If this worker claimed the step before the step update, catch it here.
	flowStatus = strings.TrimSpace(flowStatus)
	flowToken = strings.TrimSpace(flowToken)
	if flowStatus == foremanapi.StatusCancelled || flowStatus == foremanapi.StatusFailed || flowStatus == foremanapi.StatusCompleted {
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=?",
			flowStatus, stepID,
		)
		return errors.Trace(err)
	}

	// Deserialize graph
	var graph workflow.Graph
	err = json.Unmarshal([]byte(graphJSON), &graph)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}

	// Build the Flow carrier
	var state map[string]any
	err = json.Unmarshal([]byte(stateJSON), &state)
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}
	var priorChanges map[string]any
	err = json.Unmarshal([]byte(priorChangesJSON), &priorChanges)
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

	// Check breakpoints: pause before executing the task if a breakpoint matches.
	// Skip if this step already hit a breakpoint (breakpoint_hit flag prevents re-triggering on Resume).
	if !breakpointHit {
		var breakpoints map[string]string
		err := json.Unmarshal([]byte(breakpointsJSON), &breakpoints)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		if len(breakpoints) > 0 && breakpoints[taskName] == "b" {
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
			flowArgs := append([]any{foremanapi.StatusInterrupted}, chainFlowIDs...)
			flowArgs = append(flowArgs, foremanapi.StatusRunning, foremanapi.StatusInterrupted)
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
			stepArgs := append([]any{foremanapi.StatusInterrupted}, allStepIDs...)
			stepArgs = append(stepArgs, foremanapi.StatusRunning, foremanapi.StatusInterrupted)
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
				foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, foremanapi.StatusInterrupted)
			}
			// Fire OnFlowStopped on the root flow's notify hostname (if set)
			rootFlowID := chainFlowIDs[len(chainFlowIDs)-1]
			rootCompositeID := chainCompositeIDs[len(chainCompositeIDs)-1]
			var rootNotifyHostname string
			db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", rootFlowID).Scan(&rootNotifyHostname)
			rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
			if rootNotifyHostname != "" {
				foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, rootCompositeID, foremanapi.StatusInterrupted, nil)
			}

			svc.IncrementStepsExecuted(ctx, 1, taskName, foremanapi.StatusInterrupted)
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
	var currentFlowStatus string
	var actorClaims map[string]any
	errorRouted := false
	var actorToken string

	// Subgraph handling: if the task is a subgraph, create and start a subgraph flow
	// instead of executing it as an HTTP task. The surgraph step stays running with
	// a far-future lease. When the subgraph flow completes, completeSurgraphFlow merges
	// the result back and re-enqueues this step for transition evaluation.
	if graph.IsSubgraph(taskName) {
		// Check if a subgraph flow already exists for this surgraph step.
		// If it completed, its final_state was already merged into our changes by completeSurgraphFlow.
		// Skip to transition evaluation (fall through the subgraph block).
		// If it is still active, park and wait. If none exists, create one.
		var activeSubgraphCount, completedSubgraphCount int
		err = svc.Parallel(
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM microbus_flows WHERE surgraph_flow_id=? AND surgraph_step_depth=? AND status IN (?, ?, ?)",
					flowID, stepDepth,
					foremanapi.StatusCreated, foremanapi.StatusRunning, foremanapi.StatusInterrupted,
				).Scan(&activeSubgraphCount)
				return errors.Trace(err)
			},
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM microbus_flows WHERE surgraph_flow_id=? AND surgraph_step_depth=? AND status=?",
					flowID, stepDepth, foremanapi.StatusCompleted,
				).Scan(&completedSubgraphCount)
				return errors.Trace(err)
			},
		)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		if completedSubgraphCount > 0 {
			// Subgraph flow already completed - its final_state was merged into our changes by completeSurgraphFlow.
			// Skip task execution and proceed to post-execution handling.
			resultFlow = flow
			goto postExecution
		}
		if activeSubgraphCount == 0 {
			// No subgraph flow exists yet - create and start one
			subgraphGraph, err := svc.fetchGraph(ctx, taskName)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}
			mergedState, err := workflow.MergeState(state, priorChanges, nil)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}
			subgraphFlowID, err := svc.createSubgraphFlow(ctx, shardNum, flowID, stepDepth, taskName, subgraphGraph, mergedState, actorClaimsJSON, traceParent, breakpointsJSON)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}
			err = svc.Start(ctx, subgraphFlowID)
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}
		}
		// Park the surgraph step: extend lease far into the future (only if step is still running).
		// This is safe because the lease is always cleared by a code path:
		// - Subgraph flow completes -> completeSurgraphFlow sets step to PENDING with expired lease
		// - Subgraph flow fails -> failSurgraphFlow fails the surgraph step and flow
		// - Subgraph flow cancelled -> cancelSurgraphFlow cancels the surgraph flow
		// - Surgraph cancelled -> Cancel cascades to subgraph flow via cancelSubgraphFlows
		// The subgraph flow's own steps have normal short leases, so pollPendingSteps
		// recovers it if the foreman crashes after creating it.
		parkMs := max(svc.RetentionDays(), 1) * 24 * 60 * 60 * 1000
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			parkMs, stepID, foremanapi.StatusRunning,
		)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}

	// Mint an access token from the original actor's claims
	if err = json.Unmarshal([]byte(actorClaimsJSON), &actorClaims); err != nil {
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

	// Execute the task
	svc.LogDebug(ctx, "Executing task", "task", taskName, "flow", workflowName)
	resultFlow, err = svc.executeTask(taskCtx, taskName, flow, actorToken, time.Duration(timeBudgetMs)*time.Millisecond)
	if err != nil {
		// Record the input state on the span
		inputState, mergeErr := workflow.MergeState(state, priorChanges, nil)
		if mergeErr != nil {
			return errors.Trace(mergeErr)
		}
		for k, v := range inputState {
			taskSpan.SetAttributes("workflow.state."+k, v)
		}
		taskSpan.SetError(err)

		// Check for error transition before failing the flow
		if _, ok := graph.ErrorTransition(taskName); ok {
			svc.LogDebug(ctx, "Task error routed", "task", taskName, "flow", workflowName, "error", err)
			taskSpan.SetAttributes("workflow.command", "onError")
			errorRouted = true

			// Serialize the error as a TracedError into a synthetic result flow
			tracedErr := errors.Convert(err)
			resultFlow = workflow.NewRawFlow()
			resultFlow.SetRawState(state)
			resultFlow.SetRawChanges(nil)
			resultFlow.Set("onErr", tracedErr)
			goto postExecution
		}

		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}

	// Check if the flow was cancelled while the task was running
	err = db.QueryRowContext(ctx,
		"SELECT status FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&currentFlowStatus)
	if err != nil {
		return errors.Trace(err)
	}
	currentFlowStatus = strings.TrimSpace(currentFlowStatus)
	if currentFlowStatus == foremanapi.StatusCancelled {
		// Even if this errors out, the next iteration will detect that the flow is cancelled.
		db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE step_id=?",
			foremanapi.StatusCancelled, stepID,
		)
		return nil
	}

postExecution:
	// Accumulate this execution's changes on top of prior changes.
	// The state column is invariant; all mutations accumulate in the changes column.
	accumulatedChanges, _ := workflow.MergeState(priorChanges, resultFlow.RawChanges(), nil)
	changesJSON, _ := json.Marshal(accumulatedChanges)

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
		flowArgs := append([]any{foremanapi.StatusInterrupted}, chainFlowIDs...)
		flowArgs = append(flowArgs, foremanapi.StatusRunning, foremanapi.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET status=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status IN (?, ?)",
			flowArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}

		// Interrupt all steps in the chain, persisting changes on the current step via CASE
		allStepIDs := append([]any{stepID}, chainStepIDs...)
		stepPlaceholders := strings.Repeat("?,", len(allStepIDs)-1) + "?"
		stepArgs := []any{stepID, string(changesJSON), foremanapi.StatusInterrupted}
		stepArgs = append(stepArgs, allStepIDs...)
		stepArgs = append(stepArgs, foremanapi.StatusRunning, foremanapi.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET changes=CASE WHEN step_id=? THEN ? ELSE changes END, status=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id IN ("+stepPlaceholders+") AND status IN (?, ?)",
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
			foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, foremanapi.StatusInterrupted)
		}
		// Fire OnFlowStopped on the root flow's notify hostname (if set)
		rootFlowID := chainFlowIDs[len(chainFlowIDs)-1]
		rootCompositeID := chainCompositeIDs[len(chainCompositeIDs)-1]
		var rootNotifyHostname string
		db.QueryRowContext(ctx, "SELECT notify_hostname FROM microbus_flows WHERE flow_id=?", rootFlowID).Scan(&rootNotifyHostname)
		rootNotifyHostname = strings.TrimSpace(rootNotifyHostname)
		if rootNotifyHostname != "" {
			foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, rootCompositeID, foremanapi.StatusInterrupted, nil)
		}

		svc.IncrementStepsExecuted(ctx, 1, taskName, foremanapi.StatusInterrupted)
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
			string(changesJSON), stepID, foremanapi.StatusRunning,
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

		// Build the child's initial state: parent merged state + explicit input, filtered by DeclareInputs
		childInputState, err := workflow.MergeState(state, accumulatedChanges, nil)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		if subgraphInput != nil {
			childInputState, err = workflow.MergeState(childInputState, subgraphInput, subgraphGraph.Reducers())
			if err != nil {
				svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
				return errors.Trace(err)
			}
		}

		// Create and start the child flow
		subgraphFlowKey, err := svc.createSubgraphFlow(ctx, shardNum, flowID, stepDepth, subgraphWorkflow, subgraphGraph, childInputState, actorClaimsJSON, traceParent, breakpointsJSON)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}
		err = svc.Start(ctx, subgraphFlowKey)
		if err != nil {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
			return errors.Trace(err)
		}

		// Park the step: extend lease far into the future
		parkMs := max(svc.RetentionDays(), 1) * 24 * 60 * 60 * 1000
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET lease_expires=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			parkMs, stepID, foremanapi.StatusRunning,
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
		svc.LogDebug(ctx, "Task retried", "task", taskName, "flow", workflowName, "attempt", attempt)
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

		// State column is invariant. Accumulated changes already include this execution's output.
		// On the next attempt, the flow builder merges state+changes so the task sees everything.
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, changes=?, attempt=?, not_before=DATE_ADD_MILLIS(NOW_UTC(), ?), lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=?",
			foremanapi.StatusPending, string(changesJSON), attempt+1, retrySleepMs, stepID,
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
		svc.IncrementStepsExecuted(ctx, 1, taskName, foremanapi.StatusCompleted)
		taskSpan.SetAttributes("workflow.command", "next")
	}
	gotoTarget := resultFlow.GotoRequested()
	stepRes, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, changes=?, goto_next=?, updated_at=NOW_UTC() WHERE step_id=? AND status!=?",
		foremanapi.StatusCompleted, string(changesJSON), gotoTarget, stepID, foremanapi.StatusCancelled,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := stepRes.RowsAffected(); n == 0 {
		return nil // Step was cancelled concurrently
	}

	// Error-routed: cancel all siblings at this step depth and skip fan-in.
	// The error handler runs as a single next step, not as a fan-in merge.
	if errorRouted {
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE flow_id=? AND step_depth=? AND step_id!=? AND status IN (?, ?, ?)",
			foremanapi.StatusCancelled, flowID, stepDepth, stepID,
			foremanapi.StatusPending, foremanapi.StatusRunning, foremanapi.StatusInterrupted,
		)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		// Fan-in check: count unfinished and failed siblings in parallel
		var unfinishedSiblings int
		var failedSiblings int
		err = svc.Parallel(
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND step_depth=? AND step_id!=? AND status IN (?, ?, ?)",
					flowID, stepDepth, stepID,
					foremanapi.StatusPending, foremanapi.StatusRunning, foremanapi.StatusInterrupted,
				).Scan(&unfinishedSiblings)
				return errors.Trace(err)
			},
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND step_depth=? AND status IN (?, ?)",
					flowID, stepDepth,
					foremanapi.StatusFailed, foremanapi.StatusCancelled,
				).Scan(&failedSiblings)
				return errors.Trace(err)
			},
		)
		if err != nil {
			return errors.Trace(err)
		}
		if unfinishedSiblings > 0 {
			svc.LogDebug(ctx, "Pending siblings", "task", taskName, "unfinished", unfinishedSiblings, "flow", workflowName)
			return nil // Other branches still running; the last one to finish will advance the flow
		}
		if failedSiblings > 0 {
			svc.failStep(ctx, shardNum, stepID, flowID, flowToken, errors.New("sibling branch failed or was cancelled"), taskName)
			return nil
		}
	}
	// Evaluate transitions
	var nextTasks []nextStep
	if errorRouted {
		nextTasks, err = svc.evaluateErrorTransitions(&graph, taskName, resultFlow)
	} else {
		nextTasks, err = svc.evaluateTransitions(&graph, taskName, resultFlow)
	}
	if err != nil {
		svc.failStep(ctx, shardNum, stepID, flowID, flowToken, err, taskName)
		return errors.Trace(err)
	}

	// Filter out END pseudo-nodes
	var realTasks []nextStep
	for _, t := range nextTasks {
		if t.taskName != "" && t.taskName != workflow.END {
			realTasks = append(realTasks, t)
		}
	}

	if len(realTasks) == 0 {
		// Flow complete - no successor.
		// Set flow to completed first so that if we crash before updating the step,
		// the step's lease expires, pollPendingSteps enqueues it, and the
		// terminal flow check in recovery processStep marks it completed.
		svc.LogDebug(ctx, "Flow completed", "flow", workflowName)
		if _, err := svc.completeFlow(ctx, shardNum, flowID, flowToken, notifyHostname); err != nil {
			return errors.Trace(err)
		}
		_, err = db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE step_id=?",
			foremanapi.StatusCompleted, stepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}

	nextStepDepth := stepDepth + 1
	sleepMs := sleepDur.Milliseconds()

	// Atomically insert next steps and update the flow within a transaction.
	// The SELECT FOR UPDATE on the flow row serializes concurrent workers finishing
	// fan-out siblings, preventing duplicate next steps.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Acquire an exclusive lock on the flow row to serialize fan-in workers.
	// An UPDATE (rather than SELECT FOR UPDATE) is used to immediately acquire a
	// write lock, preventing SQLite shared-cache deadlocks where two deferred
	// transactions both hold read locks and neither can upgrade to write.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET updated_at=NOW_UTC() WHERE flow_id=?",
		flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Check if another worker already created next steps (fan-in race)
	var existingCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM microbus_steps WHERE flow_id=? AND step_depth=?",
		flowID, nextStepDepth,
	).Scan(&existingCount)
	if err != nil {
		return errors.Trace(err)
	}
	if existingCount > 0 {
		return nil // Another worker already created next steps
	}

	// Compute the merged state for the next step(s) inside the transaction
	// to avoid SQLite deadlocks from concurrent readers blocking writers.
	// Loads state and changes from all steps at this step_depth from the database.
	// In the sequential case, this is just our own state + changes.
	// In the fan-in case, changes from all siblings are merged.
	nextState, err := svc.mergeStepDepthState(ctx, tx, flowID, stepDepth, graph.Reducers())
	if err != nil {
		return errors.Trace(err)
	}
	nextStateJSON, _ := json.Marshal(nextState)

	var newStepIDs []int
	for _, next := range realTasks {
		svc.LogDebug(ctx, "Creating next task", "task", next.taskName, "flow", workflowName)

		// For forEach fan-out, inject the item into state (not changes)
		stepStateJSON := nextStateJSON
		if next.item != nil {
			perStepState := make(map[string]any, len(nextState)+1)
			maps.Copy(perStepState, nextState)
			perStepState[next.itemKey] = next.item
			stepStateJSON, _ = json.Marshal(perStepState)
		}

		nextTimeBudget := svc.taskTimeBudget(&graph, next.taskName)
		newStepID, err := tx.InsertReturnID(ctx, "step_id",
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, not_before)"+
				" VALUES (?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?))",
			flowID, nextStepDepth, utils.RandomIdentifier(16), next.taskName, string(stepStateJSON), foremanapi.StatusPending, nextTimeBudget.Milliseconds(), sleepMs,
		)
		if err != nil {
			return errors.Trace(err)
		}
		newStepIDs = append(newStepIDs, int(newStepID))
	}

	// Update flow's step_id: use 0 for fan-out (multiple steps), actual ID for sequential
	nextFlowStepID := newStepIDs[0]
	if len(newStepIDs) > 1 {
		nextFlowStepID = 0
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		nextFlowStepID, flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	// Enqueue next step(s) or schedule for later (outside the transaction)
	if sleepDur > 0 {
		svc.shortenNextPoll(time.Now().Add(sleepDur))
	} else {
		for _, id := range newStepIDs {
			foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, int(id))
		}
	}
	return nil
}

// mergeStepDepthState computes the merged state for the next step by loading state and changes
// from all steps at the given step_depth from the database. All siblings share the same base state;
// their changes are merged using the graph's reducers (ordered by updated_at).
// Fields without an explicit reducer use replace semantics (last write wins).
func (svc *Service) mergeStepDepthState(ctx context.Context, db sequel.Executor, flowID int, stepDepth int, reducers map[string]workflow.Reducer) (map[string]any, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT state, changes FROM microbus_steps WHERE flow_id=? AND step_depth=? ORDER BY updated_at, step_id",
		flowID, stepDepth,
	)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	var baseState map[string]any
	var allChanges []map[string]any
	for rows.Next() {
		var stateJSON, changesJSON string
		if err := rows.Scan(&stateJSON, &changesJSON); err != nil {
			return nil, errors.Trace(err)
		}
		if baseState == nil {
			if err := json.Unmarshal([]byte(stateJSON), &baseState); err != nil {
				return nil, errors.Trace(err)
			}
		}
		var changes map[string]any
		if err := json.Unmarshal([]byte(changesJSON), &changes); err != nil {
			return nil, errors.Trace(err)
		}
		allChanges = append(allChanges, changes)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Trace(err)
	}

	// Apply all changes onto the base state using the appropriate reducer per field
	merged := baseState
	for _, changes := range allChanges {
		merged, err = workflow.MergeState(merged, changes, reducers)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return merged, nil
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
	err = db.QueryRowContext(ctx,
		"SELECT surgraph_flow_id, surgraph_step_depth FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&surgraphFlowID, &surgraphStepDepth)
	if err != nil {
		return true, errors.Trace(err)
	}
	if surgraphFlowID != 0 {
		if err := svc.completeSurgraphFlow(ctx, shardNum, surgraphFlowID, surgraphStepDepth, workflowName, finalStateJSON); err != nil {
			return true, errors.Trace(err)
		}
	}

	return true, nil
}

// completeSurgraphFlow merges a completed subgraph flow's final state into the surgraph step's
// changes and re-enqueues the surgraph step for transition evaluation.
func (svc *Service) completeSurgraphFlow(ctx context.Context, shardNum int, surgraphFlowID int, surgraphStepDepth int, subgraphWorkflowName string, subgraphFinalStateJSON string) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	// Find the surgraph step, parent flow graph (for reducers), and subgraph graph (for output filtering) in parallel.
	// The step is identified by flow_id + step_depth + running status. We don't match on task_name
	// because dynamic subgraphs have a different task_name than the child workflow.
	var surgraphStepID int
	var surgraphStepChangesJSON string
	var surgraphGraphJSON string
	var subgraphGraphJSON string
	err = svc.Parallel(
		func() error {
			err := db.QueryRowContext(ctx,
				"SELECT step_id, changes FROM microbus_steps WHERE flow_id=? AND step_depth=? AND status=?",
				surgraphFlowID, surgraphStepDepth, foremanapi.StatusRunning,
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
		var workflowName string
		err = db.QueryRowContext(ctx,
			"SELECT surgraph_flow_id, surgraph_step_depth, workflow_name FROM microbus_flows WHERE flow_id=?",
			currentFlowID,
		).Scan(&surgraphFlowID, &surgraphStepDepth, &workflowName)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
		if surgraphFlowID == 0 {
			break // Reached the root flow
		}
		// Look up the flow token and the step_id of the surgraph step in parallel
		workflowName = strings.TrimSpace(workflowName)
		var surgraphFlowToken string
		var surgraphStepID int
		err = svc.Parallel(
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT flow_token FROM microbus_flows WHERE flow_id=?",
					surgraphFlowID,
				).Scan(&surgraphFlowToken)
				return errors.Trace(err)
			},
			func() error {
				err := db.QueryRowContext(ctx,
					"SELECT step_id FROM microbus_steps WHERE flow_id=? AND step_depth=? AND task_name=?",
					surgraphFlowID, surgraphStepDepth, workflowName,
				).Scan(&surgraphStepID)
				return errors.Trace(err)
			},
		)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
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

	// Find the latest step_depth for this flow
	var maxStepDepth int
	err = db.QueryRowContext(ctx,
		"SELECT MAX(step_depth) FROM microbus_steps WHERE flow_id=?",
		flowID,
	).Scan(&maxStepDepth)
	if err != nil {
		return "", "", errors.Trace(err)
	}

	// Merge state from all steps at the latest step_depth
	merged, err := svc.mergeStepDepthState(ctx, db, flowID, maxStepDepth, graph.Reducers())
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

// taskTimeBudget returns the time budget for a task.
// It uses the graph's per-task budget if set, otherwise the foreman's DefaultTimeBudget config.
func (svc *Service) taskTimeBudget(graph *workflow.Graph, taskName string) time.Duration {
	if budget := graph.TimeBudget(taskName); budget > 0 {
		return budget
	}
	return svc.DefaultTimeBudget()
}

// timerLoop sleeps until nextPoll, then calls pollPendingSteps. It re-evaluates
// whenever wakeTimer is signaled or the sleep expires. Capped at maxPollInterval.
func (svc *Service) timerLoop(ctx context.Context) {
	for {
		svc.nextPollLock.Lock()
		deadline := svc.nextPoll
		svc.nextPollLock.Unlock()

		delay := max(0, min(time.Until(deadline), maxPollInterval))

		select {
		case <-time.After(delay):
		case _, ok := <-svc.wakeTimer:
			if !ok {
				return // Channel closed, shutting down
			}
			continue // Re-evaluate nextPoll
		}

		err := svc.pollPendingSteps(ctx)
		if err != nil {
			svc.LogError(ctx, "Polling pending steps", "error", err)
		}
	}
}

// pollPendingSteps queries for pending steps due within maxPollInterval, pushes due steps
// to the work queue, and updates nextPoll to the earliest future not_before.
func (svc *Service) pollPendingSteps(ctx context.Context) error {
	type dueJob struct {
		shard  int
		stepID int
	}
	var mu sync.Mutex
	var nearestDelay time.Duration = -1
	var dueJobs []dueJob

	pollShard := func(shardIdx int) func() error {
		return func() (err error) {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}

			// Recover steps stuck in running whose lease has expired
			res, err := db.ExecContext(ctx,
				"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE status=? AND lease_expires<=NOW_UTC()",
				foremanapi.StatusPending, foremanapi.StatusRunning,
			)
			if err != nil {
				return errors.Trace(err)
			}
			if recovered, _ := res.RowsAffected(); recovered > 0 {
				svc.IncrementStepsRecovered(ctx, int(recovered))
			}

			// Find pending steps due within the poll horizon
			// Calculate delay as (not_before - NOW) in milliseconds to avoid clock skew between Go and the database
			rows, err := db.QueryContext(ctx,
				"SELECT step_id, DATE_DIFF_MILLIS(not_before, NOW_UTC()) FROM microbus_steps WHERE status=? AND not_before<=DATE_ADD_MILLIS(NOW_UTC(), ?) AND lease_expires<=NOW_UTC()",
				foremanapi.StatusPending, maxPollInterval.Milliseconds(),
			)
			if err != nil {
				return errors.Trace(err)
			}

			var shardDueJobs []dueJob
			var shardNearestDelay time.Duration = -1
			for rows.Next() {
				var id int
				var delayMs float64
				if err := rows.Scan(&id, &delayMs); err != nil {
					rows.Close()
					return errors.Trace(err)
				}
				if delayMs <= 0 {
					shardDueJobs = append(shardDueJobs, dueJob{shard: shardIdx, stepID: id})
				} else {
					d := time.Duration(delayMs * float64(time.Millisecond))
					if shardNearestDelay < 0 || d < shardNearestDelay {
						shardNearestDelay = d
					}
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return errors.Trace(err)
			}

			mu.Lock()
			dueJobs = append(dueJobs, shardDueJobs...)
			if shardNearestDelay >= 0 && (nearestDelay < 0 || shardNearestDelay < nearestDelay) {
				nearestDelay = shardNearestDelay
			}
			mu.Unlock()
			return nil
		}
	}
	jobs := make([]func() error, svc.numDBShards())
	for i := range jobs {
		jobs[i] = pollShard(i)
	}
	err := svc.Parallel(jobs...)
	if err != nil {
		return errors.Trace(err)
	}

	// Enqueue due steps outside the parallel poll
	for _, j := range dueJobs {
		foremanapi.NewMulticastClient(svc).Enqueue(ctx, j.shard, j.stepID)
	}

	// Wake up next to process the nearest job
	now := time.Now()
	svc.nextPollLock.Lock()
	if nearestDelay >= 0 {
		svc.nextPoll = now.Add(nearestDelay)
	} else {
		svc.nextPoll = now.Add(maxPollInterval)
	}
	svc.nextPollLock.Unlock()
	return nil
}

// shortenNextPoll updates nextPoll to tm if tm is earlier than the current value, and wakes the timer goroutine.
func (svc *Service) shortenNextPoll(tm time.Time) {
	svc.nextPollLock.Lock()
	if tm.Before(svc.nextPoll) {
		svc.nextPoll = tm
	}
	svc.nextPollLock.Unlock()

	// Non-blocking signal to wake the timer
	select {
	case svc.wakeTimer <- struct{}{}:
	default:
	}
}

// executeTask sends the flow to a task endpoint and returns the resulting flow.
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

// nextStep describes a step to be created during transition evaluation.
type nextStep struct {
	taskName string
	item     any    // non-nil for forEach fan-out
	itemKey  string // state key for the item (the "as" alias or forEach field name)
}

// evaluateTransitions determines the next task(s) to execute based on the graph transitions
// and the current flow state. Returns multiple candidates for fan-out.
func (svc *Service) evaluateTransitions(graph *workflow.Graph, currentTask string, flow *workflow.RawFlow) ([]nextStep, error) {
	// Check for goto override
	if gotoTarget := flow.GotoRequested(); gotoTarget != "" {
		for _, tr := range graph.Transitions() {
			if tr.From == currentTask && tr.To == gotoTarget && tr.WithGoto {
				return []nextStep{{taskName: gotoTarget}}, nil
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
// Returns at most one candidate (the first matching error transition).
func (svc *Service) evaluateErrorTransitions(graph *workflow.Graph, currentTask string, flow *workflow.RawFlow) ([]nextStep, error) {
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

	for _, tr := range graph.Transitions() {
		if tr.From != currentTask || !tr.OnError {
			continue
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
	return nil, nil
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

/*
PurgeExpiredFlows deletes terminated flows and their steps that have exceeded the retention period. It scans the
microbus_flows table by primary key in batches, stopping when it encounters records newer than the cutoff.
This heuristic assumes auto-increment IDs correlate with creation time, which holds per shard since each shard
has an independent sequence and flows are never migrated between shards. The updated_at check provides a safety
net for long-lived flows that were recently active (e.g. resumed after a long interruption).
*/
func (svc *Service) PurgeExpiredFlows(ctx context.Context) (err error) { // MARKER: PurgeExpiredFlows
	retentionDays := svc.RetentionDays()
	if retentionDays <= 0 {
		return nil // Purging disabled
	}
	cutoffMs := retentionDays * 24 * 60 * 60 * 1000
	cutoffTime := time.Now().Add(-time.Duration(cutoffMs) * time.Millisecond)
	batchSize := 1000

	purgeShard := func(shardIdx int) func() error {
		return func() (err error) {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}

			// Step 1: Scan flows by PK to find terminated flows older than the cutoff
			var lastFlowID int
			var expiredFlowIDs []int
			for {
				rows, err := db.QueryContext(ctx,
					"SELECT flow_id, created_at, updated_at FROM microbus_flows WHERE flow_id>? AND status IN (?, ?, ?) ORDER BY flow_id LIMIT_OFFSET(?, 0)",
					lastFlowID,
					foremanapi.StatusCompleted, foremanapi.StatusFailed, foremanapi.StatusCancelled,
					batchSize,
				)
				if err != nil {
					return errors.Trace(err)
				}
				count := 0
				pastCutoff := false
				for rows.Next() {
					var flowID int
					var createdAt, updatedAt time.Time
					if err := rows.Scan(&flowID, &createdAt, &updatedAt); err != nil {
						rows.Close()
						return errors.Trace(err)
					}
					lastFlowID = flowID
					count++
					// Use created_at as cursor-stop heuristic, but also check updated_at to protect recently-active flows
					if updatedAt.After(cutoffTime) {
						continue
					}
					if createdAt.After(cutoffTime) {
						pastCutoff = true
						continue
					}
					expiredFlowIDs = append(expiredFlowIDs, flowID)
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return errors.Trace(err)
				}

				if count < batchSize || pastCutoff {
					break // No more rows or we've passed the cutoff
				}
			}

			if len(expiredFlowIDs) == 0 {
				return nil
			}

			// Step 2: Delete steps for expired flows in batches
			for i := 0; i < len(expiredFlowIDs); i += batchSize {
				end := min(i+batchSize, len(expiredFlowIDs))
				batch := expiredFlowIDs[i:end]
				placeholders := strings.Repeat("?,", len(batch)-1) + "?"
				args := make([]any, len(batch))
				for j, id := range batch {
					args[j] = id
				}
				_, err := db.ExecContext(ctx,
					"DELETE FROM microbus_steps WHERE flow_id IN ("+placeholders+")",
					args...,
				)
				if err != nil {
					return errors.Trace(err)
				}
			}

			// Step 3: Delete the flow rows in batches
			for i := 0; i < len(expiredFlowIDs); i += batchSize {
				end := min(i+batchSize, len(expiredFlowIDs))
				batch := expiredFlowIDs[i:end]
				placeholders := strings.Repeat("?,", len(batch)-1) + "?"
				args := make([]any, len(batch))
				for j, id := range batch {
					args[j] = id
				}
				_, err := db.ExecContext(ctx,
					"DELETE FROM microbus_flows WHERE flow_id IN ("+placeholders+")",
					args...,
				)
				if err != nil {
					return errors.Trace(err)
				}
			}

			svc.LogDebug(ctx, "Purged expired flows", "shard", shardIdx, "count", len(expiredFlowIDs))
			return nil
		}
	}
	jobs := make([]func() error, svc.numDBShards())
	for i := range jobs {
		jobs[i] = purgeShard(i)
	}
	err = svc.Parallel(jobs...)
	return errors.Trace(err)
}

// stripProto removes the protocol from the URL.
func stripProto(u string) string {
	left, right, cut := strings.Cut(u, "://")
	if !cut {
		return left
	}
	return right
}

/*
Await blocks until the flow stops (i.e. is no longer created, pending, or running),
then returns the status and snapshot. Returns empty status and nil snapshot on timeout.
*/
func (svc *Service) Await(ctx context.Context, flowKey string) (status string, state map[string]any, err error) { // MARKER: Await
	stopped := func(s string) bool {
		return s != "" && s != foremanapi.StatusCreated && s != foremanapi.StatusPending && s != foremanapi.StatusRunning
	}

	// Register a waiter channel before checking current status
	// to avoid a race where the status changes between the check and the registration.
	ch := make(chan string, 1)
	svc.waitersLock.Lock()
	if svc.waiters == nil {
		svc.waiters = make(map[string][]chan string)
	}
	svc.waiters[flowKey] = append(svc.waiters[flowKey], ch)
	svc.waitersLock.Unlock()

	defer func() {
		svc.waitersLock.Lock()
		chans := svc.waiters[flowKey]
		for i, c := range chans {
			if c == ch {
				svc.waiters[flowKey] = append(chans[:i], chans[i+1:]...)
				break
			}
		}
		if len(svc.waiters[flowKey]) == 0 {
			delete(svc.waiters, flowKey)
		}
		svc.waitersLock.Unlock()
	}()

	// Wait for notification or context cancellation.
	// Non-terminal notifications (e.g. running) are ignored; only terminal statuses exit the loop.
	for {
		status, state, err = svc.Snapshot(ctx, flowKey)
		if err != nil {
			return "", nil, errors.Trace(err)
		}
		if stopped(status) {
			return status, state, nil
		}
		select {
		case <-ch:
			continue
		case <-ctx.Done():
			return "", nil, nil
		}
	}
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal status and state.
*/
func (svc *Service) Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) { // MARKER: Run
	flowKey, err := svc.Create(ctx, workflowName, initialState)
	if err != nil {
		return "", nil, errors.Trace(err)
	}
	err = svc.Start(ctx, flowKey)
	if err != nil {
		svc.Cancel(ctx, flowKey) // Best-effort cleanup
		return "", nil, errors.Trace(err)
	}
	status, state, err = svc.Await(ctx, flowKey)
	if err != nil {
		svc.Cancel(ctx, flowKey) // Best-effort cleanup
		return "", nil, errors.Trace(err)
	}
	return status, state, nil
}

/*
NotifyStatusChange is an internal multicast signal that wakes up all Await callers waiting on the given flow.
*/
func (svc *Service) NotifyStatusChange(ctx context.Context, flowKey string, status string) (err error) { // MARKER: NotifyStatusChange
	svc.waitersLock.Lock()
	chans := svc.waiters[flowKey]
	// Copy the slice to avoid holding the lock while sending
	waiting := make([]chan string, len(chans))
	copy(waiting, chans)
	svc.waitersLock.Unlock()

	for _, ch := range waiting {
		select {
		case ch <- status:
		default:
		}
	}
	return nil
}

// parseFlowKey extracts the shard, numeric flow ID and flow token from a composite flow ID string.
// Format: "{shard}-{flowID}-{token}"
func parseFlowKey(flowKey string) (shardNum int, flowID int, flowToken string, err error) {
	parts := strings.SplitN(flowKey, "-", 3)
	if len(parts) != 3 {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	shardNum64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	flowID64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	return int(shardNum64), int(flowID64), parts[2], nil
}

// parseStepKey extracts the shard, numeric step ID and step token from a composite step key string.
// Format: "{shard}-{stepID}-{token}"
func parseStepKey(stepKey string) (shardNum int, stepID int, stepToken string, err error) {
	parts := strings.SplitN(stepKey, "-", 3)
	if len(parts) != 3 {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	shardNum64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	stepID64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	return int(shardNum64), int(stepID64), parts[2], nil
}
