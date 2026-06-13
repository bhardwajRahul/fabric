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
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"
)

// Probe schedule constants for the per-task 404 ack-timeout breaker.
const (
	breakerInitialProbeDelay = 100 * time.Millisecond
	breakerProbeMultiplier   = 2.0
	breakerMaxProbeDelay     = 1 * time.Minute
)

// Cause labels for the breaker metrics. Stored on the breaker at trip time so
// probe outcomes (success/failure) carry the same cause label as the original
// trip, even when the failing cause and the closing probe are temporally far apart.
const (
	breakerCauseAckTimeout  = "ack_timeout" // 404 ack-timeout: no subscriber answered
	breakerCauseUnavailable = "unavailable" // 503 Service Unavailable: downstream not ready / in maintenance
	breakerCauseOverloaded  = "overloaded"  // 529 Site Overloaded
)

// taskBreaker is the per-task circuit breaker state. Zero trippedAt = closed (admitting).
// It trips when a task's endpoint is unreachable (404 Ack Timeout) or signals overload (529).
type taskBreaker struct {
	trippedAt    time.Time
	probeAttempt int
	nextProbeAt  time.Time
	cause        string // breakerCauseAckTimeout or breakerCauseOverloaded; zero only while closed
}

// breakerProbeBackoff returns the wait until the n-th probe attempt (1-indexed),
// capped at breakerMaxProbeDelay.
func breakerProbeBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := float64(breakerInitialProbeDelay)
	for i := 1; i < attempt; i++ {
		d *= breakerProbeMultiplier
		if d >= float64(breakerMaxProbeDelay) {
			return breakerMaxProbeDelay
		}
	}
	return time.Duration(d)
}

// breakerTrip records a trip in the local in-memory breaker map. On a fresh trip the breaker
// is armed for its first probe; on a repeat trip it is a no-op (the schedule advances at
// probe DISPATCH via breakerCommit, not at failure - matching the original design where the
// breakerAdmits gate at refill time bounded probe attempts per window). Returns whether this
// was a fresh trip and the current nextProbeAt. Pure in-memory; the caller drives the DB-side
// effects (bulk-park + probe not_before update) via breakerBulkPark.
func (svc *Service) breakerTrip(taskName, cause string) (fresh bool, nextProbeAt time.Time) {
	now := time.Now()
	svc.breakersLock.Lock()
	defer svc.breakersLock.Unlock()
	b, ok := svc.breakers[taskName]
	if !ok {
		b = &taskBreaker{}
		svc.breakers[taskName] = b
	}
	if b.trippedAt.IsZero() {
		b.trippedAt = now
		b.probeAttempt = 0
		b.nextProbeAt = now.Add(breakerProbeBackoff(1))
		b.cause = cause
		fresh = true
		svc.refreshNextProbeLocked()
	}
	return fresh, b.nextProbeAt
}

// breakerCommit advances the probe schedule and returns the new nextProbeAt. Called from
// processStep at dispatch admission, it advances only when the probe is genuinely due
// (now >= nextProbeAt); see the probe-time gate in CLAUDE.md for why not every dispatch of a
// tripped task counts as a probe. No-op when the task has no tripped breaker.
func (svc *Service) breakerCommit(taskName string) (nextProbeAt time.Time, advanced bool) {
	now := time.Now()
	svc.breakersLock.Lock()
	defer svc.breakersLock.Unlock()
	b, ok := svc.breakers[taskName]
	if !ok || b.trippedAt.IsZero() {
		return time.Time{}, false
	}
	if now.Before(b.nextProbeAt) {
		return b.nextProbeAt, false
	}
	b.probeAttempt++
	b.nextProbeAt = now.Add(breakerProbeBackoff(b.probeAttempt))
	svc.refreshNextProbeLocked()
	return b.nextProbeAt, true
}

// breakerClose flips a tripped breaker back to closed (admitting) and bulk-unparks the
// breaker-parked steps for taskName on the shard whose probe just succeeded. Local-only:
// closures are not gossiped and the unpark is shard-scoped so each shard's released backlog
// hits downstream as a rolling wave instead of a unison flood. Other shards stay parked
// until their own probes succeed (each shard's probe runs on its own dispatch schedule and
// the success/failure outcomes are independent). No-op when the breaker was already closed
// or absent.
func (svc *Service) breakerClose(ctx context.Context, taskName string, shardNum int) {
	svc.breakersLock.Lock()
	b, ok := svc.breakers[taskName]
	if !ok || b.trippedAt.IsZero() {
		svc.breakersLock.Unlock()
		return
	}
	cause := b.cause
	b.trippedAt = time.Time{}
	b.probeAttempt = 0
	b.nextProbeAt = time.Time{}
	b.cause = ""
	// This breaker may have held the soonest probe; recompute across the rest.
	svc.refreshNextProbeLocked()
	svc.breakersLock.Unlock()
	svc.IncrementTaskBreakerProbes(ctx, 1, taskName, "success", cause)
	// Unpark this shard's breaker-parked steps. Idempotent. Per-shard, symmetric with the
	// per-shard trip in breakerBulkPark.
	err := svc.breakerBulkUnpark(ctx, taskName, shardNum)
	if err != nil {
		svc.LogError(ctx, "Bulk-unpark on breaker close", "task", taskName, "shard", shardNum, "error", err)
	}
}

