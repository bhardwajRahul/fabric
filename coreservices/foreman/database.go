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
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

)

// shard returns the database connection for the given 1-based shard index. Indices outside
// [1, numShards] return a "flow not found" error.
func (svc *Service) shard(n int) (*sequel.DB, error) {
	svc.dbsLock.RLock()
	defer svc.dbsLock.RUnlock()
	if n < 1 || n > len(svc.dbs) {
		return nil, errors.New("flow not found", http.StatusNotFound)
	}
	return svc.dbs[n-1], nil
}

// numDBShards returns the current number of database shards. Valid shard indices are 1..n.
func (svc *Service) numDBShards() int {
	svc.dbsLock.RLock()
	n := len(svc.dbs)
	svc.dbsLock.RUnlock()
	return n
}

// eachShard fans op out over every shard concurrently. op is invoked once per shard, receiving
// the resolved database handle and the 1-based shard index. Any shard's error fails the whole
// call (svc.Parallel returns the first non-nil error). The foreman is not shard-fault-tolerant
// by design; a single-shard outage degrades every cross-shard operation, and the operation's
// caller is expected to retry on its next cycle (timer poll, refill, observer scrape, etc.).
//
// Concurrency contract: op for shard N must only write to slot N of any per-shard accumulator
// the caller captured. Disjoint-slot writes are safe without locking; shared state across shards
// (e.g. an aggregate minimum) requires the caller's own synchronization.
func (svc *Service) eachShard(ctx context.Context, op func(ctx context.Context, db *sequel.DB, shard int) error) (err error) {
	numShards := svc.numDBShards()
	if numShards == 1 {
		db, err := svc.shard(1)
		if err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(op(ctx, db, 1))
	}
	jobs := make([]func() error, 0, numShards)
	for i := 1; i <= numShards; i++ {
		si := i
		jobs = append(jobs, func() error {
			db, err := svc.shard(si)
			if err != nil {
				return errors.Trace(err)
			}
			return op(ctx, db, si)
		})
	}
	err = svc.Parallel(jobs...)
	return errors.Trace(err)
}

