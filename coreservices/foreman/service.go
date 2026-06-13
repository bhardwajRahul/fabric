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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	"github.com/microbus-io/throttle"

	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/lru"
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
	// backlogPollInterval is the safety-net cadence for picking up due pending work that no other
	// wake mechanism covered. The primary wake paths are: the completion doorbell (workers and
	// Enqueue), per-task valve-window alignment (runRefill's shortenNextPoll on a valve drop),
	// per-task breaker probe schedule (breakerTrip's shortenNextPoll on nextProbeAt), and
	// per-step not_before timing (sized by pollPendingSteps from MIN(not_before)). What remains
	// is orphan recovery (crashed worker; bounded above by lease expiry of time_budget+leaseMargin,
	// ~2.5min) and any state we missed. One minute is a comfortable cap for those.
	backlogPollInterval = 1 * time.Minute
)

// parked encodes why a step is excluded from the selection band. Steps with parked != 0 are
// physically excluded from idx_microbus_steps_selection and idx_microbus_steps_saturation, so
// the refill scan, saturation count, and lease-expiry recovery never see them. Two reasons today:
//   - parkedSubgraph: surgraph parent waiting for its child flow to resolve. Status stays
//     'running' (the step IS running, logically), no lease is consulted, and the step returns to
//     parked=0 + status='pending' when completeSurgraphFlow re-dispatches it.
//   - parkedBreaker: pending step held back because the breaker for its task name is tripped.
//     The breaker leaves the oldest one unparked as the probe; the rest sit at parked=2 until
//     the probe succeeds and the breaker closes (bulk-unpark).
// Parked discriminator values for microbus_steps.parked. The DB column is SMALLINT so the
// index leaf entries are 2 bytes each; the Go-side type stays untyped so call sites bind these
// directly into queries without `int16(x)` casts at every use.
const (
	parkedNone     = 0
	parkedSubgraph = 1
	parkedBreaker  = 2
)

// initialParkedFor returns the parked value a new pending step for taskName should be inserted
// with, given the local breaker state. When the local breaker is tripped, the new step is born
// parked=2 so the selection scan never sees it - avoiding the wasted dispatch attempt + 404 +
// gossip + bulk-park cycle that would otherwise self-correct one step at a time. When the
// breaker is closed (the common case), parked=0 and the step is immediately selectable.
func (svc *Service) initialParkedFor(taskName string) int {
	if svc.BreakerTripped(taskName) {
		return parkedBreaker
	}
	return parkedNone
}

// parkTrippedSteps scans the pending steps of flowID, finds those whose task has a locally-tripped
// breaker, and parks them (parked=2) within the given transaction. Cheap shortcut when no
// breakers are tripped on this replica - skips the SELECT entirely. Used by Start (CREATED ->
// PENDING transition) and Retry (which inserts a fresh PENDING row). One additional UPDATE per
// affected task; off the hot path.
func (svc *Service) parkTrippedSteps(ctx context.Context, tx sequel.Executor, flowID int) error {
	svc.breakersLock.RLock()
	if len(svc.breakers) == 0 {
		svc.breakersLock.RUnlock()
		return nil
	}
	trippedTasks := make(map[string]struct{}, len(svc.breakers))
	for name, b := range svc.breakers {
		if !b.trippedAt.IsZero() {
			trippedTasks[name] = struct{}{}
		}
	}
	svc.breakersLock.RUnlock()
	if len(trippedTasks) == 0 {
		return nil
	}
	// Pull only the task names that actually appear in this flow's pending set so we
	// avoid running a per-task UPDATE for tasks that aren't in this graph.
	rows, err := tx.QueryContext(ctx,
		"SELECT DISTINCT task_name FROM microbus_steps WHERE flow_id=? AND status=? AND parked=?",
		flowID, workflow.StatusPending, parkedNone,
	)
	if err != nil {
		return errors.Trace(err)
	}
	defer rows.Close()
	affectedTasks := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return errors.Trace(err)
		}
		if _, ok := trippedTasks[name]; ok {
			affectedTasks = append(affectedTasks, name)
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Trace(err)
	}
	for _, name := range affectedTasks {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET parked=?, updated_at=NOW_UTC() WHERE flow_id=? AND task_name=? AND status=? AND parked=?",
			parkedBreaker, flowID, name, workflow.StatusPending, parkedNone,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

/*
Service implements the foreman.core microservice.

The foreman orchestrates agentic workflow execution. It fetches workflow graph definitions, executes tasks sequentially,
evaluates transition conditions, and persists state after each step.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	dbs     []*sequel.DB
	dbsLock sync.RWMutex

	// Candidate cache and worker pool. Workers, the timer goroutine, and the refiller
	// all use svc.Lifetime() as their root context. OnShutdown drains them in strict
	// order before the connector cancels the lifetime ctx, so in-flight DB writes are
	// never aborted mid-operation.
	cache   candidateCache
	workers sync.WaitGroup

	// Single-slot refiller. refillTrigger is buffered(1) and never closed, so a
	// non-blocking send from any goroutine at any time (including the shutdown
	// drain window) coalesces into at most one pending refill - this is the
	// single-slot selection gate. refillStop terminates the refiller goroutine.
	refillTrigger      chan struct{}
	refillStop         chan struct{}
	refiller           sync.WaitGroup
	lastRefillPriority int // most recent band the refiller selected; 0 when none

	// Timer goroutine for polling delayed steps. timerWorker is a WaitGroup separate from the worker
	// pool so OnShutdown can stage the drain.
	//
	// The timer wakes on whichever of two independent deadlines is sooner:
	//   - nextPoll: the work cadence (nearest not_before, valve window, backlog cap). Reset by
	//     pollPendingSteps every cycle; shortened by shortenNextPoll.
	//   - nextProbe: the soonest tripped-breaker probe, owned solely by the breaker subsystem
	//     (UnixNano; 0 = none). Kept separate so pollPendingSteps' nextPoll reset can never clobber
	//     a pending probe - the bug that let the breaker's 100ms..1m schedule collapse to the 1m
	//     backlog cadence.
	//
	// wakeTimer is the nudge channel: signalTimer sends on it non-blockingly. It is buffered(1) and
	// NEVER closed, so a send can never panic regardless of which goroutine nudges (workers, the
	// breaker subsystem via refreshNextProbeLocked, the Enqueue/TripBreaker RPC handlers) or how it
	// races shutdown - the same rationale as refillTrigger. The timer is stopped by closing the
	// dedicated timerStop.
	nextPoll     time.Time
	nextPollLock sync.Mutex
	nextProbe    atomic.Int64
	wakeTimer    chan struct{}
	timerStop    chan struct{}
	timerWorker  sync.WaitGroup

	// Wait registry for Await (keyed by flowKey)
	waitersLock sync.Mutex
	waiters     map[string][]chan string // flowKey -> list of waiting channels

	// Adaptive per-task dispatch rate state. See backpressure.go and the
	// "Adaptive Per-Task Concurrency" section of CLAUDE.md.
	valves     map[string]*taskValve
	valvesLock sync.RWMutex

	// Per-task 404 ack-timeout breaker. See breaker.go.
	breakers     map[string]*taskBreaker
	breakersLock sync.RWMutex

	// Per-flow parsed-graph cache. The graphJSON column is frozen at flow
	// creation, so every step of the same flow re-unmarshals identical bytes;
	// this LRU eliminates that redundant work. Keyed by (shard, flowID); see
	// graphCacheKey. Bounded to 256 entries with a 15-minute TTL.
	graphCache *lru.Cache[graphCacheKey, *workflow.Graph]
}

// graphCacheKey scopes the per-flow graph LRU by shard, since flow_id is only
// unique within a shard.
type graphCacheKey struct {
	shard  int
	flowID int
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
	svc.timerStop = make(chan struct{})
	svc.nextPoll = time.Now() // recover any lease-expired steps left running by a prior replica
	svc.valves = map[string]*taskValve{}
	svc.breakers = map[string]*taskBreaker{}
	// Reconstitute breakers from the DB before workers start. A previous replica may have left
	// parked=parkedBreaker rows that are invisible to the selection index; without re-arming the
	// in-memory state, this replica wouldn't know a probe schedule is due.
	if err := svc.reconstituteBreakers(ctx); err != nil {
		return errors.Trace(err)
	}
	svc.graphCache = lru.New[graphCacheKey, *workflow.Graph](256, 15*time.Minute)
	// Workers, timer, and refiller share svc.Lifetime() as their root context. OnShutdown
	// drains them in strict order before the connector cancels the lifetime ctx, so in-flight
	// DB writes never observe cancellation. Workers use raw goroutines (not svc.Go) so they
	// don't count as pending operations during the connector's drain phase; their lifecycle
	// is managed by svc.workers.Wait() in OnShutdown, which runs after cache.close() wakes
	// them up.
	lifetimeCtx := svc.Lifetime()
	numWorkers := svc.Workers()
	for range numWorkers {
		svc.workers.Add(1)
		go func() {
			defer svc.workers.Done()
			svc.workerLoop(lifetimeCtx)
		}()
	}
	// Timer goroutine for polling delayed steps. Tracked separately from the
	// worker pool so OnShutdown drains workers before closing wakeTimer.
	svc.timerWorker.Add(1)
	go func() {
		defer svc.timerWorker.Done()
		svc.timerLoop(lifetimeCtx)
	}()
	// Single refiller goroutine. Coalesced trigger sends make it the single-slot
	// selection gate. Stopped after workers and timer have drained so no caller of
	// requestRefill remains.
	svc.refiller.Add(1)
	go func() {
		defer svc.refiller.Done()
		svc.refillerLoop(lifetimeCtx)
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
	svc.workers.Wait()
	// Stop the timer via its dedicated timerStop; wakeTimer is never closed. signalTimer (via
	// shortenNextPoll / refreshNextProbeLocked) sends on wakeTimer from workers, the breaker
	// subsystem, and the Enqueue/TripBreaker RPC handlers - a never-closed buffered channel
	// makes every such send safe regardless of drain ordering, the same rationale as refillTrigger.
	if svc.timerStop != nil {
		close(svc.timerStop) // Terminate timerLoop
	}
	svc.timerWorker.Wait()
	// Workers and timer have drained, so no requestRefill caller remains. Stop the
	// refiller and drain it. refillTrigger is never closed, so any late coalesced
	// send (e.g. from the timer's last poll) is a harmless no-op rather than a panic;
	// the refiller exits on refillStop and a refill into the now-closed cache is a no-op.
	if svc.refillStop != nil {
		close(svc.refillStop)
	}
	svc.refiller.Wait()
	// All worker/timer/refiller DB writes are complete. The connector cancels the
	// lifetime ctx after OnShutdown returns, by which point nothing observes it.
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
	// Zero out the last-selected band's distinct-fairness-keys series so the
	// dashboard doesn't show a stale count from a foreman that is now gone.
	if svc.lastRefillPriority != 0 {
		_ = svc.RecordStepsFairnessKeys(ctx, 0, strconv.Itoa(svc.lastRefillPriority))
		svc.lastRefillPriority = 0
	}
	svc.closeDatabase(ctx)
	return nil
}

// OnObserveStepsQueueDepth records the current depth of the local candidate cache.
func (svc *Service) OnObserveStepsQueueDepth(ctx context.Context) (err error) { // MARKER: StepsQueueDepth
	err = svc.RecordStepsQueueDepth(ctx, svc.cache.len())
	return errors.Trace(err)
}

// OnObserveStepsPending records the backlog depth per priority band - the primary
// "starvation forming" signal given there is no aging. Any shard's error fails the whole
// observe call; the gauge stays at its last-recorded value until the next successful scrape.
func (svc *Service) OnObserveStepsPending(ctx context.Context) (err error) { // MARKER: StepsPending
	numShards := svc.numDBShards()
	perShard := make([]map[int]int, numShards+1)
	err = svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		rows, err := db.QueryContext(ctx,
			"SELECT priority, COUNT(*) FROM microbus_steps"+
				" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() GROUP BY priority",
			workflow.StatusPending,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		m := map[int]int{}
		for rows.Next() {
			var priority, count int
			err := rows.Scan(&priority, &count)
			if err != nil {
				return errors.Trace(err)
			}
			m[priority] = count
		}
		err = rows.Err()
		if err != nil {
			return errors.Trace(err)
		}
		perShard[shard] = m
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	byPriority := map[int]int{}
	for i := 1; i <= numShards; i++ {
		for priority, count := range perShard[i] {
			byPriority[priority] += count
		}
	}
	for priority, count := range byPriority {
		err = svc.RecordStepsPending(ctx, count, strconv.Itoa(priority))
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// OnObserveStepsOldestPendingAgeSeconds records the age of the oldest due pending step per band -
// the direct visible starvation watch. Any shard's error fails the whole observe call; the
// gauge stays at its last-recorded value until the next successful scrape.
func (svc *Service) OnObserveStepsOldestPendingAgeSeconds(ctx context.Context) (err error) { // MARKER: StepsOldestPendingAgeSeconds
	numShards := svc.numDBShards()
	perShard := make([]map[int]int, numShards+1)
	err = svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		rows, err := db.QueryContext(ctx,
			"SELECT priority, DATE_DIFF_MILLIS(NOW_UTC(), MIN(created_at)) FROM microbus_steps"+
				" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() GROUP BY priority",
			workflow.StatusPending,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		m := map[int]int{}
		for rows.Next() {
			var priority int
			var ageMs sql.NullFloat64
			err := rows.Scan(&priority, &ageMs)
			if err != nil {
				return errors.Trace(err)
			}
			if ageMs.Valid {
				m[priority] = int(ageMs.Float64 / 1000)
			}
		}
		err = rows.Err()
		if err != nil {
			return errors.Trace(err)
		}
		perShard[shard] = m
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}
	oldest := map[int]int{} // priority -> max age seconds
	for i := 1; i <= numShards; i++ {
		for priority, sec := range perShard[i] {
			if sec > oldest[priority] {
				oldest[priority] = sec
			}
		}
	}
	for priority, sec := range oldest {
		err = svc.RecordStepsOldestPendingAgeSeconds(ctx, sec, strconv.Itoa(priority))
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
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

	mmd, err := foremanapi.NewFlowRenderer(steps).WithLinks("step").Render()
	if err != nil {
		return errors.Trace(err)
	}

	if r.URL.Query().Get("format") == "raw" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, mmd)
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
</html>`, flowKey, mmd)
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
	// Open new shards using 1-based indices: oldCount+1..newCount.
	for i := oldCount + 1; i <= newCount; i++ {
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
	shardNum := rand.IntN(svc.numDBShards()) + 1 // shards are 1-based
	opts = svc.resolveFlowOptions(ctx, opts)
	flowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, graph, initialState, 0, "", opts)
	return flowKey, errors.Trace(err)
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state using the
graph's reducers. The threadKey can be any flowKey that belongs to the thread (including the original one).
The new flow uses the same workflow graph, belongs to the same thread, and is returned in created status.
It is intended for multi-turn workflows where outputs feed back as inputs.

The new flow's scheduling and lifetime (priority, fairness, deadline) come from caller-supplied opts,
resolved like a fresh Create. A nil opts gets fresh defaults rather than inheriting from the prior flow -
each turn in a thread is a fresh unit of work and the prior turn's deadline is already in the past.
*/
func (svc *Service) Continue(ctx context.Context, threadKey string, additionalState any, opts *workflow.FlowOptions) (newFlowKey string, err error) { // MARKER: Continue
	shardNum, flowID, flowToken, err := parseFlowKey(threadKey)
	if err != nil {
		return "", errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}
	opts = svc.resolveFlowOptions(ctx, opts)

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

	// Find the latest flow in the thread for graph + final_state inheritance only.
	// Scheduling (priority/fairness/deadline) comes from opts, not the prior flow.
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
	if flowStatus != workflow.StatusCompleted {
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
	newFlowKey, err = svc.createWithGraph(ctx, shardNum, workflowName, &graph, mergedState, threadID, threadToken, opts)
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
	graph.AddTransition(taskName, workflow.END)
	shardNum := rand.IntN(svc.numDBShards()) + 1 // shards are 1-based
	flowKey, err = svc.createWithGraph(ctx, shardNum, taskName, graph, initialState, 0, "", svc.resolveFlowOptions(ctx, nil))
	return flowKey, errors.Trace(err)
}

// createWithGraph is the shared implementation for Create, CreateTask, and Continue.
// It creates a new flow from a pre-built graph in "created" status without starting it.
// If threadID is 0, the new flow starts its own thread (thread_id = flow_id).
// If threadID is non-zero, the new flow joins the specified thread.
func (svc *Service) createWithGraph(ctx context.Context, shardNum int, workflowName string, graph *workflow.Graph, initialState any, threadID int, threadToken string, opts *workflow.FlowOptions) (flowKey string, err error) {
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

	// Tenant: read from the caller's frame; 0 (no-tenant sentinel) when absent or unparseable.
	tenantID, _ := frame.Of(ctx).Tenant()

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

	// Atomically create the flow and its first step within a transaction.
	// Wrapped in a deadlock-retry loop because InnoDB next-key locks on the
	// step's secondary indexes (idx_microbus_steps_selection /
	// idx_microbus_steps_saturation) can produce 1213 deadlocks under
	// concurrent flow creation on the same shard.
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", errors.Trace(err)
	}
	timeBudget := svc.taskTimeBudget()
	stepToken := utils.RandomIdentifier(16)
	var newFlowID int64
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			err = svc.Sleep(ctx, time.Duration(attempt)*time.Millisecond)
			if err != nil {
				return "", errors.Trace(err)
			}
		}
		newFlowID, err = svc.createWithGraphTx(ctx, db, flowToken, workflowName, graphJSON, actorClaimsJSON, traceParent, tenantID, threadID, threadToken, entryPoint, stateJSON, stepToken, timeBudget, opts)
		if err == nil {
			break
		}
		if !sequel.IsLockContentionError(err) || attempt == maxAttempts-1 {
			return "", errors.Trace(err)
		}
	}
	svc.LogDebug(ctx, "Flow created", "flow", workflowName, "task", entryPoint)
	return fmt.Sprintf("%d-%d-%s", shardNum, newFlowID, flowToken), nil
}

