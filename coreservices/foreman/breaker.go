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
	"time"

	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
)

// Probe schedule constants for the per-task 404 ack-timeout breaker.
const (
	breakerInitialProbeDelay = 100 * time.Millisecond
	breakerProbeMultiplier   = 2.0
	breakerMaxProbeDelay     = 1 * time.Minute
)

// taskBreaker is the per-task circuit breaker state. Zero trippedAt = closed (admitting).
// It trips when a task's endpoints is unreachable (404 Ack Timeout).
type taskBreaker struct {
	trippedAt    time.Time
	probeAttempt int
	nextProbeAt  time.Time
}

// breakerAdmits reports whether the breaker for taskName admits one more
// dispatch this refill cycle. Read-only; does NOT advance the probe schedule.
// Use breakerProbeCommitted after actually selecting a tripped-task step.
// alreadyProbed is true once any step of this task has been admitted this
// refill, so the gate caps a tripped task at one probe per cycle. runRefill
// calls admittable twice per refill (once filtering rows into fairness
// buckets, once re-checking at bucket head); a side-effecting predicate would
// advance the schedule on the first call and then reject itself on the
// second.
func (svc *Service) breakerAdmits(taskName string, alreadyProbed bool, now time.Time) bool {
	svc.breakersLock.RLock()
	defer svc.breakersLock.RUnlock()
	b, ok := svc.breakers[taskName]
	if !ok || b.trippedAt.IsZero() {
		return true
	}
	if alreadyProbed || now.Before(b.nextProbeAt) {
		return false
	}
	return true
}

// breakerCommit advances the probe schedule (probeAttempt and
// nextProbeAt) after a probe step has been added to the batch and wakes the
// poll timer at the new nextProbeAt. A no-op when the task has no tripped breaker.
func (svc *Service) breakerCommit(taskName string, now time.Time) {
	svc.breakersLock.Lock()
	b, ok := svc.breakers[taskName]
	if !ok || b.trippedAt.IsZero() {
		svc.breakersLock.Unlock()
		return
	}
	b.probeAttempt++
	b.nextProbeAt = now.Add(breakerProbeBackoff(b.probeAttempt))
	nextProbe := b.nextProbeAt
	svc.breakersLock.Unlock()
	svc.shortenNextPoll(nextProbe)
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

// breakerTrip trips taskName's breaker (if not already), arms the first
// probe, gossips the trip, and increments the trips metric. Returns true on a fresh trip.
func (svc *Service) breakerTrip(ctx context.Context, taskName string) (tripped bool) {
	now := time.Now()
	svc.breakersLock.Lock()
	b, ok := svc.breakers[taskName]
	if !ok {
		b = &taskBreaker{}
		svc.breakers[taskName] = b
	}
	if !b.trippedAt.IsZero() {
		svc.breakersLock.Unlock()
		return false
	}
	b.trippedAt = now
	b.probeAttempt = 0
	b.nextProbeAt = now.Add(breakerProbeBackoff(1))
	firstProbeAt := b.nextProbeAt
	svc.breakersLock.Unlock()
	svc.IncrementTaskBreakerTrips(ctx, 1, taskName)
	foremanapi.NewMulticastClient(svc).TripBreaker(ctx, taskName)
	// Wake the poll timer at the first probe time so the schedule fires
	// promptly regardless of the default poll cadence.
	svc.shortenNextPoll(firstProbeAt)
	return true
}

// breakerClose flips a tripped breaker back to closed (admitting). Local-only;
// closures are not gossiped. No-op when the breaker was already closed or absent.
func (svc *Service) breakerClose(ctx context.Context, taskName string) {
	svc.breakersLock.Lock()
	b, ok := svc.breakers[taskName]
	if !ok || b.trippedAt.IsZero() {
		svc.breakersLock.Unlock()
		return
	}
	b.trippedAt = time.Time{}
	b.probeAttempt = 0
	b.nextProbeAt = time.Time{}
	svc.breakersLock.Unlock()
	svc.IncrementTaskBreakerProbes(ctx, 1, taskName, "success")
}

// handleAckTimeout bounces the offending step back to pending with no added
// delay and trips the breaker. Wakes the poll timer at the next-probe moment
// so the breaker's exponential schedule is honored regardless of the default
// poll cadence. Steps are never failed by this path.
func (svc *Service) handleAckTimeout(ctx context.Context, shardNum, stepID int, taskName, workflowName string) error {
	tripped := svc.breakerTrip(ctx, taskName)
	svc.IncrementTaskBreakerProbes(ctx, 1, taskName, "failure")
	db, err := svc.shard(shardNum)
	if err != nil {
		return errors.Trace(err)
	}
	res, err := db.ExecContext(ctx,
		"UPDATE microbus_steps SET status=?, not_before=NOW_UTC(), lease_expires=NOW_UTC(), updated_at=NOW_UTC() WHERE step_id=? AND status=?",
		workflow.StatusPending, stepID, workflow.StatusRunning,
	)
	if err != nil {
		return errors.Trace(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // step was cancelled / failed / completed by a concurrent path
	}
	svc.LogDebug(ctx, "Task ack timeout (404)", "task", taskName, "step", stepID, "flow", workflowName, "tripped", tripped)
	return nil
}

// BreakerCount returns the number of tasks that currently carry a
// breaker entry. Intended for fixtures.
func (svc *Service) BreakerCount() int {
	svc.breakersLock.RLock()
	defer svc.breakersLock.RUnlock()
	return len(svc.breakers)
}

// BreakerTripped reports whether the breaker for taskName is currently
// tripped on this replica. Intended for fixtures.
func (svc *Service) BreakerTripped(taskName string) bool {
	svc.breakersLock.RLock()
	defer svc.breakersLock.RUnlock()
	b, ok := svc.breakers[taskName]
	if !ok {
		return false
	}
	return !b.trippedAt.IsZero()
}
