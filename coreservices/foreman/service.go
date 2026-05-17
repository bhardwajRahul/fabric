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
	"math"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

	"go.opentelemetry.io/otel/trace"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

var (
	_ foremanapi.Client
)

const (
	sequenceName = "foreman@2026-03-10" // Do not change

	maxPollInterval = 5 * time.Minute
	leaseMargin     = 30 * time.Second // margin on top of time_budget for lease duration
	// backlogPollInterval caps the wake timer while due pending steps exist but
	// are undispatched on this replica (e.g. an idle replica that received no
	// doorbell). It restores the old poll-driven liveness net without enumerating
	// the backlog onto a queue. Workers drive refilling directly when busy.
	backlogPollInterval = 2 * time.Second
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

	// Candidate cache and worker pool. rootCtx is the foreman's own lifetime
	// context for all worker/timer/refiller database operations - it must not be
	// tied to OnStartup's ctx (request/startup-scoped) nor to svc.Lifetime()
	// (still context.Background() during OnStartup, per connector/CLAUDE.md). It
	// is cancelled in OnShutdown only after the pools have drained.
	cache      candidateCache
	workers    sync.WaitGroup
	rootCtx    context.Context
	rootCancel context.CancelFunc

	// Single-slot refiller. refillTrigger is buffered(1) and never closed, so a
	// non-blocking send from any goroutine at any time (including the shutdown
	// drain window) coalesces into at most one pending refill - this is the
	// single-slot selection gate. refillStop terminates the refiller goroutine.
	refillTrigger    chan struct{}
	refillStop       chan struct{}
	refiller         sync.WaitGroup
	lastDistinctKeys atomic.Int64 // distinct fairness keys seen in the most recent refill

	// Timer goroutine for polling delayed steps. timerWorker is a WaitGroup
	// separate from the worker pool so OnShutdown can drain the workers (the only
	// callers of shortenNextPoll) before closing wakeTimer.
	nextPoll     time.Time
	nextPollLock sync.Mutex
	wakeTimer    chan struct{}
	timerWorker  sync.WaitGroup

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

	// Initialize the candidate cache and start workers.
	// Workers use raw goroutines (not svc.Go) so they don't count as pending operations during the drain phase of shutdown.
	// Their lifecycle is managed by svc.workers.Wait() in OnShutdown, which runs after cache.close() wakes them up.
	svc.cache.init(svc.Workers())
	svc.refillTrigger = make(chan struct{}, 1)
	svc.refillStop = make(chan struct{})
	svc.wakeTimer = make(chan struct{}, 1)
	svc.nextPoll = time.Now().Add(5 * time.Minute)
	// Workers and the timer loop run for the service lifetime and own all the
	// foreman's database operations. They use rootCtx, the foreman's own root
	// context, NOT OnStartup's ctx (request/startup-scoped, could be cancelled
	// mid-flow and strand a fan-in/step write) and NOT svc.Lifetime() (still
	// context.Background() during OnStartup). rootCtx is cancelled in OnShutdown
	// only after the pool has drained, so in-flight DB writes are never aborted.
	svc.rootCtx, svc.rootCancel = context.WithCancel(context.Background())
	numWorkers := svc.Workers()
	for range numWorkers {
		svc.workers.Add(1)
		go func() {
			defer svc.workers.Done()
			svc.workerLoop(svc.rootCtx)
		}()
	}
	// Timer goroutine for polling delayed steps. Tracked separately from the
	// worker pool so OnShutdown drains workers before closing wakeTimer.
	svc.timerWorker.Add(1)
	go func() {
		defer svc.timerWorker.Done()
		svc.timerLoop(svc.rootCtx)
	}()
	// Single refiller goroutine. Coalesced trigger sends make it the single-slot
	// selection gate. Stopped after workers and timer have drained so no DB op
	// outlives rootCtx and no caller of requestRefill remains.
	svc.refiller.Add(1)
	go func() {
		defer svc.refiller.Done()
		svc.refillerLoop(svc.rootCtx)
	}()
	svc.requestRefill() // pick up any steps left pending by a prior replica
	return nil
}