// createWithGraphTx executes one attempt of the create-flow transaction. Pulled
// out of createWithGraph so the outer function can retry on InnoDB deadlocks
// (sequel.IsLockContentionError) without duplicating the body.
func (svc *Service) createWithGraphTx(ctx context.Context, db *sequel.DB, flowToken, workflowName string, graphJSON, actorClaimsJSON []byte, traceParent string, tenantID, threadID int, threadToken, entryPoint string, stateJSON []byte, stepToken string, timeBudget time.Duration, opts *workflow.FlowOptions) (newFlowID int64, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer tx.Rollback()

	newFlowID, err = tx.InsertReturnID(ctx, "flow_id",
		"INSERT INTO microbus_flows (flow_token, workflow_name, graph, actor_claims, status, trace_parent, priority, fairness_key, fairness_weight, tenant_id)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		flowToken, workflowName, string(graphJSON), string(actorClaimsJSON), workflow.StatusCreated, traceParent, opts.Priority, opts.FairnessKey, opts.FairnessWeight, tenantID,
	)
	if err != nil {
		return 0, errors.Trace(err)
	}

	startDelayMs := int64(0)
	if !opts.StartAt.IsZero() {
		if d := time.Until(opts.StartAt).Milliseconds(); d > 0 {
			startDelayMs = d
		}
	}
	newStepID, err := tx.InsertReturnID(ctx, "step_id",
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, time_budget_ms, not_before, lease_expires, priority, fairness_key, fairness_weight)"+
			" VALUES (?, 1, ?, ?, ?, ?, ?, DATE_ADD_MILLIS(NOW_UTC(), ?), DATE_ADD_MILLIS(NOW_UTC(), ?), ?, ?, ?)",
		newFlowID, stepToken, entryPoint, string(stateJSON), workflow.StatusCreated, timeBudget.Milliseconds(), startDelayMs, leaseMargin.Milliseconds(), opts.Priority, opts.FairnessKey, opts.FairnessWeight,
	)
	if err != nil {
		return 0, errors.Trace(err)
	}

	// Combined post-insert UPDATE: thread_id self-references when no thread was
	// provided (we needed newFlowID to set it), and step_id points at the row
	// just inserted. Done in one round-trip instead of two.
	if threadID == 0 {
		threadID = int(newFlowID)
		threadToken = flowToken
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET thread_id=?, thread_token=?, step_id=?, updated_at=NOW_UTC() WHERE flow_id=?",
		threadID, threadToken, newStepID, newFlowID,
	)
	if err != nil {
		return 0, errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return 0, errors.Trace(err)
	}
	return newFlowID, nil
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
	if flowStatus != workflow.StatusCreated {
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
		workflow.StatusPending, flowID, workflow.StatusCreated,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// If any of the just-transitioned steps belong to a task whose local breaker is currently
	// tripped, park them at insert-into-the-pending-pool time so selection never sees them. One
	// extra UPDATE per tripped task in this flow's graph; bounded by graph fan-out, off the hot
	// path. Skipped entirely when no breakers are tripped (the common case).
	err = svc.parkTrippedSteps(ctx, tx, flowID)
	if err != nil {
		return errors.Trace(err)
	}

	notifyHostname = strings.TrimSpace(notifyHostname)
	// started_at stamps the moment this run actually began dispatching. Mirrors the per-step
	// started_at semantics: distinct from created_at (row insert), reset on Restart/RestartFrom.
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, notify_hostname=?, started_at=NOW_UTC(), updated_at=NOW_UTC() WHERE flow_id=? AND status=?",
		workflow.StatusRunning, notifyHostname, flowID, workflow.StatusCreated,
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
	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "from", workflow.StatusCreated, "to", workflow.StatusRunning)
	svc.IncrementFlowsStarted(ctx, 1, workflowName)
	compositeID := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, compositeID, workflow.StatusRunning)

	// Enqueue the current step for processing (outside the transaction)
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, int(stepID))
	return nil
}

