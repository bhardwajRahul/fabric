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
	"github.com/microbus-io/sequel"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

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

// pollPendingSteps recovers steps whose crash-recovery lease has expired, detects orphaned flows, sizes the wake
// timer to the nearest future not_before, and rings the local refiller doorbell. It no longer enumerates the backlog
// onto a queue: selection is the refiller's job, so the poll only recovers and nudges.
func (svc *Service) pollPendingSteps(ctx context.Context) error {
	var mu sync.Mutex
	var nearestDelay time.Duration = -1

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

			// Size the wake timer to the nearest future-due pending step (flow.Sleep / retry
			// backoff). Steps due now are picked up by the refiller via the doorbell rung below,
			// so only future delays matter here. Computed as an aggregate to avoid scanning the
			// backlog; delay is (not_before - NOW) to avoid Go/database clock skew.
			var nearestMs sql.NullFloat64
			err = db.QueryRowContext(ctx,
				"SELECT DATE_DIFF_MILLIS(MIN(not_before), NOW_UTC()) FROM microbus_steps"+
					" WHERE status=? AND not_before>NOW_UTC() AND not_before<=DATE_ADD_MILLIS(NOW_UTC(), ?) AND lease_expires<=NOW_UTC()",
				foremanapi.StatusPending, maxPollInterval.Milliseconds(),
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
				foremanapi.StatusPending,
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
				foremanapi.StatusRunning, orphanFlowThresholdMs,
				foremanapi.StatusPending, foremanapi.StatusRunning, foremanapi.StatusCreated, foremanapi.StatusInterrupted,
			)
			if err != nil {
				return errors.Trace(err)
			}
			for orphanRows.Next() {
				var orphanFlowID int
				var orphanWorkflow string
				if err := orphanRows.Scan(&orphanFlowID, &orphanWorkflow); err != nil {
					orphanRows.Close()
					return errors.Trace(err)
				}
				svc.LogError(ctx, "Orphaned flow detected: status=running but no non-terminal steps",
					"flow", orphanFlowID,
					"workflow", orphanWorkflow,
					"shard", shardIdx,
				)
			}
			orphanRows.Close()
			if err := orphanRows.Err(); err != nil {
				return errors.Trace(err)
			}

			mu.Lock()
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

	// Wake up next to process the nearest future-due step
	now := time.Now()
	svc.nextPollLock.Lock()
	if nearestDelay >= 0 {
		svc.nextPoll = now.Add(nearestDelay)
	} else {
		svc.nextPoll = now.Add(maxPollInterval)
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