// BreakerTripped reports whether the breaker for taskName is currently tripped on this replica.
// Used by insert paths to decide a new step's initial parked value; also intended for fixtures.
func (svc *Service) BreakerTripped(taskName string) bool {
	svc.breakersLock.RLock()
	defer svc.breakersLock.RUnlock()
	b, ok := svc.breakers[taskName]
	if !ok {
		return false
	}
	return !b.trippedAt.IsZero()
}

// BreakerCount returns the number of tasks that currently carry a breaker entry. Intended for
// fixtures.
func (svc *Service) BreakerCount() int {
	svc.breakersLock.RLock()
	defer svc.breakersLock.RUnlock()
	return len(svc.breakers)
}

// handleBreakerTrip bounces the offending step back to pending, parked at parkedBreaker so
// the refiller cannot pick it up before bulk-park promotes a probe; records the trip
// in-memory; then bulk-parks sibling steps and elevates the per-shard oldest to parked=0 as
// the probe. Steps are never failed by this path. cause is the label
// (breakerCauseAckTimeout or breakerCauseOverloaded) that flows into the metric; the
// mechanics are identical for both.
func (svc *Service) handleBreakerTrip(ctx context.Context, shardNum, stepID int, taskName, workflowName, cause string) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	res, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, parked=?, not_before=NOW_UTC(), lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
		workflow.StatusPending, parkedBreaker, stepID, workflow.StatusRunning,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // step was cancelled / failed / completed by a concurrent path
	}
	fresh, nextProbeAt := svc.breakerTrip(taskName, cause)
	svc.IncrementTaskBreakerProbes(ctx, 1, taskName, "failure", cause)
	if fresh {
		svc.IncrementTaskBreakerTrips(ctx, 1, taskName, cause)
		foremanapi.NewMulticastClient(svc).TripBreaker(ctx, taskName)
	}
	// Retry bulk-park on lock contention so a probe is always re-elected before returning;
	// a rolled-back park-and-elevate would otherwise wedge recovery (see CLAUDE.md).
	const maxBulkParkAttempts = 5
	for attempt := 0; attempt < maxBulkParkAttempts; attempt++ {
		if attempt > 0 {
			serr := svc.Sleep(ctx, time.Duration(attempt)*time.Millisecond)
			if serr != nil {
				return errors.Trace(serr)
			}
		}
		err = svc.breakerBulkPark(ctx, taskName, nextProbeAt)
		if err == nil {
			break
		}
		if !sequel.IsLockContentionError(err) || attempt == maxBulkParkAttempts-1 {
			return errors.Trace(err)
		}
	}
	svc.LogDebug(ctx, "Task breaker tripped", "task", taskName, "step", stepID, "flow", workflowName, "cause", cause, "fresh", fresh)
	return nil
}