/*
Snapshot returns the current outcome of a flow: status, state, plus the populated side-channel
field (Error / InterruptPayload / CancelReason) for whichever non-running status the flow is in.
For an interrupted flow, State is the merged snapshot at the time of the interrupt and
InterruptPayload is the raw flow.Interrupt(payload) argument; they are returned as distinct
fields rather than pre-merged.
*/
func (svc *Service) Snapshot(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error) { // MARKER: Snapshot
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Query the flow
	retried := false
queryFlow:
	var flowStatus string
	var flowStepID int
	var workflowName string
	var finalStateJSON string
	var graphJSON string
	var flowErrorMsg string
	var flowCancelReason string
	err = db.QueryRowContext(ctx,
		"SELECT status, step_id, workflow_name, final_state, graph, error, cancel_reason FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&flowStatus, &flowStepID, &workflowName, &finalStateJSON, &graphJSON, &flowErrorMsg, &flowCancelReason)
	if err == sql.ErrNoRows {
		return nil, errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	fanOut := flowStepID == 0

	// For terminated flows, return the pre-computed final_state plus the side-channel string.
	if flowStatus == workflow.StatusCompleted || flowStatus == workflow.StatusFailed || flowStatus == workflow.StatusCancelled {
		var finalState map[string]any
		if err = json.Unmarshal([]byte(finalStateJSON), &finalState); err != nil {
			return nil, errors.Trace(err)
		}
		return &workflow.FlowOutcome{
			FlowKey:      flowKey,
			Status:       flowStatus,
			State:        finalState,
			Error:        flowErrorMsg,
			CancelReason: flowCancelReason,
		}, nil
	}

	// Query the current step
	var stateJSON, changesJSON, interruptPayloadJSON, taskName string
	var stepDepth int
	if fanOut {
		// Pick the most recently active step
		err = db.QueryRowContext(ctx,
			"SELECT state, changes, interrupt_payload, task_name, step_depth FROM microbus_steps WHERE flow_id=? AND status IN (?, ?, ?, ?) ORDER BY updated_at DESC LIMIT_OFFSET(1, 0)",
			flowID,
			workflow.StatusCreated, workflow.StatusPending, workflow.StatusRunning, workflow.StatusInterrupted,
		).Scan(&stateJSON, &changesJSON, &interruptPayloadJSON, &taskName, &stepDepth)
	} else {
		err = db.QueryRowContext(ctx,
			"SELECT state, changes, interrupt_payload, task_name, step_depth FROM microbus_steps WHERE step_id=? AND status IN (?, ?, ?, ?)",
			flowStepID,
			workflow.StatusCreated, workflow.StatusPending, workflow.StatusRunning, workflow.StatusInterrupted,
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
		return &workflow.FlowOutcome{FlowKey: flowKey, Status: flowStatus}, nil
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Merge state and changes
	var rawState map[string]any
	err = json.Unmarshal([]byte(stateJSON), &rawState)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var rawChanges map[string]any
	err = json.Unmarshal([]byte(changesJSON), &rawChanges)
	if err != nil {
		return nil, errors.Trace(err)
	}
	rawMerged, err := workflow.MergeState(rawState, rawChanges, nil)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Parse the interrupt payload (if any) as a distinct field. The state is returned
	// without merging the payload - the caller can merge themselves with the graph's
	// reducers if a combined view is wanted.
	var payload map[string]any
	if interruptPayloadJSON != "" && interruptPayloadJSON != "{}" {
		if err = json.Unmarshal([]byte(interruptPayloadJSON), &payload); err != nil {
			return nil, errors.Trace(err)
		}
		if len(payload) == 0 {
			payload = nil
		}
	}

	return &workflow.FlowOutcome{
		FlowKey:          flowKey,
		Status:           flowStatus,
		State:            rawMerged,
		InterruptPayload: payload,
	}, nil
}

// Fingerprint returns a sha256-based opaque hash of the flow's status, total step count, and
// max(step.updated_at) — walked across the flow and all descendant subgraph flows. Cheap to call:
// one PK lookup on microbus_flows plus a recursive descendant walk plus a single aggregate
// SELECT against microbus_steps. Designed as the change-detection probe for long-polling
// watchers — every diagram-affecting change moves the hash, no-op events don't.
func (svc *Service) Fingerprint(ctx context.Context, flowKey string) (fingerprint string, status string, err error) { // MARKER: Fingerprint
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	err = db.QueryRowContext(ctx,
		"SELECT status FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return "", "", errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return "", "", errors.Trace(err)
	}
	status = strings.TrimSpace(status)

	flowIDs := []any{flowID}
	descendants, err := svc.allDescendantSubgraphFlows(ctx, db, flowID)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	for _, id := range descendants {
		flowIDs = append(flowIDs, id)
	}

	ph := strings.Repeat("?,", len(flowIDs)-1) + "?"
	var count int
	// MAX(updated_at) is an untyped aggregate expression; SQLite returns it as a
	// string (no column affinity) while other dialects return a time value. Scan
	// into any and hash its string form — the fingerprint only needs a stable,
	// change-detecting digest, not a parsed timestamp.
	var maxUpdated any
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*), MAX(updated_at) FROM microbus_steps WHERE flow_id IN ("+ph+")",
		flowIDs...,
	).Scan(&count, &maxUpdated)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	if b, ok := maxUpdated.([]byte); ok {
		maxUpdated = string(b)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%v", status, count, maxUpdated)))
	return hex.EncodeToString(sum[:]), status, nil
}

/*
Resume continues an interrupted flow, delivering resumeData to the task that armed flow.Interrupt (recorded on the
leaf step's resume_data column and returned by flow.Interrupt on re-entry). It fails with 409 if the flow is paused
at a breakpoint rather than an interrupt; ResumeBreak is the entry point for that case.
*/
func (svc *Service) Resume(ctx context.Context, flowKey string, resumeData any) (err error) { // MARKER: Resume
	return svc.resume(ctx, flowKey, false, resumeData)
}

/*
ResumeBreak continues a flow paused at a breakpoint, merging stateOverrides into the leaf step's input state so
the about-to-run task observes them (replace semantics; the task has not run yet). It fails with 409 if the flow is
paused at an interrupt rather than a breakpoint; Resume is the entry point for that case.
*/
func (svc *Service) ResumeBreak(ctx context.Context, flowKey string, stateOverrides any) (err error) { // MARKER: ResumeBreak
	return svc.resume(ctx, flowKey, true, stateOverrides)
}

// resume is the shared machinery for Resume (interrupt) and ResumeBreak. It locates the leaf interrupted step,
// verifies its park kind matches breakpoint, applies the kind-specific write to the leaf, re-parks intermediate
// surgraph steps, transitions the chain's flows back to running, and enqueues the leaf. data is the resume payload
// for an interrupt or the state overrides for a breakpoint.
func (svc *Service) resume(ctx context.Context, flowKey string, breakpoint bool, data any) (err error) {
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
	if flowStatus != workflow.StatusInterrupted {
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

	// Verify the leaf's park kind matches the caller's intent. interrupt_done=1 marks a flow.Interrupt park
	// (set when the task armed it); a breakpoint pause leaves it 0 with breakpoint_hit=1. The two are
	// mutually exclusive at the leaf, so each entry point rejects the other's pause outright rather than
	// silently mishandling it.
	var leafInterruptDone, leafBreakpointHit bool
	var leafStateJSON string
	err = db.QueryRowContext(ctx,
		"SELECT interrupt_done, breakpoint_hit, state FROM microbus_steps WHERE step_id=?",
		leafStepID,
	).Scan(&leafInterruptDone, &leafBreakpointHit, &leafStateJSON)
	if err != nil {
		return errors.Trace(err)
	}
	if breakpoint && (leafInterruptDone || !leafBreakpointHit) {
		return errors.New("flow is not paused at a breakpoint; use Resume to continue an interrupt", http.StatusConflict)
	}
	if !breakpoint && !leafInterruptDone {
		return errors.New("flow is not paused at an interrupt; use ResumeBreak to continue a breakpoint", http.StatusConflict)
	}

	// Prepare the kind-specific leaf write. For an interrupt, the payload is recorded on resume_data and
	// returned by flow.Interrupt on re-dispatch (state/changes untouched). For a breakpoint, the overrides
	// are merged onto the leaf's input state with replace semantics so the about-to-run task observes them;
	// resume_data stays empty since the task arms its interrupt fresh.
	resumeDataJSON := "{}"
	breakpointStateJSON := leafStateJSON
	if breakpoint {
		var leafState map[string]any
		err = json.Unmarshal([]byte(leafStateJSON), &leafState)
		if err != nil {
			return errors.Trace(err)
		}
		merged, err := workflow.MergeState(leafState, data, nil)
		if err != nil {
			return errors.Trace(err)
		}
		mergedJSON, err := json.Marshal(merged)
		if err != nil {
			return errors.Trace(err)
		}
		breakpointStateJSON = string(mergedJSON)
	} else if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return errors.Trace(err)
		}
		var resumeMap map[string]any
		err = json.Unmarshal(b, &resumeMap)
		if err != nil {
			return errors.Trace(err)
		}
		if len(resumeMap) > 0 {
			resumeDataJSON = string(b)
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

	// Re-park surgraph steps: restore to (status='running', parked=1). No lease manipulation -
	// the parked column itself excludes them from selection/saturation/recovery while the leaf
	// re-runs and propagates resolution back up.
	if len(parkStepIDs) > 0 {
		parkPlaceholders := strings.Repeat("?,", len(parkStepIDs)-1) + "?"
		parkArgs := append([]any{workflow.StatusRunning, parkedSubgraph}, parkStepIDs...)
		parkArgs = append(parkArgs, workflow.StatusInterrupted)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, parked=?, updated_at=NOW_UTC() WHERE step_id IN ("+parkPlaceholders+") AND status=?",
			parkArgs...,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Reset the leaf step to pending. An interrupt records resumeData on resume_data (state/changes
	// untouched, interrupt_done was set at park time); a breakpoint writes the merged input state and
	// leaves breakpoint_hit=1 so the re-dispatch skips the breakpoint instead of re-triggering it.
	var res sql.Result
	if breakpoint {
		res, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, state=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			workflow.StatusPending, breakpointStateJSON, leafStepID, workflow.StatusInterrupted,
		)
	} else {
		res, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, resume_data=?, lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
			workflow.StatusPending, resumeDataJSON, leafStepID, workflow.StatusInterrupted,
		)
	}
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
			workflow.StatusRunning, chainFlowID, workflow.StatusInterrupted,
			chainFlowID, workflow.StatusInterrupted,
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
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, chainCompositeIDs[i], workflow.StatusRunning)
	}

	// If another step anywhere in the chain is still interrupted with a payload,
	// propagate it up so the caller sees it on the next State()/Await() call.
	// The next interrupt can be at any level (fan-out sibling at any depth).
	if len(parkStepIDs) > 0 {
		flowPlaceholders := strings.Repeat("?,", len(chainFlowIDs)-1) + "?"
		findArgs := append([]any{workflow.StatusInterrupted}, chainFlowIDs...)
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
The reason string is stored as cancel_reason on every flow in the chain; the existing
status-guard WHERE clause makes it first-cancel-wins for the flow row.
*/
func (svc *Service) Cancel(ctx context.Context, flowKey string, reason string) (err error) { // MARKER: Cancel
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
	if flowStatus == workflow.StatusCompleted || flowStatus == workflow.StatusFailed || flowStatus == workflow.StatusCancelled {
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

	// Cancel all active steps across all flows in one UPDATE per table. parked=parkedNone keeps the
	// terminal-implies-unparked invariant: many of these steps are subgraph callers waiting on a
	// child (parked=parkedSubgraph) or pending rows held by a tripped breaker (parked=parkedBreaker);
	// leaving them parked would strand them outside the selection index for a future Restart.
	flowPlaceholders := strings.Repeat("?,", len(allFlowIDs)-1) + "?"
	stepArgs := append([]any{workflow.StatusCancelled, parkedNone}, allFlowIDs...)
	stepArgs = append(stepArgs, workflow.StatusCreated, workflow.StatusPending, workflow.StatusInterrupted, workflow.StatusRunning)
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status IN (?, ?, ?, ?)",
		stepArgs...,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Cancel surgraph caller steps in the chain (if any). These are precisely the steps that were
	// parked=parkedSubgraph waiting for their child to resolve - the case where leaving parked != 0
	// causes the wedged-Restart symptom this whole invariant exists to prevent.
	if len(surgraphStepIDs) > 0 {
		surgraphStepPlaceholders := strings.Repeat("?,", len(surgraphStepIDs)-1) + "?"
		surgraphStepArgs := append([]any{workflow.StatusCancelled, parkedNone}, surgraphStepIDs...)
		surgraphStepArgs = append(surgraphStepArgs, workflow.StatusCreated, workflow.StatusPending, workflow.StatusInterrupted, workflow.StatusRunning)
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, parked=?, updated_at=NOW_UTC() WHERE step_id IN ("+surgraphStepPlaceholders+") AND status IN (?, ?, ?, ?)",
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

	// Cancel all flows with their computed final_state via CASE.
	// cancel_reason is set in the same UPDATE; the WHERE-clause status guard provides first-cancel-wins.
	caseClause := "CASE"
	flowArgs := []any{}
	for i, fid := range allFlowIDs {
		caseClause += " WHEN flow_id=? THEN ?"
		flowArgs = append(flowArgs, fid, finalStates[i])
	}
	caseClause += " END"
	flowArgs = append(flowArgs, workflow.StatusCancelled, reason)
	flowArgs = append(flowArgs, allFlowIDs...)
	flowArgs = append(flowArgs, workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled)
	res, err := tx.ExecContext(ctx,
		"UPDATE microbus_flows SET final_state="+caseClause+", status=?, cancel_reason=?, updated_at=NOW_UTC() WHERE flow_id IN ("+flowPlaceholders+") AND status NOT IN (?, ?, ?)",
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
			foremanapi.NewMulticastTrigger(svc).ForHost(rootNotifyHostname).OnFlowStopped(ctx, &workflow.FlowOutcome{
				FlowKey:      rootCompositeID,
				Status:       workflow.StatusCancelled,
				State:        finalState,
				CancelReason: reason,
			})
		}
	}
	for i, cid := range allCompositeIDs {
		svc.LogInfo(ctx, "Flow status transition", "flow", allFlowIDs[i], "to", workflow.StatusCancelled)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, cid, workflow.StatusCancelled)
	}
	return nil
}

/*
Restart re-executes a terminated flow from its entry step. Every step except the entry step is
deleted, descendant subgraph flows (and their steps) are deleted, the entry step is reset to
pending with state = merge(originalEntryState, stateOverrides), and the flow row is flipped to
running. The flow must be in a terminal status (completed/failed/cancelled/interrupted).

stateOverrides is a top-level shallow replace over the original entry state: keys present in
overrides win; keys mapped to null (nil) are deleted; keys absent in overrides are preserved.
*/
func (svc *Service) Restart(ctx context.Context, flowKey string, stateOverrides any) (err error) { // MARKER: Restart
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

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
	if !isRestartable(flowStatus) {
		return errors.New("flow is not in a terminal status (status: %s)", flowStatus, http.StatusConflict)
	}

	// Find the entry step: the one with predecessor_id=0 in this flow. Read its current state so
	// we can apply overrides before reset.
	var entryStepID int
	var entryState string
	err = db.QueryRowContext(ctx,
		"SELECT step_id, state FROM microbus_steps WHERE flow_id=? AND predecessor_id=0",
		flowID,
	).Scan(&entryStepID, &entryState)
	if err == sql.ErrNoRows {
		return errors.New("flow has no entry step", http.StatusInternalServerError)
	}
	if err != nil {
		return errors.Trace(err)
	}

	mergedStateJSON, err := mergeWithOverrides(entryState, stateOverrides)
	if err != nil {
		return errors.Trace(err)
	}

	descendantFlowIDs, err := svc.allDescendantSubgraphFlows(ctx, db, flowID)
	if err != nil {
		return errors.Trace(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	if len(descendantFlowIDs) > 0 {
		ph := strings.Repeat("?,", len(descendantFlowIDs)-1) + "?"
		args := make([]any, 0, len(descendantFlowIDs))
		for _, id := range descendantFlowIDs {
			args = append(args, id)
		}
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
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Wipe every step of this flow except the entry step. The DELETE clears every spawn step too,
	// so cohort_arrivals/cohort_failures bookkeeping resets automatically.
	_, err = tx.ExecContext(ctx,
		"DELETE FROM microbus_steps WHERE flow_id=? AND step_id<>?",
		flowID, entryStepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// created_at is reset on the entry step alongside the in-place rewind so duration math and the
	// step's fairness-key age reflect this attempt, not the original failed run. parked=parkedNone
	// is defensive: the terminal transition that left this step in its prior state should already
	// have reset parked (see "Step Parking" in CLAUDE.md), but Restart-explicit re-zeroing keeps
	// it self-contained - any future code path that leaves a terminal step parked can't wedge a
	// Restart by hiding the entry step from the selection index.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, state=?, changes='{}', error='', goto_next='',"+
			" attempt=0, breakpoint_hit=0, interrupt_done=0, resume_data='{}',"+
			" subgraph_done=0, subgraph_result='{}', subgraph_error='',"+
			" successor_id=0, cohort_arrivals=0, cohort_failures=0,"+
			" not_before=NOW_UTC(), lease_expires=NOW_UTC(), created_at=NOW_UTC(), updated_at=NOW_UTC()"+
			" WHERE step_id=?",
		workflow.StatusPending, parkedNone, mergedStateJSON, entryStepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Reset created_at on the flow row too: Flow.CreatedAt() is exposed to tasks for elapsed-time
	// guards, and a guard wired against the original creation moment would fire immediately on
	// every restarted attempt past the threshold. started_at is reset alongside so duration
	// metrics reflect this attempt's wall time, not the original run's.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_flows SET status=?, step_id=?, error='', cancel_reason='', final_state='{}', created_at=NOW_UTC(), started_at=NOW_UTC(), updated_at=NOW_UTC()"+
			" WHERE flow_id=? AND flow_token=?",
		workflow.StatusRunning, entryStepID, flowID, flowToken,
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "from", flowStatus, "to", workflow.StatusRunning, "via", "Restart")
	foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, flowKey, workflow.StatusRunning)
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, entryStepID)
	return nil
}

/*
RestartFrom re-executes a flow from the named step. The DAG subtree downstream of the step (walked
via successor_id) is deleted; cohort counters on every affected spawn are decremented to reflect
the swept members' contributions; descendant subgraph flows spawned by any swept step are
cascade-deleted; the target step is UPDATEd in place to pending with state =
merge(originalStepState, stateOverrides); the flow row is flipped to running.

When the target step lives inside a subgraph child flow, every ancestor in the surgraph chain gets
the same treatment around its caller step: the caller's downstream DAG subtree in the parent is
swept, the caller is reset to its parked-subgraph state, and the parent flow is flipped to running.
Sweeping cascades up because the parent's downstream consumed the (now stale) result of the child
that's about to re-run. The caller's own child flow is NOT deleted — that's the one we're operating
in.

The target step must be in a terminal status (completed/failed/cancelled/interrupted). The target's
flow can be in any non-cancelled status.
*/
func (svc *Service) RestartFrom(ctx context.Context, stepKey string, stateOverrides any) (err error) { // MARKER: RestartFrom
	shardNum, stepID, stepToken, err := parseStepKey(stepKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}

	var flowID, lineageID, predecessorID int
	var stepStatus, stepState string
	err = db.QueryRowContext(ctx,
		"SELECT flow_id, status, state, lineage_id, predecessor_id FROM microbus_steps WHERE step_id=? AND step_token=?",
		stepID, stepToken,
	).Scan(&flowID, &stepStatus, &stepState, &lineageID, &predecessorID)
	if err == sql.ErrNoRows {
		return errors.New("step not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	stepStatus = strings.TrimSpace(stepStatus)
	if !isRestartableStep(stepStatus) {
		return errors.New("step is not in a terminal status (status: %s)", stepStatus, http.StatusConflict)
	}

	var flowStatus, flowToken string
	err = db.QueryRowContext(ctx,
		"SELECT status, flow_token FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&flowStatus, &flowToken)
	if err != nil {
		return errors.Trace(err)
	}
	flowStatus = strings.TrimSpace(flowStatus)
	flowToken = strings.TrimSpace(flowToken)
	if flowStatus == workflow.StatusCancelled {
		return errors.New("flow is cancelled and cannot be restarted from a step", http.StatusConflict)
	}

	// Within-flow sweep of the target's downstream.
	subtree, err := svc.collectDAGSubtree(ctx, db, flowID, stepID)
	if err != nil {
		return errors.Trace(err)
	}

	// Walk up the surgraph chain so each ancestor's caller step (the one that called flow.Subgraph
	// down toward this flow) and that caller's downstream get the same sweep. Same shard throughout
	// (subgraph children share their parent's shard).
	upFlowIDs, upStepIDs, _, err := svc.surgraphChain(ctx, shardNum, flowID, flowToken)
	if err != nil {
		return errors.Trace(err)
	}

	type parentLevel struct {
		flowID       int
		flowToken    string
		flowStatus   string
		callerStepID int
		subtree      []sweptMember
	}
	var parents []parentLevel
	for i, sid := range upStepIDs {
		parentFlowID, ok := upFlowIDs[i+1].(int)
		if !ok {
			return errors.New("unexpected non-int parent flowID at level %d", i)
		}
		callerStepID, ok := sid.(int)
		if !ok {
			return errors.New("unexpected non-int surgraph stepID at level %d", i)
		}
		var pTok, pStatus string
		err = db.QueryRowContext(ctx,
			"SELECT flow_token, status FROM microbus_flows WHERE flow_id=?",
			parentFlowID,
		).Scan(&pTok, &pStatus)
		if err != nil {
			return errors.Trace(err)
		}
		pStatus = strings.TrimSpace(pStatus)
		if pStatus == workflow.StatusCancelled {
			return errors.New("ancestor surgraph flow is cancelled and cannot be restarted into", http.StatusConflict)
		}
		pSub, err := svc.collectDAGSubtree(ctx, db, parentFlowID, callerStepID)
		if err != nil {
			return errors.Trace(err)
		}
		parents = append(parents, parentLevel{
			flowID:       parentFlowID,
			flowToken:    strings.TrimSpace(pTok),
			flowStatus:   pStatus,
			callerStepID: callerStepID,
			subtree:      pSub,
		})
	}

	mergedStateJSON, err := mergeWithOverrides(stepState, stateOverrides)
	if err != nil {
		return errors.Trace(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	// Cascade-delete subgraph flows rooted at any swept step, in the target's flow and at each
	// parent level. Caller steps themselves are NOT in any swept set (their child flows are the
	// surgraph chain we're operating in), so they survive untouched.
	sweepEverywhere := func(members []sweptMember) error {
		for _, m := range members {
			err = svc.deleteSubgraphFlowsRootedAt(ctx, tx, m.stepID)
			if err != nil {
				return errors.Trace(err)
			}
		}
		return nil
	}
	err = sweepEverywhere(subtree)
	if err != nil {
		return errors.Trace(err)
	}
	for _, p := range parents {
		err = sweepEverywhere(p.subtree)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Tally per-spawn decrements across every swept member (target's local subtree, the target
	// itself, and each parent level's subtree). Caller steps stay put and don't participate.
	type cohortDelta struct{ arrivalsDelta, failuresDelta int }
	deltaBySpawn := map[int]*cohortDelta{}
	bump := func(spawnID int, status string) {
		if spawnID == 0 {
			return
		}
		d, ok := deltaBySpawn[spawnID]
		if !ok {
			d = &cohortDelta{}
			deltaBySpawn[spawnID] = d
		}
		switch status {
		case workflow.StatusCompleted, workflow.StatusCancelled, workflow.StatusFailed:
			d.arrivalsDelta++
		}
		if status == workflow.StatusFailed {
			d.failuresDelta++
		}
	}
	for _, m := range subtree {
		bump(m.lineageID, m.status)
	}
	bump(lineageID, stepStatus)
	for _, p := range parents {
		for _, m := range p.subtree {
			bump(m.lineageID, m.status)
		}
	}

	// Delete swept rows in one statement per level.
	deleteIDs := func(members []sweptMember) error {
		if len(members) == 0 {
			return nil
		}
		ids := make([]any, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.stepID)
		}
		ph := strings.Repeat("?,", len(ids)-1) + "?"
		_, err = tx.ExecContext(ctx,
			"DELETE FROM microbus_steps WHERE step_id IN ("+ph+")",
			ids...,
		)
		return errors.Trace(err)
	}
	err = deleteIDs(subtree)
	if err != nil {
		return errors.Trace(err)
	}
	for _, p := range parents {
		err = deleteIDs(p.subtree)
		if err != nil {
			return errors.Trace(err)
		}
	}

	if predecessorID != 0 {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET successor_id=0 WHERE step_id=? AND successor_id<>?",
			predecessorID, stepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	for spawnID, d := range deltaBySpawn {
		err = svc.undoCohortBumps(ctx, tx, spawnID, d.arrivalsDelta, d.failuresDelta)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// created_at reset so duration math reflects this attempt; the surrounding flow's created_at is
	// left intact since only this step is being rewound, not the whole flow. parked=parkedNone is
	// defensive (see Restart for the rationale): keep the target step self-contained against any
	// future code path that might leave a terminal step parked.
	_, err = tx.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, state=?, changes='{}', error='', goto_next='',"+
			" attempt=0, breakpoint_hit=0, interrupt_done=0, resume_data='{}',"+
			" subgraph_done=0, subgraph_result='{}', subgraph_error='',"+
			" successor_id=0,"+
			" not_before=NOW_UTC(), lease_expires=NOW_UTC(), created_at=NOW_UTC(), updated_at=NOW_UTC()"+
			" WHERE step_id=?",
		workflow.StatusPending, parkedNone, mergedStateJSON, stepID,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Reset each parent surgraph caller step back to its parked-waiting-for-child state. Its
	// subgraph_done/result/error are cleared so completeSurgraphFlow re-fires when the restarted
	// child eventually completes. successor_id is cleared because the caller's old successor was
	// just swept.
	for _, p := range parents {
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, parked=?, subgraph_done=0, subgraph_result='{}', subgraph_error='', successor_id=0, error='', goto_next='', lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=?",
			workflow.StatusRunning, parkedSubgraph, p.callerStepID,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	if flowStatus != workflow.StatusRunning {
		// RestartFrom is a surgical rewind, not a fresh attempt. The flow's started_at is
		// preserved so duration metrics reflect the full lifespan since the original Start.
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET status=?, error='', cancel_reason='', final_state='{}', updated_at=NOW_UTC()"+
				" WHERE flow_id=? AND flow_token=? AND status<>?",
			workflow.StatusRunning, flowID, flowToken, workflow.StatusCancelled,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}
	for _, p := range parents {
		if p.flowStatus == workflow.StatusRunning {
			continue
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_flows SET status=?, error='', cancel_reason='', final_state='{}', updated_at=NOW_UTC()"+
				" WHERE flow_id=? AND flow_token=? AND status<>?",
			workflow.StatusRunning, p.flowID, p.flowToken, workflow.StatusCancelled,
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Trace(err)
	}

	flowKey := fmt.Sprintf("%d-%d-%s", shardNum, flowID, flowToken)
	if flowStatus != workflow.StatusRunning {
		svc.LogInfo(ctx, "Flow status transition", "flow", flowID, "from", flowStatus, "to", workflow.StatusRunning, "via", "RestartFrom", "step", stepID)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, flowKey, workflow.StatusRunning)
	}
	for _, p := range parents {
		if p.flowStatus == workflow.StatusRunning {
			continue
		}
		parentFlowKey := fmt.Sprintf("%d-%d-%s", shardNum, p.flowID, p.flowToken)
		svc.LogInfo(ctx, "Flow status transition", "flow", p.flowID, "from", p.flowStatus, "to", workflow.StatusRunning, "via", "RestartFrom", "surgraphStep", p.callerStepID)
		foremanapi.NewMulticastClient(svc).NotifyStatusChange(ctx, parentFlowKey, workflow.StatusRunning)
	}
	foremanapi.NewMulticastClient(svc).Enqueue(ctx, shardNum, stepID)
	return nil
}

// skipSurgraphForward walks past any surgraph-step wrappers on the forward
// path: if id is a step that has a subgraph attached, return that subgraph's
// entry step instead. Loops for nested subgraphs. Returns id unchanged when it
// is 0 or not a surgraph step.
func (svc *Service) skipSurgraphForward(ctx context.Context, db *sequel.DB, id int) (int, error) {
	for id > 0 {
		var childFlow int
		err := db.QueryRowContext(ctx,
			"SELECT flow_id FROM microbus_flows WHERE surgraph_step_id=?",
			id,
		).Scan(&childFlow)
		if err == sql.ErrNoRows {
			return id, nil
		}
		if err != nil {
			return 0, errors.Trace(err)
		}
		if childFlow == 0 {
			return id, nil
		}
		var entry int
		err = db.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps WHERE flow_id=? AND predecessor_id=0 ORDER BY step_id LIMIT 1",
			childFlow,
		).Scan(&entry)
		if err != nil {
			if err == sql.ErrNoRows {
				return id, nil
			}
			return 0, errors.Trace(err)
		}
		id = entry
	}
	return id, nil
}

// skipSurgraphBackward is the backward counterpart: if id is a surgraph
// wrapper, return the subgraph's exit step (completed, with successor_id=0).
// Loops for nested subgraphs.
func (svc *Service) skipSurgraphBackward(ctx context.Context, db *sequel.DB, id int) (int, error) {
	for id > 0 {
		var childFlow int
		err := db.QueryRowContext(ctx,
			"SELECT flow_id FROM microbus_flows WHERE surgraph_step_id=?",
			id,
		).Scan(&childFlow)
		if err == sql.ErrNoRows {
			return id, nil
		}
		if err != nil {
			return 0, errors.Trace(err)
		}
		if childFlow == 0 {
			return id, nil
		}
		var exit int
		err = db.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps WHERE flow_id=? AND successor_id=0 AND status='completed' ORDER BY step_id DESC LIMIT 1",
			childFlow,
		).Scan(&exit)
		if err != nil {
			if err == sql.ErrNoRows {
				return id, nil
			}
			return 0, errors.Trace(err)
		}
		id = exit
	}
	return id, nil
}

/*
Step returns the full detail of one execution step, including the state,
changes and interrupt payload that History intentionally omits to keep
flow-wide responses bounded.
*/
func (svc *Service) Step(ctx context.Context, stepKey string) (step *foremanapi.FlowStep, err error) { // MARKER: Step
	shardNum, stepID, stepToken, err := parseStepKey(stepKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var taskName, statusStr, errMsg string
	var stateJSON, changesJSON, interruptJSON string
	var stepDepth, attempt, predID, succID int
	var createdAt, updatedAt time.Time
	err = db.QueryRowContext(ctx,
		"SELECT step_depth, task_name, attempt, state, changes, interrupt_payload, status, error, created_at, updated_at, predecessor_id, successor_id FROM microbus_steps WHERE step_id=? AND step_token=?",
		stepID, stepToken,
	).Scan(&stepDepth, &taskName, &attempt, &stateJSON, &changesJSON, &interruptJSON, &statusStr, &errMsg, &createdAt, &updatedAt, &predID, &succID)
	if err == sql.ErrNoRows {
		return nil, errors.New("step not found", http.StatusNotFound)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	fs := &foremanapi.FlowStep{
		StepKey:       stepKey,
		StepID:        stepID,
		StepDepth:     stepDepth,
		TaskName:      taskName,
		Attempt:       attempt,
		PredecessorID: predID,
		SuccessorID:   succID,
		Status:        strings.TrimSpace(statusStr),
		Error:         strings.TrimSpace(errMsg),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	err = json.Unmarshal([]byte(stateJSON), &fs.State)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = json.Unmarshal([]byte(changesJSON), &fs.Changes)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if interruptJSON != "" {
		err = json.Unmarshal([]byte(interruptJSON), &fs.InterruptPayload)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	// Navigation skips the surgraph wrapper entirely: it's a routing/structural
	// step (parked while the subgraph runs) and isn't on the execution timeline
	// the user wants to walk. We resolve the effective neighbor in three steps:
	//
	//   1. Start with the intra-flow predecessor_id / successor_id.
	//   2. If the current step is a subgraph entry/exit (predID/succID == 0),
	//      stitch across the seam to the *parent's* surgraph-step's intra-flow
	//      neighbor (skipping the wrapper itself).
	//   3. If the current step is itself a surgraph (has a child flow attached),
	//      jump straight to that child flow's entry on successor.
	//   4. Repeat the "neighbor is a surgraph -> descend" walk until the
	//      effective neighbor is a regular step. Nested subgraphs naturally
	//      unwrap in one direction or the other.
	effectivePredID := predID
	effectiveSuccID := succID
	if predID == 0 || succID == 0 {
		// We may be inside a subgraph - look up our own flow's surgraph linkage.
		var surgraphStepID int
		err = db.QueryRowContext(ctx,
			"SELECT f.surgraph_step_id FROM microbus_steps s JOIN microbus_flows f ON s.flow_id = f.flow_id WHERE s.step_id=?",
			stepID,
		).Scan(&surgraphStepID)
		if err != nil && err != sql.ErrNoRows {
			return nil, errors.Trace(err)
		}
		if surgraphStepID > 0 {
			// Cross-flow ascend: skip the surgraph wrapper and jump to its
			// intra-flow neighbor in the parent flow.
			var parentPred, parentSucc int
			err = db.QueryRowContext(ctx,
				"SELECT predecessor_id, successor_id FROM microbus_steps WHERE step_id=?",
				surgraphStepID,
			).Scan(&parentPred, &parentSucc)
			if err != nil && err != sql.ErrNoRows {
				return nil, errors.Trace(err)
			}
			if effectivePredID == 0 && parentPred > 0 {
				effectivePredID = parentPred
			}
			if effectiveSuccID == 0 && parentSucc > 0 {
				effectiveSuccID = parentSucc
			}
		}
	}
	// If the current step itself is a surgraph, descend on the successor side
	// (entry of its subgraph) so navigation enters the child instead of skipping
	// past it.
	var ownChildFlow int
	err = db.QueryRowContext(ctx,
		"SELECT flow_id FROM microbus_flows WHERE surgraph_step_id=?",
		stepID,
	).Scan(&ownChildFlow)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Trace(err)
	}
	if ownChildFlow > 0 {
		var entry int
		err = db.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps WHERE flow_id=? AND predecessor_id=0 ORDER BY step_id LIMIT 1",
			ownChildFlow,
		).Scan(&entry)
		if err != nil && err != sql.ErrNoRows {
			return nil, errors.Trace(err)
		}
		if entry > 0 {
			effectiveSuccID = entry
		}
	}
	// Walk past any surgraph wrapper that the effective neighbor lands on,
	// descending into the appropriate end of the subgraph (entry for forward,
	// exit for backward). Loop in case of nested subgraphs.
	effectiveSuccID, err = svc.skipSurgraphForward(ctx, db, effectiveSuccID)
	if err != nil {
		return nil, errors.Trace(err)
	}
	effectivePredID, err = svc.skipSurgraphBackward(ctx, db, effectivePredID)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Resolve predecessor/successor step keys for navigation. The cross-flow
	// fallbacks above still land on the same shard (subgraph flows have shard
	// affinity with their parent), so one IN query fetches both rows.
	if effectivePredID > 0 || effectiveSuccID > 0 {
		var ids []any
		if effectivePredID > 0 {
			ids = append(ids, effectivePredID)
		}
		if effectiveSuccID > 0 && effectiveSuccID != effectivePredID {
			ids = append(ids, effectiveSuccID)
		}
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		nrows, err := db.QueryContext(ctx,
			"SELECT step_id, step_token FROM microbus_steps WHERE step_id IN ("+placeholders+")",
			ids...,
		)
		if err != nil {
			return nil, errors.Trace(err)
		}
		defer nrows.Close()
		for nrows.Next() {
			var nid int
			var ntoken string
			if err := nrows.Scan(&nid, &ntoken); err != nil {
				return nil, errors.Trace(err)
			}
			key := fmt.Sprintf("%d-%d-%s", shardNum, nid, strings.TrimSpace(ntoken))
			if nid == effectivePredID {
				fs.PrevKey = key
			}
			if nid == effectiveSuccID {
				fs.NextKey = key
			}
		}
	}
	return fs, nil
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


// ShardInfo probes every database shard in parallel and returns per-shard health (latency, row
// counts, error). A shard that fails any of its probes contributes an entry with non-empty
// Error and partial counts; the call itself does not fail. Shards are 1-indexed in the result.
func (svc *Service) ShardInfo(ctx context.Context) (shards []foremanapi.ShardSummary, err error) { // MARKER: ShardInfo
	numShards := svc.numDBShards()
	// Slot 0 unused; shards 1..numShards.
	results := make([]foremanapi.ShardSummary, numShards+1)
	jobs := make([]func() error, 0, numShards)
	for i := 1; i <= numShards; i++ {
		shardIdx := i
		jobs = append(jobs, func() error {
			results[shardIdx].Shard = shardIdx
			db, err := svc.shard(shardIdx)
			if err != nil {
				results[shardIdx].Error = err.Error()
				return nil
			}
			start := time.Now()
			var one int
			err = db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
			results[shardIdx].LatencyMs = int(time.Since(start) / time.Millisecond)
			if err != nil {
				results[shardIdx].Error = err.Error()
				return nil
			}
			var steps, flows int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM microbus_steps").Scan(&steps); err != nil {
				results[shardIdx].Error = err.Error()
				return nil
			}
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM microbus_flows").Scan(&flows); err != nil {
				results[shardIdx].Error = err.Error()
				return nil
			}
			results[shardIdx].Steps = steps
			results[shardIdx].Flows = flows
			return nil
		})
	}
	_ = svc.Parallel(jobs...)
	shards = make([]foremanapi.ShardSummary, 0, numShards)
	for i := 1; i <= numShards; i++ {
		shards = append(shards, results[i])
	}
	return shards, nil
}

// List queries flows by status, workflow name, or thread. Pagination is per-shard; pass
// Query.Cursor = the previous call's ListOut.NextCursor to get the next page. Design rationale
// for the per-shard pagination shape is in coreservices/foreman/CLAUDE.md under "Database
// Sharding."
func (svc *Service) List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, nextCursor string, err error) { // MARKER: List
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	numShards := svc.numDBShards()

	joinSQL, whereSQL, baseArgs, restrictShardNum, err := svc.queryClauses(ctx, query)
	if err != nil {
		return nil, "", errors.Trace(err)
	}

	// Decode the opaque cursor of the form "s=fid,s=fid,...". Shards absent from the cursor
	// have no upper bound and start from the top.
	perShardCursor := map[int]int{}
	if query.Cursor != "" {
		for _, part := range strings.Split(query.Cursor, ",") {
			s, fid, ok := strings.Cut(part, "=")
			if !ok {
				return nil, "", errors.New("malformed cursor", http.StatusBadRequest)
			}
			si, sErr := strconv.Atoi(s)
			fi, fErr := strconv.Atoi(fid)
			if sErr != nil || fErr != nil || si < 1 {
				return nil, "", errors.New("malformed cursor", http.StatusBadRequest)
			}
			perShardCursor[si] = fi
		}
	}

	// Per-shard quota. Single-shard queries (thread or Query.Shard) get the full limit; otherwise
	// the limit is sliced ceil(limit/numShards) per shard so the aggregate stays close to limit.
	// The ceiling guarantees at least one row per shard.
	singleShard := restrictShardNum != 0
	perShardLimit := limit
	if !singleShard && numShards > 0 {
		perShardLimit = (limit + numShards - 1) / numShards
		if perShardLimit < 1 {
			perShardLimit = 1
		}
	}

	type listRow struct {
		summary foremanapi.FlowSummary
		flowID  int
	}
	// Slot 0 unused; shards 1..numShards.
	perShard := make([][]listRow, numShards+1)

	queryShard := func(shardIdx int) func() error {
		// restrictShardNum, if set, makes other shards no-ops.
		if restrictShardNum != 0 && shardIdx != restrictShardNum {
			return func() error { return nil }
		}
		return func() (err error) {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}
			conditions := []string{whereSQL}
			args := append([]any(nil), baseArgs...)
			if cur, ok := perShardCursor[shardIdx]; ok {
				conditions = append(conditions, "f.flow_id<?")
				args = append(args, cur)
			}
			if sc, scArgs := searchClause(db.DriverName(), shardIdx, query.Search); sc != "" {
				conditions = append(conditions, sc)
				args = append(args, scArgs...)
			}
			args = append(args, perShardLimit)
			stmt := "SELECT f.flow_id, f.flow_token, f.thread_id, f.thread_token, f.workflow_name, f.status, s.task_name, f.error, f.cancel_reason, f.created_at, f.started_at, f.updated_at" +
				" FROM microbus_flows f" + joinSQL +
				" WHERE " + strings.Join(conditions, " AND ") +
				" ORDER BY f.flow_id DESC LIMIT_OFFSET(?, 0)"
			rows, err := db.QueryContext(ctx, stmt, args...)
			if err != nil {
				return errors.Trace(err)
			}
			defer rows.Close()
			var shardRows []listRow
			for rows.Next() {
				var lr listRow
				var flowToken, threadToken, flowError, cancelReason string
				var threadID int
				var taskName sql.NullString
				err = rows.Scan(&lr.flowID, &flowToken, &threadID, &threadToken, &lr.summary.WorkflowName, &lr.summary.Status, &taskName, &flowError, &cancelReason, &lr.summary.CreatedAt, &lr.summary.StartedAt, &lr.summary.UpdatedAt)
				if err != nil {
					return errors.Trace(err)
				}
				lr.summary.FlowKey = fmt.Sprintf("%d-%d-%s", shardIdx, lr.flowID, strings.TrimSpace(flowToken))
				lr.summary.ThreadKey = fmt.Sprintf("%d-%d-%s", shardIdx, threadID, strings.TrimSpace(threadToken))
				lr.summary.Status = strings.TrimSpace(lr.summary.Status)
				lr.summary.TaskName = taskName.String
				lr.summary.Error = strings.TrimSpace(flowError)
				lr.summary.CancelReason = strings.TrimSpace(cancelReason)
				shardRows = append(shardRows, lr)
			}
			if err := rows.Err(); err != nil {
				return errors.Trace(err)
			}
			perShard[shardIdx] = shardRows
			return nil
		}
	}
	jobs := make([]func() error, 0, numShards)
	for i := 1; i <= numShards; i++ {
		jobs = append(jobs, queryShard(i))
	}
	err = svc.Parallel(jobs...)
	if err != nil {
		return nil, "", errors.Trace(err)
	}

	// Aggregate shard-grouped (each shard's rows already flow_id DESC). Build the next cursor by
	// pinning each shard's smallest returned flow_id; shards that returned no rows carry their
	// previous cursor forward (still "below this id"), or are absent if they had no prior cursor.
	nextPerShard := map[int]int{}
	for s, fid := range perShardCursor {
		nextPerShard[s] = fid
	}
	for s := 1; s <= numShards; s++ {
		rows := perShard[s]
		if len(rows) == 0 {
			continue
		}
		// Smallest flow_id is the last one in DESC order.
		nextPerShard[s] = rows[len(rows)-1].flowID
		for _, lr := range rows {
			flows = append(flows, lr.summary)
		}
	}
	// If no shard advanced this call (every shard returned zero rows), we have reached the end.
	anyAdvance := false
	for s, fid := range nextPerShard {
		if cur, had := perShardCursor[s]; !had || cur != fid {
			anyAdvance = true
			break
		}
	}
	if anyAdvance {
		// Stable encoding: sort by shard so the cursor string is deterministic.
		shardOrder := make([]int, 0, len(nextPerShard))
		for s := range nextPerShard {
			shardOrder = append(shardOrder, s)
		}
		sort.Ints(shardOrder)
		var b strings.Builder
		for i, s := range shardOrder {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Itoa(s))
			b.WriteByte('=')
			b.WriteString(strconv.Itoa(nextPerShard[s]))
		}
		nextCursor = b.String()
	}
	return flows, nextCursor, nil
}

// searchClause builds the per-shard SQL predicate and bind args for Query.Search. The match
// is case-insensitive substring against workflow_name, current task_name, error, cancel_reason,
// and the composed flow key "{shard}-{flow_id}-{flow_token}". Empty search yields "", nil.
func searchClause(driverName string, shardIdx int, search string) (sql string, args []any) {
	if search == "" {
		return "", nil
	}
	pattern := "%" + strings.ToLower(search) + "%"
	var flowKeyExpr string
	switch driverName {
	case "mysql", "mssql":
		flowKeyExpr = fmt.Sprintf("CONCAT('%d-', f.flow_id, '-', TRIM(f.flow_token))", shardIdx)
	default:
		flowKeyExpr = fmt.Sprintf("'%d-' || f.flow_id || '-' || TRIM(f.flow_token)", shardIdx)
	}
	sql = "(LOWER(f.workflow_name) LIKE ? OR LOWER(s.task_name) LIKE ? OR LOWER(f.error) LIKE ? OR LOWER(f.cancel_reason) LIKE ? OR LOWER(" + flowKeyExpr + ") LIKE ?)"
	return sql, []any{pattern, pattern, pattern, pattern, pattern}
}

// queryClauses resolves a Query into the SQL fragments shared by List, Purge, and other
// query-driven endpoints. Returns the FROM-clause join, the WHERE-clause body (without a leading
// "WHERE"), the bind args matching whereSQL placeholders, and a restrictShardNum which (when non-
// zero) restricts the operation to a single shard (set by Query.ThreadKey or Query.Shard).
// The returned whereSQL always includes the root-flow filter (surgraph_flow_id=0). The cursor
// predicate is NOT included; callers that paginate append it per shard.
func (svc *Service) queryClauses(ctx context.Context, query foremanapi.Query) (joinSQL string, whereSQL string, args []any, restrictShardNum int, err error) {
	numShards := svc.numDBShards()
	if query.Shard < 0 || query.Shard > numShards {
		return "", "", nil, 0, errors.New("invalid shard", http.StatusBadRequest)
	}
	restrictShardNum = query.Shard

	conditions := []string{"f.surgraph_flow_id=0"}
	if query.Status != "" {
		conditions = append(conditions, "f.status=?")
		args = append(args, query.Status)
	}
	if query.WorkflowName != "" {
		conditions = append(conditions, "f.workflow_name=?")
		args = append(args, query.WorkflowName)
	}
	if query.ThreadKey != "" {
		threadShardNum, threadFlowID, threadFlowToken, parseErr := parseFlowKey(query.ThreadKey)
		if parseErr != nil {
			return "", "", nil, 0, errors.Trace(parseErr)
		}
		db, dErr := svc.shard(threadShardNum)
		if dErr != nil {
			return "", "", nil, 0, errors.Trace(dErr)
		}
		var resolvedThreadID int
		err = db.QueryRowContext(ctx,
			"SELECT thread_id FROM microbus_flows WHERE flow_id=? AND flow_token=?",
			threadFlowID, threadFlowToken,
		).Scan(&resolvedThreadID)
		if err != nil {
			return "", "", nil, 0, errors.New("flow not found", http.StatusNotFound)
		}
		conditions = append(conditions, "f.thread_id=?")
		args = append(args, resolvedThreadID)
		restrictShardNum = threadShardNum
	}
	if query.TaskName != "" {
		conditions = append(conditions, "s.task_name=?")
		args = append(args, query.TaskName)
	}
	if query.TenantID != 0 {
		conditions = append(conditions, "f.tenant_id=?")
		args = append(args, query.TenantID)
	}
	if query.OlderThan > 0 {
		conditions = append(conditions, "f.updated_at < DATE_ADD_MILLIS(NOW_UTC(), ?)")
		args = append(args, -int64(query.OlderThan/time.Millisecond))
	}
	if query.NewerThan > 0 {
		conditions = append(conditions, "f.updated_at >= DATE_ADD_MILLIS(NOW_UTC(), ?)")
		args = append(args, -int64(query.NewerThan/time.Millisecond))
	}
	// Note: query.Search is applied per-shard by searchClause, since the composed flow key
	// "{shard}-{flow_id}-{flow_token}" needs the shard literal baked into the SQL.

	joinSQL = " LEFT JOIN microbus_steps s ON f.step_id = s.step_id"
	whereSQL = strings.Join(conditions, " AND ")
	return joinSQL, whereSQL, args, restrictShardNum, nil
}

/*
Delete removes a flow and its steps from the database. The flow must not be running. Subgraph
children and thread descendants are left dangling (their parent references become stale); this
matches the framework's stance that Continue lineage is best-effort and not guaranteed across
operator-driven retention.
*/
func (svc *Service) Delete(ctx context.Context, flowKey string) (err error) { // MARKER: Delete
	shardNum, flowID, flowToken, err := parseFlowKey(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Trace(err)
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		"SELECT status FROM microbus_flows WHERE flow_id=? AND flow_token=?",
		flowID, flowToken,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return errors.New("flow not found", http.StatusNotFound)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if strings.TrimSpace(status) == workflow.StatusRunning {
		return errors.New("cannot delete a running flow; cancel it first", http.StatusConflict)
	}
	_, err = tx.ExecContext(ctx,
		"DELETE FROM microbus_steps WHERE flow_id=?",
		flowID,
	)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = tx.ExecContext(ctx,
		"DELETE FROM microbus_flows WHERE flow_id=? AND flow_token=? AND status<>?",
		flowID, flowToken, workflow.StatusRunning,
	)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(tx.Commit())
}

/*
Purge deletes flows matching the query, except those currently running. Operates per shard in
parallel; capped at 10000 flows per call. Returns the count of flows actually deleted.
*/
func (svc *Service) Purge(ctx context.Context, query foremanapi.Query) (deleted int, err error) { // MARKER: Purge
	const purgeCap = 10000
	limit := query.Limit
	if limit <= 0 || limit > purgeCap {
		limit = purgeCap
	}
	numShards := svc.numDBShards()

	joinSQL, whereSQL, baseArgs, restrictShardNum, err := svc.queryClauses(ctx, query)
	if err != nil {
		return 0, errors.Trace(err)
	}

	singleShard := restrictShardNum != 0
	perShardLimit := limit
	if !singleShard && numShards > 0 {
		perShardLimit = (limit + numShards - 1) / numShards
		if perShardLimit < 1 {
			perShardLimit = 1
		}
	}

	perShardDeleted := make([]int, numShards+1)
	purgeShard := func(shardIdx int) func() error {
		if restrictShardNum != 0 && shardIdx != restrictShardNum {
			return func() error { return nil }
		}
		return func() error {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}
			where := whereSQL
			args := append([]any(nil), baseArgs...)
			if sc, scArgs := searchClause(db.DriverName(), shardIdx, query.Search); sc != "" {
				where += " AND " + sc
				args = append(args, scArgs...)
			}
			args = append(args, workflow.StatusRunning, perShardLimit)
			selectIDs := "SELECT DISTINCT f.flow_id FROM microbus_flows f" + joinSQL +
				" WHERE " + where + " AND f.status<>? LIMIT_OFFSET(?, 0)"
			rows, err := db.QueryContext(ctx, selectIDs, args...)
			if err != nil {
				return errors.Trace(err)
			}
			var flowIDs []any
			for rows.Next() {
				var fid int
				if err := rows.Scan(&fid); err != nil {
					rows.Close()
					return errors.Trace(err)
				}
				flowIDs = append(flowIDs, fid)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return errors.Trace(err)
			}
			if len(flowIDs) == 0 {
				return nil
			}

			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return errors.Trace(err)
			}
			defer tx.Rollback()

			placeholders := strings.Repeat("?,", len(flowIDs)-1) + "?"
			_, err = tx.ExecContext(ctx,
				"DELETE FROM microbus_steps WHERE flow_id IN ("+placeholders+")",
				flowIDs...,
			)
			if err != nil {
				return errors.Trace(err)
			}
			// Re-guard against the race where a flow transitioned to running between SELECT and DELETE.
			delArgs := append([]any(nil), flowIDs...)
			delArgs = append(delArgs, workflow.StatusRunning)
			res, err := tx.ExecContext(ctx,
				"DELETE FROM microbus_flows WHERE flow_id IN ("+placeholders+") AND status<>?",
				delArgs...,
			)
			if err != nil {
				return errors.Trace(err)
			}
			n, _ := res.RowsAffected()
			perShardDeleted[shardIdx] = int(n)
			return errors.Trace(tx.Commit())
		}
	}
	jobs := make([]func() error, 0, numShards)
	for i := 1; i <= numShards; i++ {
		jobs = append(jobs, purgeShard(i))
	}
	err = svc.Parallel(jobs...)
	if err != nil {
		return 0, errors.Trace(err)
	}
	for i := 1; i <= numShards; i++ {
		deleted += perShardDeleted[i]
	}
	return deleted, nil
}

/*
Enqueue rings the work doorbell, signalling that a step is pending. This endpoint is called by foreman replicas to
distribute work across the cluster. It does not enqueue a specific step: the receiving replica looks up the announced
step's priority and not_before, and either defers to the poll timer (if not yet due) or decides via its candidate
cache whether to refill or head-insert for priority preemption.
*/
func (svc *Service) Enqueue(ctx context.Context, shard int, stepID int) (err error) { // MARKER: Enqueue
	if frame.Of(ctx).FromHost() != Hostname {
		return errors.New("enqueue is restricted to foreman replicas", http.StatusForbidden)
	}
	// Resolve the announced step's priority and not_before delay via a PK lookup
	// (off the selection hot path). A miss (step already consumed) yields MaxInt
	// priority and 0 delay, so the doorbell only wakes an idle replica and never
	// blanket-requeries a busy one. The delay is computed DB-side as (not_before
	// - NOW) to avoid Go/database clock skew.
	priority := math.MaxInt
	var notBeforeDelayMs sql.NullFloat64
	if db, derr := svc.shard(shard); derr == nil {
		db.QueryRowContext(ctx,
			"SELECT priority, DATE_DIFF_MILLIS(not_before, NOW_UTC()) FROM microbus_steps WHERE step_id=?",
			stepID,
		).Scan(&priority, &notBeforeDelayMs)
	}
	// If the step is not yet due, defer to the poll timer instead of head-inserting
	// into the cache: the work cannot run now, so there is nothing to preempt. The
	// shortened poll wakes pollPendingSteps at the right moment, which will then
	// ring the refiller's doorbell normally.
	if notBeforeDelayMs.Valid && notBeforeDelayMs.Float64 > 0 {
		wakeAt := time.Now().Add(time.Duration(notBeforeDelayMs.Float64 * float64(time.Millisecond)))
		svc.shortenNextPoll(wakeAt)
		svc.LogDebug(ctx, "Doorbell deferred", "stepID", stepID, "delayMs", notBeforeDelayMs.Float64)
		return nil
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
		j, ok, needRefill := svc.cache.pop()
		if needRefill {
			svc.requestRefill()
		}
		if !ok {
			return // Cache closed
		}
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

// timerLoop sleeps until the sooner of nextPoll / nextProbe, then calls pollPendingSteps. It
// re-evaluates whenever wakeTimer is signaled or the sleep expires, and exits when timerStop is
// closed. Capped at maxPollInterval.
func (svc *Service) timerLoop(ctx context.Context) {
	for {
		svc.nextPollLock.Lock()
		deadline := svc.nextPoll
		svc.nextPollLock.Unlock()

		// Wake for a tripped breaker's probe too, on its own deadline. Floor an overdue probe to
		// breakerInitialProbeDelay so a probe still being dispatched (selection picked it up but
		// the schedule hasn't been advanced yet by the next breakerTrip on failure) doesn't spin
		// the timer at zero delay.
		if probeNanos := svc.nextProbe.Load(); probeNanos != 0 {
			probe := time.Unix(0, probeNanos)
			if floor := time.Now().Add(breakerInitialProbeDelay); probe.Before(floor) {
				probe = floor
			}
			if probe.Before(deadline) {
				deadline = probe
			}
		}

		delay := max(0, min(time.Until(deadline), maxPollInterval))

		select {
		case <-svc.timerStop:
			return // Shutting down
		case <-time.After(delay):
		case <-svc.wakeTimer:
			continue // Nudged: re-evaluate the deadline
		}

		err := svc.pollPendingSteps(ctx)
		if err != nil {
			svc.LogError(ctx, "Polling pending steps", "error", err)
		}
	}
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
Await blocks until the flow stops (i.e. is no longer created, pending, or running), then returns
the FlowOutcome. A flow failure surfaces as outcome.Status="failed" with outcome.Error populated;
the Go-level error return is for transport/foreman/timeout failures only.
*/
func (svc *Service) Await(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error) { // MARKER: Await
	stopped := func(s string) bool {
		return s != "" && s != workflow.StatusCreated && s != workflow.StatusPending && s != workflow.StatusRunning
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
		outcome, err = svc.Snapshot(ctx, flowKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if outcome != nil && stopped(outcome.Status) {
			return outcome, nil
		}
		select {
		case <-ch:
			continue
		case <-ctx.Done():
			return nil, errors.Trace(ctx.Err(), http.StatusRequestTimeout)
		}
	}
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal FlowOutcome.
The Go-level error return is for transport/foreman/timeout failures; a workflow that fails surfaces
as outcome.Status="failed" with outcome.Error populated.
*/
func (svc *Service) Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (outcome *workflow.FlowOutcome, err error) { // MARKER: Run
	flowKey, err := svc.Create(ctx, workflowName, initialState, opts)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = svc.Start(ctx, flowKey)
	if err != nil {
		svc.Cancel(ctx, flowKey, "") // Best-effort cleanup
		return nil, errors.Trace(err)
	}
	outcome, err = svc.Await(ctx, flowKey)
	if err != nil {
		svc.Cancel(ctx, flowKey, "") // Best-effort cleanup
		return nil, errors.Trace(err)
	}
	return outcome, nil
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

/*
SyncValve receives a gossiped per-task valve state from a peer foreman replica and merges
it using the convergent register rule: latest tCong wins, smaller wCong on tie. The local
throttle's sliding-window counters are not part of the gossip - each replica's throttle is
its own per-replica state.
*/
func (svc *Service) SyncValve(ctx context.Context, taskName string, wCong int, tCong time.Time) (err error) { // MARKER: SyncValve
	if frame.Of(ctx).FromHost() != foremanapi.Hostname {
		return nil // only foreman replicas may push valves; verified source is set by the connector
	}
	if taskName == "" {
		return nil
	}
	svc.valvesLock.Lock()
	defer svc.valvesLock.Unlock()
	cur, ok := svc.valves[taskName]
	if !ok {
		svc.valves[taskName] = &taskValve{
			wCong:    wCong,
			tCong:    tCong,
			throttle: throttle.New(time.Second, math.MaxInt),
		}
		return nil
	}
	if tCong.After(cur.tCong) || (tCong.Equal(cur.tCong) && wCong < cur.wCong) {
		cur.wCong = wCong
		cur.tCong = tCong
	}
	return nil
}

// TripBreaker receives a per-task breaker trip from a peer foreman replica. Stamps the local
// clock and drives bulk-park exactly as a local trip does so this replica's view of the task's
// pending steps converges with the originating replica's. Closures are not gossiped; each peer
// closes locally when its own probe succeeds.
func (svc *Service) TripBreaker(ctx context.Context, taskName string) (err error) { // MARKER: TripBreaker
	if frame.Of(ctx).FromHost() != foremanapi.Hostname {
		return nil // gated to foreman peers; verified source is set by the connector
	}
	if taskName == "" {
		return nil
	}
	// Use breakerTrip with the gossip cause label; cause is preserved on the first replica's
	// trip and re-stamped here only if we hadn't seen this breaker yet. The "ack_timeout" label
	// is a reasonable default for the gossip path since the gossip itself doesn't carry cause.
	fresh, nextProbeAt := svc.breakerTrip(taskName, breakerCauseAckTimeout)
	if !fresh {
		return nil // already tripped here too; no fresh bulk-park needed
	}
	err = svc.breakerBulkPark(ctx, taskName, nextProbeAt)
	if err != nil {
		svc.LogError(ctx, "Bulk-park on gossip trip", "task", taskName, "error", err)
	}
	return nil
}

// OnObserveTaskRateLimit emits the current adaptive dispatch-rate ceiling per task in ops/sec.
func (svc *Service) OnObserveTaskRateLimit(ctx context.Context) (err error) { // MARKER: TaskRateLimit
	now := time.Now()
	svc.valvesLock.RLock()
	snapshot := make(map[string]*taskValve, len(svc.valves))
	for k, v := range svc.valves {
		snapshot[k] = v
	}
	svc.valvesLock.RUnlock()
	for task, v := range snapshot {
		if v.wCong == 0 {
			continue // un-anchored: throttle is counting but not gating, no meaningful "limit" to report
		}
		if err = svc.RecordTaskRateLimit(ctx, v.recoverRate(now), task); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// OnObserveTaskConcurrencyRunning emits the cluster-wide in-flight count per task.
func (svc *Service) OnObserveTaskConcurrencyRunning(ctx context.Context) (err error) { // MARKER: TaskConcurrencyRunning
	running, err := svc.countTasks(ctx, workflow.StatusRunning)
	if err != nil {
		return errors.Trace(err)
	}
	for task, count := range running {
		if err = svc.RecordTaskConcurrencyRunning(ctx, count, task); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// OnObserveTaskBreakerState emits the current breaker state per task (0=closed/admitting, 1=tripped/blocked).
func (svc *Service) OnObserveTaskBreakerState(ctx context.Context) (err error) { // MARKER: TaskBreakerState
	svc.breakersLock.RLock()
	snapshot := make(map[string]bool, len(svc.breakers))
	for k, b := range svc.breakers {
		snapshot[k] = !b.trippedAt.IsZero()
	}
	svc.breakersLock.RUnlock()
	for task, tripped := range snapshot {
		state := 0
		if tripped {
			state = 1
		}
		if err = svc.RecordTaskBreakerState(ctx, state, task); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// parseFlowKey extracts the shard, numeric flow ID and flow token from a composite flow ID string.
// Format: "{shard}-{flowID}-{token}" with a 1-based shard.
func parseFlowKey(flowKey string) (shardNum int, flowID int, flowToken string, err error) {
	parts := strings.SplitN(flowKey, "-", 3)
	if len(parts) != 3 {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	shardNum64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || shardNum64 < 1 {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	flowID64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid flow ID", http.StatusBadRequest)
	}
	return int(shardNum64), int(flowID64), parts[2], nil
}

// parseStepKey extracts the shard, numeric step ID and step token from a composite step key string.
// Format: "{shard}-{stepID}-{token}" with a 1-based shard.
func parseStepKey(stepKey string) (shardNum int, stepID int, stepToken string, err error) {
	parts := strings.SplitN(stepKey, "-", 3)
	if len(parts) != 3 {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	shardNum64, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || shardNum64 < 1 {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	stepID64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", errors.New("invalid step key", http.StatusBadRequest)
	}
	return int(shardNum64), int(stepID64), parts[2], nil
}