// openDatabase opens connections to all database shards and runs migrations. Shards are
// 1-indexed throughout the framework; `%d` in the DSN expands to the 1-based index.
func (svc *Service) openDatabase(ctx context.Context) (err error) {
	numShards := svc.NumShards()
	dataSourceName := svc.SQLDataSourceName()
	if svc.Deployment() == connector.TESTING && dataSourceName == "" {
		dataSourceName = "file:shard_%d" // SQLite
	}
	if numShards > 1 && !strings.Contains(dataSourceName, "%d") {
		return errors.New("SQLDataSourceName must contain %%d when NumShards > 1")
	}
	for i := 1; i <= numShards; i++ {
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
//
// Uses sequel.Open (not OpenSingleton) so the foreman gets a dedicated *sql.DB
// per shard with its own connection pool. The pool size is then set explicitly
// from SQLConnectionPool below; if a co-bundled microservice also opens the
// same DSN via OpenSingleton, sequel's sqrt-based auto-sizing for that
// singleton does not touch this *DB's pool.
//
// In TESTING, CreateTestingDatabase materializes the per-test database (one
// DROP + CREATE per unique test id) and returns its resolved DSN; Open then
// gives the foreman its own pool against that database.
func (svc *Service) openDatabaseShard(ctx context.Context, dataSourceName string, shardIndex int) (db *sequel.DB, err error) {
	_ = ctx
	const driverName = "" // Inferred from the data source name
	dsn := dataSourceName
	if strings.Contains(dataSourceName, "%d") {
		dsn = fmt.Sprintf(dataSourceName, shardIndex)
	}
	if svc.Deployment() == connector.TESTING {
		dsn, err = sequel.CreateTestingDatabase(driverName, dsn, svc.Plane())
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	db, err = sequel.Open(driverName, dsn)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Workers hold a connection only during the brief DB-only segments of
	// processStep (the rest of the time they're awaiting an HTTP task call),
	// so the pool need not match the worker count - but keeping idle == max
	// avoids the close/reopen churn that dominates when idle < max under load.
	// The default of 8 is empirically the knee of the throughput-vs-connections
	// curve for typical workloads.
	poolSize := svc.SQLConnectionPool()
	db.SetMaxOpenConns(poolSize)
	db.SetMaxIdleConns(poolSize)
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

// pollPendingSteps recovers steps whose crash-recovery lease has expired, detects orphaned flows, sizes the wake
// timer to the nearest future not_before, and rings the local refiller doorbell. It no longer enumerates the backlog
// onto a queue: selection is the refiller's job, so the poll only recovers and nudges.
//
// Any shard's error fails the whole poll cycle; the timer loop retries on its next tick.
func (svc *Service) pollPendingSteps(ctx context.Context) error {
	var mu sync.Mutex
	var nearestDelay time.Duration = -1

	err := svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) (err error) {
		// Recover steps stuck in running whose lease has expired
		res, err := db.ExecContext(ctx,
			"UPDATE microbus_steps SET status=?, updated_at=NOW_UTC() WHERE status=? AND lease_expires<=NOW_UTC()",
			workflow.StatusPending, workflow.StatusRunning,
		)
		if err != nil {
			return errors.Trace(err)
		}
		if recovered, _ := res.RowsAffected(); recovered > 0 {
			svc.IncrementStepsRecovered(ctx, int(recovered))
		}

		// Size the wake timer to the nearest future-due pending step (flow.Sleep / retry
		// backoff). Steps due now are picked up by the refiller via the doorbell rung below,
		// so only future delays matter here. Computed as an aggregate to avoid scanning the
		// backlog; delay is (not_before - NOW) to avoid Go/database clock skew.
		var nearestMs sql.NullFloat64
		err = db.QueryRowContext(ctx,
			"SELECT DATE_DIFF_MILLIS(MIN(not_before), NOW_UTC()) FROM microbus_steps"+
				" WHERE status=? AND not_before>NOW_UTC() AND not_before<=DATE_ADD_MILLIS(NOW_UTC(), ?) AND lease_expires<=NOW_UTC()",
			workflow.StatusPending, maxPollInterval.Milliseconds(),
		).Scan(&nearestMs)
		if err != nil {
			return errors.Trace(err)
		}
		var shardNearestDelay time.Duration = -1
		if nearestMs.Valid && nearestMs.Float64 > 0 {
			shardNearestDelay = time.Duration(nearestMs.Float64 * float64(time.Millisecond))
		}
		// If a due pending backlog exists on this shard, keep the poll cadence
		// tight so an idle replica re-scans soon even with no doorbell.
		var dueExists sql.NullInt64
		err = db.QueryRowContext(ctx,
			"SELECT 1 FROM microbus_steps WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() LIMIT_OFFSET(1, 0)",
			workflow.StatusPending,
		).Scan(&dueExists)
		if err != nil && err != sql.ErrNoRows {
			return errors.Trace(err)
		}
		if err != sql.ErrNoRows && (shardNearestDelay < 0 || shardNearestDelay > backlogPollInterval) {
			shardNearestDelay = backlogPollInterval
		}
		err = nil

		// Detect orphaned flows: status=running but no non-terminal steps exist anywhere
		// in the flow. This should never happen in steady state - it indicates a bug
		// (e.g. a failure between marking the last step completed and inserting next steps).
		// We log only; auto-recovery is intentionally not implemented because it would have
		// to duplicate the fan-in/transition logic and risks double-advancement on a flow
		// that was actually mid-transaction. The threshold is well past any normal
		// transient state and avoids log noise during the microsecond windows of fan-in.
		const orphanFlowThresholdMs = -5 * 60 * 1000 // 5 minutes back
		orphanRows, err := db.QueryContext(ctx,
			"SELECT flow_id, workflow_name FROM microbus_flows f"+
				" WHERE status=? AND updated_at <= DATE_ADD_MILLIS(NOW_UTC(), ?)"+
				" AND NOT EXISTS ("+
				"   SELECT 1 FROM microbus_steps s"+
				"   WHERE s.flow_id = f.flow_id AND s.status IN (?, ?, ?, ?)"+
				" )",
			workflow.StatusRunning, orphanFlowThresholdMs,
			workflow.StatusPending, workflow.StatusRunning, workflow.StatusCreated, workflow.StatusInterrupted,
		)
		if err != nil {
			return errors.Trace(err)
		}
		for orphanRows.Next() {
			var orphanFlowID int
			var orphanWorkflow string
			err := orphanRows.Scan(&orphanFlowID, &orphanWorkflow)
			if err != nil {
				orphanRows.Close()
				return errors.Trace(err)
			}
			svc.LogError(ctx, "Orphaned flow detected: status=running but no non-terminal steps",
				"flow", orphanFlowID,
				"workflow", orphanWorkflow,
				"shard", shard,
			)
		}
		orphanRows.Close()
		err = orphanRows.Err()
		if err != nil {
			return errors.Trace(err)
		}

		mu.Lock()
		if shardNearestDelay >= 0 && (nearestDelay < 0 || shardNearestDelay < nearestDelay) {
			nearestDelay = shardNearestDelay
		}
		mu.Unlock()
		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}

	// Wake up next to process the nearest future-due step. Take min(current,
	// proposed) to preserve a shorter wakeup set by shortenNextPoll (e.g. the
	// breaker's next probe time), but always advance past "now" so we don't
	// busy-loop on an already-expired deadline.
	now := time.Now()
	var proposed time.Time
	if nearestDelay >= 0 {
		proposed = now.Add(nearestDelay)
	} else {
		proposed = now.Add(maxPollInterval)
	}
	svc.nextPollLock.Lock()
	if svc.nextPoll.Before(now) || proposed.Before(svc.nextPoll) {
		svc.nextPoll = proposed
	}
	svc.nextPollLock.Unlock()

	// Ring the local doorbell so recovered or freshly-due steps are selected
	// promptly. Coalesced and gated, so this is cheap and a no-op when idle.
	svc.requestRefill()
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