// breakerBulkPark ensures each shard has exactly one probe (parked=parkedNone) for taskName,
// the oldest pending step, with every other pending step held at parkedBreaker. The probe's
// not_before is set to nextProbeAt so selection naturally honors the breaker's exponential
// backoff without a separate admission filter. parkedSubgraph rows are left alone - they
// belong to a different parking regime. Park-all-then-elevate-oldest pattern: a single tx
// per shard wraps the park UPDATE, the probe SELECT, and the elevate UPDATE - the SELECT
// reads the just-parked rows, the elevate flips one of them back to parked=parkedNone, and
// concurrent Cancel/Fail paths block on the row locks until the tx commits, so the elevated
// probe is guaranteed to be in status=pending when we leave. Per-shard parallel: each shard
// leaves its own local-oldest as a probe (so in a multi-shard deployment the probe count is
// up to NumShards, not 1, but still bounded). Idempotent: multiple replicas observing the
// same trip (local or gossiped) all run this; the convergent SQL makes the result identical
// regardless of order.
func (svc *Service) breakerBulkPark(ctx context.Context, taskName string, nextProbeAt time.Time) error {
	probeBackoffMs := time.Until(nextProbeAt).Milliseconds()
	if probeBackoffMs < 0 {
		probeBackoffMs = 0
	}
	return svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errors.Trace(err)
		}
		defer tx.Rollback()
		// Park every pending step for this task to parkedBreaker (parkedSubgraph untouched).
		// Includes the previous probe (parked=parkedNone) and the previously-held backlog
		// (parked=parkedBreaker); the no-op self-update on the latter is cheap.
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET parked=?, updated_at=NOW_UTC()"+
				" WHERE task_name=? AND status=? AND parked IN (?,?)",
			parkedBreaker, taskName, workflow.StatusPending, parkedNone, parkedBreaker,
		)
		if err != nil {
			return errors.Trace(err)
		}
		// Pick this shard's probe: the oldest pending step for the task (all now at
		// parkedBreaker after the UPDATE above).
		var probeID int
		err = tx.QueryRowContext(ctx,
			"SELECT step_id FROM microbus_steps"+
				" WHERE task_name=? AND status=? AND parked=?"+
				" ORDER BY created_at ASC, step_id ASC LIMIT_OFFSET(1, 0)",
			taskName, workflow.StatusPending, parkedBreaker,
		).Scan(&probeID)
		if err == sql.ErrNoRows {
			return errors.Trace(tx.Commit()) // nothing pending for this task on this shard
		}
		if err != nil {
			return errors.Trace(err)
		}
		// Elevate the probe to parked=parkedNone and arm its not_before for the breaker's
		// next-probe deadline. The status=? guard is defensive: within this tx the row is
		// locked by the prior UPDATE so a concurrent Cancel cannot have transitioned it,
		// but the guard documents the invariant and protects against future code paths
		// that might bypass the lock.
		_, err = tx.ExecContext(ctx,
			"UPDATE microbus_steps SET parked=?, not_before=DATE_ADD_MILLIS(NOW_UTC(), ?), updated_at=NOW_UTC()"+
				" WHERE step_id=? AND status=?",
			parkedNone, probeBackoffMs, probeID, workflow.StatusPending,
		)
		if err != nil {
			return errors.Trace(err)
		}
		return errors.Trace(tx.Commit())
	})
}

// breakerBulkUnpark flips every breaker-parked step for taskName on shardNum back to
// parked=0 so it's re-eligible for selection. Called from breakerClose at the shard whose
// probe just succeeded; other shards' parked rows are untouched - they'll be released when
// their own probes succeed independently. Idempotent.
func (svc *Service) breakerBulkUnpark(ctx context.Context, taskName string, shardNum int) error {
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = db.ExecContext(ctx,
		"UPDATE microbus_steps SET parked=?, updated_at=NOW_UTC() WHERE task_name=? AND parked=?",
		parkedNone, taskName, parkedBreaker,
	)
	return errors.Trace(err)
}

// reconstituteBreakers re-arms the in-memory breaker map after a restart by scanning each shard
// for tasks that still have parked=parkedBreaker rows. For each such task it calls breakerTrip
// locally - schedule starts fresh on this replica (probeAttempt=0, first probe at now+100ms)
// regardless of where peers are in their own schedules. The DB-side state (parked rows + the
// elevated probe per shard) is already correct from the prior replica's breakerBulkPark, so no
// SQL writes are needed. Local-only: does NOT multicast TripBreaker - peers already have their
// own in-memory state and the gossip merge would clobber their accumulated probeAttempt with
// our fresh "now" timestamp. Cause defaults to ack_timeout (the most common reason rows survive
// a restart); a wrong label only affects the first probe-failure metric until the breaker
// re-trips and records the actual cause.
func (svc *Service) reconstituteBreakers(ctx context.Context) error {
	return svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		rows, err := db.QueryContext(ctx,
			"SELECT DISTINCT task_name FROM microbus_steps WHERE parked=? AND status=?",
			parkedBreaker, workflow.StatusPending,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		for rows.Next() {
			var taskName string
			if err := rows.Scan(&taskName); err != nil {
				return errors.Trace(err)
			}
			svc.breakerTrip(taskName, breakerCauseAckTimeout)
		}
		return errors.Trace(rows.Err())
	})
}

// refreshNextProbeLocked recomputes svc.nextProbe as the soonest probe across all tripped breakers
// (UnixNano; 0 when none is tripped) and wakes the timer to re-evaluate. The breaker subsystem owns
// nextProbe end to end: pollPendingSteps never touches it, so the backlog-cadence reset of nextPoll
// cannot clobber a pending probe. Called only on the infrequent trip/close transitions, so
// the O(breakers) scan stays off the poll hot path. Caller must hold breakersLock.
func (svc *Service) refreshNextProbeLocked() {
	var soonest int64
	for _, b := range svc.breakers {
		if b.trippedAt.IsZero() {
			continue
		}
		n := b.nextProbeAt.UnixNano()
		if soonest == 0 || n < soonest {
			soonest = n
		}
	}
	svc.nextProbe.Store(soonest)
	svc.signalTimer()
}