// requestRefill asks the refiller to run a selection scan. It is a non-blocking
// send on a buffered(1), never-closed channel, so concurrent callers (workers at
// the low-water mark, the timer poll, the Enqueue doorbell) coalesce into at
// most one pending refill and the send never panics, even during the shutdown
// drain window.
func (svc *Service) requestRefill() {
	select {
	case svc.refillTrigger <- struct{}{}:
	default:
	}
}

// refillerLoop runs one selection scan per trigger. A single goroutine plus the
// coalesced trigger guarantee only one scan is ever in flight per replica.
func (svc *Service) refillerLoop(ctx context.Context) {
	for {
		select {
		case <-svc.refillStop:
			return
		case <-svc.refillTrigger:
			err := errors.CatchPanic(func() error {
				return svc.runRefill(ctx)
			})
			if err != nil {
				svc.LogError(ctx, "Refilling candidate cache", "error", err)
			}
		}
	}
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	svc.cache.close() // Terminate workerLoop (unblocks blocked pops independently of any channel)
	// Drain the worker pool before closing wakeTimer. Workers are the only callers
	// of shortenNextPoll, which sends on wakeTimer; a send on a closed channel
	// panics even inside a select with a default. cache.close() unblocks workers
	// independently of wakeTimer, so this cannot deadlock.
	svc.workers.Wait()
	if svc.wakeTimer != nil {
		close(svc.wakeTimer) // Terminate timerLoop (no shortenNextPoll callers remain)
	}
	svc.timerWorker.Wait()
	// Workers and timer have drained, so no requestRefill caller remains. Stop the
	// refiller and drain it before cancelling rootCtx. refillTrigger is never
	// closed, so any late coalesced send (e.g. from the timer's last poll) is a
	// harmless no-op rather than a panic; the refiller exits on refillStop and a
	// refill into the now-closed cache is a no-op.
	if svc.refillStop != nil {
		close(svc.refillStop)
	}
	svc.refiller.Wait()
	// All worker/timer/refiller DB writes are complete. Safe to cancel.
	if svc.rootCancel != nil {
		svc.rootCancel()
	}
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

// OnObserveQueueDepth records the current depth of the local candidate cache.
func (svc *Service) OnObserveQueueDepth(ctx context.Context) (err error) { // MARKER: QueueDepth
	err = svc.RecordQueueDepth(ctx, svc.cache.len())
	return errors.Trace(err)
}

// OnObservePendingStepsByPriority records the backlog depth per priority band - the primary
// "starvation forming" signal given there is no aging.
func (svc *Service) OnObservePendingStepsByPriority(ctx context.Context) (err error) { // MARKER: PendingStepsByPriority
	byPriority := map[int]int{}
	for s := 0; s < svc.numDBShards(); s++ {
		db, derr := svc.shard(s)
		if derr != nil {
			return errors.Trace(derr)
		}
		rows, derr := db.QueryContext(ctx,
			"SELECT priority, COUNT(*) FROM microbus_steps"+
				" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() GROUP BY priority",
			foremanapi.StatusPending,
		)
		if derr != nil {
			return errors.Trace(derr)
		}
		for rows.Next() {
			var priority, count int
			if derr := rows.Scan(&priority, &count); derr != nil {
				rows.Close()
				return errors.Trace(derr)
			}
			byPriority[priority] += count
		}
		rows.Close()
		if derr := rows.Err(); derr != nil {
			return errors.Trace(derr)
		}
	}
	for priority, count := range byPriority {
		err = svc.RecordPendingStepsByPriority(ctx, count, strconv.Itoa(priority))
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// OnObserveOldestPendingAgeSeconds records the age of the oldest due pending step per band -
// the direct visible starvation watch.
func (svc *Service) OnObserveOldestPendingAgeSeconds(ctx context.Context) (err error) { // MARKER: OldestPendingAgeSeconds
	oldest := map[int]int{} // priority -> max age seconds across shards
	for s := 0; s < svc.numDBShards(); s++ {
		db, derr := svc.shard(s)
		if derr != nil {
			return errors.Trace(derr)
		}
		rows, derr := db.QueryContext(ctx,
			"SELECT priority, DATE_DIFF_MILLIS(NOW_UTC(), MIN(created_at)) FROM microbus_steps"+
				" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() GROUP BY priority",
			foremanapi.StatusPending,
		)
		if derr != nil {
			return errors.Trace(derr)
		}
		for rows.Next() {
			var priority int
			var ageMs sql.NullFloat64
			if derr := rows.Scan(&priority, &ageMs); derr != nil {
				rows.Close()
				return errors.Trace(derr)
			}
			if ageMs.Valid {
				if sec := int(ageMs.Float64 / 1000); sec > oldest[priority] {
					oldest[priority] = sec
				}
			}
		}
		rows.Close()
		if derr := rows.Err(); derr != nil {
			return errors.Trace(derr)
		}
	}
	for priority, sec := range oldest {
		err = svc.RecordOldestPendingAgeSeconds(ctx, sec, strconv.Itoa(priority))
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// OnObserveDistinctFairnessKeys records the distinct fairness key count from the most recent
// refill - a near-free byproduct of the refiller's candidate key set, low cardinality.
func (svc *Service) OnObserveDistinctFairnessKeys(ctx context.Context) (err error) { // MARKER: DistinctFairnessKeys
	err = svc.RecordDistinctFairnessKeys(ctx, int(svc.lastDistinctKeys.Load()))
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
func (svc *Service) Create(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (flowKey string, err error) { // MARKER: Create
	if workflowName == "" {
		return "", errors.New("workflow name is required", http.StatusBadRequest)
	}
	graph, err := svc.fetchGraph(ctx, workflowName)
	if err != nil {
		return "", errors.Trace(err)
	}
	shardNum := rand.IntN(svc.numDBShards())
	flowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, graph, initialState, 0, "", svc.resolveFlowOptions(ctx, opts))
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
	var inherited flowSchedule
	err = db.QueryRowContext(ctx,
		"SELECT flow_id, status, final_state, graph, workflow_name, priority, fairness_key, fairness_weight FROM microbus_flows WHERE thread_id=? ORDER BY flow_id DESC LIMIT_OFFSET(1, 0)",
		threadID,
	).Scan(&latestFlowID, &flowStatus, &finalStateJSON, &graphJSON, &workflowName, &inherited.priority, &inherited.fairnessKey, &inherited.fairnessWeight)
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

	// Create a new flow with the same graph and merged state, in the same thread.
	// Priority and fairness are inherited from the latest flow in the thread.
	newFlowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, &graph, mergedState, threadID, threadToken, inherited)
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
	flowKey, err = svc.createWithGraph(ctx, shardNum, taskName, graph, initialState, 0, "", svc.resolveFlowOptions(ctx, nil))
	return flowKey, errors.Trace(err)
}

// createWithGraph is the shared implementation for Create, CreateTask, and Continue.
// It creates a new flow from a pre-built graph in "created" status without starting it.
// If threadID is 0, the new flow starts its own thread (thread_id = flow_id).
// If threadID is non-zero, the new flow joins the specified thread.
func (svc *Service) createWithGraph(ctx context.Context, shardNum int, workflowName string, graph *workflow.Graph, initialState any, threadID int, threadToken string, sched flowSchedule) (flowKey string, err error) {
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
		"INSERT INTO microbus_flows (flow_token, workflow_name, graph, actor_claims, status, forked_flow_id, forked_step_depth, trace_parent, priority, fairness_key, fairness_weight)"+
			" VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?)",
		flowToken, workflowName, string(graphJSON), string(actorClaimsJSON), foremanapi.StatusCreated, traceParent, sched.priority, sched.fairnessKey, sched.fairnessWeight,
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

	timeBudget := svc.taskTimeBudget()
	newStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lease_expires, priority, fairness_key, fairness_weight)"+
			" VALUES (?, 1, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), ?, ?, ?)",
		newFlowID, utils.RandomIdentifier(16), entryPoint, string(stateJSON), foremanapi.StatusCreated, timeBudget.Milliseconds(), leaseMargin.Milliseconds(), sched.priority, sched.fairnessKey, sched.fairnessWeight,
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

	// Ring the work doorbell for the leaf step (fire-and-forget, consistent with
	// Start and fan-in; the backlog poll recovers it if the ring is missed).
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, leafStepID.(int))
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
	var parentSched flowSchedule
	err = db.QueryRowContext(ctx,
		"SELECT workflow_name, graph, actor_claims, trace_parent, breakpoints, priority, fairness_key, fairness_weight FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&workflowName, &graphJSON, &actorClaimsJSON, &traceParent, &parentBreakpointsJSON, &parentSched.priority, &parentSched.fairnessKey, &parentSched.fairnessWeight)
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
		"INSERT INTO microbus_flows (flow_token, workflow_name, graph, actor_claims, status, forked_flow_id, forked_step_depth, trace_parent, breakpoints, priority, fairness_key, fairness_weight)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		newFlowToken, workflowName, graphJSON, actorClaimsJSON, foremanapi.StatusCreated, flowID, stepDepth, traceParent, parentBreakpointsJSON, parentSched.priority, parentSched.fairnessKey, parentSched.fairnessWeight,
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	forkTimeBudget := svc.taskTimeBudget()
	newStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lease_expires, priority, fairness_key, fairness_weight)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), ?, ?, ?)",
		newFlowID, stepDepth, utils.RandomIdentifier(16), taskName, string(mergedStateJSON), foremanapi.StatusCreated, forkTimeBudget.Milliseconds(), leaseMargin.Milliseconds(), parentSched.priority, parentSched.fairnessKey, parentSched.fairnessWeight,
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
		stepID         int
		stepDepth      int
		taskName       string
		state          string
		timeBudgetMs   int
		lineageID      int
		fanOutOrdinal  int
		predecessorID  int
		priority       int
		fairnessKey    string
		fairnessWeight float64
	}
	var failedSteps []failedStep
	if fanOut {
		// Potentially multiple failed steps
		rows, err := db.QueryContext(ctx,
			"SELECT step_id, step_depth, task_name, state, time_budget_ms, lineage_id, fan_out_ordinal, predecessor_id, priority, fairness_key, fairness_weight FROM microbus_steps WHERE flow_id=? AND status=?",
			flowID, foremanapi.StatusFailed,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		for rows.Next() {
			var fs failedStep
			err := rows.Scan(&fs.stepID, &fs.stepDepth, &fs.taskName, &fs.state, &fs.timeBudgetMs, &fs.lineageID, &fs.fanOutOrdinal, &fs.predecessorID, &fs.priority, &fs.fairnessKey, &fs.fairnessWeight)
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
			"SELECT step_id, step_depth, task_name, state, time_budget_ms, lineage_id, fan_out_ordinal, predecessor_id, priority, fairness_key, fairness_weight FROM microbus_steps WHERE step_id=? AND status=?",
			flowStepID, foremanapi.StatusFailed,
		).Scan(&fs.stepID, &fs.stepDepth, &fs.taskName, &fs.state, &fs.timeBudgetMs, &fs.lineageID, &fs.fanOutOrdinal, &fs.predecessorID, &fs.priority, &fs.fairnessKey, &fs.fairnessWeight)
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
		// Carry lineage_id and fan_out_ordinal so the retried step rejoins its fan-in
		// cohort in its original position; without lineage_id it would orphan from the
		// cohort. Carry predecessor_id so it slots back into the execution DAG where
		// the failed step was.
		newStepID, err := tx.InsertReturnID(ctx, "step_id",
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, lineage_id, fan_out_ordinal, predecessor_id, priority, fairness_key, fairness_weight)"+
				" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			flowID, fs.stepDepth, utils.RandomIdentifier(16), fs.taskName, fs.state, foremanapi.StatusPending, fs.timeBudgetMs, fs.lineageID, fs.fanOutOrdinal, fs.predecessorID, fs.priority, fs.fairnessKey, fs.fairnessWeight,
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
Set CursorFlowKey in the query to the last result's flow key to paginate. Limit defaults to 100 if not set.
*/
func (svc *Service) List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error) { // MARKER: List
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	var cursorNum int
	if query.CursorFlowKey != "" {
		_, cursorNum, _, err = parseFlowKey(query.CursorFlowKey)
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
				summary.FlowKey = fmt.Sprintf("%d-%d-%s", shardIdx, flowID, strings.TrimSpace(flowToken))
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
		return flows[i].FlowKey > flows[j].FlowKey
	})
	if len(flows) > limit {
		flows = flows[:limit]
	}
	return flows, nil
}

/*
Enqueue rings the work doorbell, signalling that a step is pending. This endpoint is called by foreman replicas to
distribute work across the cluster. It does not enqueue a specific step: the receiving replica decides, from the
step's priority relative to its candidate cache, whether to refill or to flush and refill.
*/
func (svc *Service) Enqueue(ctx context.Context, shard int, stepID int) (err error) { // MARKER: Enqueue
	if frame.Of(ctx).FromHost() != Hostname {
		return errors.New("enqueue is restricted to foreman replicas", http.StatusForbidden)
	}
	// Resolve the announced step's priority via a PK lookup (off the selection
	// hot path). A miss (step already consumed) yields MaxInt, so the doorbell
	// only wakes an idle replica and never blanket-requeries a busy one.
	priority := math.MaxInt
	if db, derr := svc.shard(shard); derr == nil {
		db.QueryRowContext(ctx, "SELECT priority FROM microbus_steps WHERE step_id=?", stepID).Scan(&priority)
	}
	ring := svc.cache.offer(job{stepID: stepID, shard: shard}, priority)
	svc.LogDebug(ctx, "Doorbell", "stepID", stepID, "priority", priority, "ring", ring)
	if ring {
		svc.requestRefill()
	}
	return nil
}

// workerLoop is the main loop for a worker goroutine. It pops a candidate from
// the cache and executes one task per iteration. Acquisition is the atomic CAS
// inside processStep, so a stale or duplicated candidate is harmless: the loser
// returns nil and the worker pops the next one. Draining the cache to the
// low-water mark asks the refiller to top it up.
func (svc *Service) workerLoop(ctx context.Context) {
	for {
		waitStart := time.Now()
		j, ok, needRefill := svc.cache.pop()
		if needRefill {
			svc.requestRefill()
		}
		if !ok {
			return // Cache closed
		}
		svc.RecordClaimWaitSeconds(ctx, time.Since(waitStart).Seconds())
		svc.LogDebug(ctx, "Worker popped", "stepID", j.stepID, "shard", j.shard, "needRefill", needRefill)
		err := errors.CatchPanic(func() error {
			return svc.processStep(ctx, j.stepID, j.shard)
		})
		if err != nil {
			svc.LogError(ctx, "Failed to process step",
				"stepID", j.stepID,
				"error", err,
			)
		}
		// Request a refill after the step has been acquired/completed (it is no
		// longer pending), so the refiller's next scan reflects the freed slot
		// and never re-selects this in-flight step. This is the liveness
		// guarantee: every completion drives one fresh post-completion scan, so
		// a single-slot coalesced trigger can never wedge with work remaining.
		svc.requestRefill()
	}
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
			return "", nil, errors.Trace(ctx.Err(), http.StatusRequestTimeout)
		}
	}
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal status and state.
*/
func (svc *Service) Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (status string, state map[string]any, err error) { // MARKER: Run
	flowKey, err := svc.Create(ctx, workflowName, initialState, opts)
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
