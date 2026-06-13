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
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

// TestBreaker_ReconstituteRearmsFromParkedRows verifies that an empty in-memory breaker map
// is correctly rebuilt by scanning microbus_steps for parked=parkedBreaker rows. This is the
// startup-recovery path: when a foreman restarts, prior parked rows would otherwise be
// invisible to selection and the in-memory breaker would not know to schedule probes.
func TestBreaker_ReconstituteRearmsFromParkedRows(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)

	// No breakers tripped at startup (clean DB).
	assert.Expect(svc.BreakerTripped("stuck.taskA"), false)
	assert.Expect(svc.BreakerCount(), 0)

	// Simulate a previous replica's state: parked-breaker rows for two distinct tasks.
	db, err := svc.shard(1)
	if !assert.NoError(err) {
		return
	}
	insertParked := func(token, taskName string) error {
		_, err := db.ExecContext(ctx,
			"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, parked, time_budget_ms, lease_expires, not_before, priority, fairness_key, fairness_weight)"+
				" VALUES (?, 1, ?, ?, '{}', ?, ?, 60000, NOW_UTC(), NOW_UTC(), 1, '', 1)",
			1, token, taskName, workflow.StatusPending, parkedBreaker,
		)
		return err
	}
	if !assert.NoError(insertParked("a1", "stuck.taskA")) {
		return
	}
	if !assert.NoError(insertParked("a2", "stuck.taskA")) {
		return
	}
	if !assert.NoError(insertParked("b1", "stuck.taskB")) {
		return
	}

	// Now reconstitute. Both tasks should be armed locally.
	if !assert.NoError(svc.reconstituteBreakers(ctx)) {
		return
	}
	assert.True(svc.BreakerTripped("stuck.taskA"))
	assert.True(svc.BreakerTripped("stuck.taskB"))
	assert.Expect(svc.BreakerCount(), 2)

	// The DB rows are untouched - reconstitution does not write.
	var parkedA, parkedB int
	assert.NoError(db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM microbus_steps WHERE task_name=? AND parked=?",
		"stuck.taskA", parkedBreaker,
	).Scan(&parkedA))
	assert.Expect(parkedA, 2)
	assert.NoError(db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM microbus_steps WHERE task_name=? AND parked=?",
		"stuck.taskB", parkedBreaker,
	).Scan(&parkedB))
	assert.Expect(parkedB, 1)

	// The schedule starts fresh on this replica (probeAttempt=0, first probe at ~now+100ms).
	svc.breakersLock.RLock()
	a := svc.breakers["stuck.taskA"]
	svc.breakersLock.RUnlock()
	assert.True(a != nil)
	if a != nil {
		assert.Expect(a.probeAttempt, 0)
		eta := time.Until(a.nextProbeAt)
		assert.True(eta > 0 && eta <= breakerInitialProbeDelay+50*time.Millisecond)
	}
}

// TestBreaker_ReconstituteIsIdempotent verifies that calling reconstituteBreakers a second
// time on an already-armed breaker is a no-op (does not roll back the schedule). breakerTrip's
// own no-op-on-already-tripped guard owns this; the test pins the property at the
// reconstitution layer to catch a future refactor that bypasses the guard.
func TestBreaker_ReconstituteIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)

	db, err := svc.shard(1)
	if !assert.NoError(err) {
		return
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, parked, time_budget_ms, lease_expires, not_before, priority, fairness_key, fairness_weight)"+
			" VALUES (?, 1, ?, ?, '{}', ?, ?, 60000, NOW_UTC(), NOW_UTC(), 1, '', 1)",
		1, "tok", "stuck.task", workflow.StatusPending, parkedBreaker,
	)
	if !assert.NoError(err) {
		return
	}

	if !assert.NoError(svc.reconstituteBreakers(ctx)) {
		return
	}
	svc.breakersLock.RLock()
	firstProbeAt := svc.breakers["stuck.task"].nextProbeAt
	svc.breakersLock.RUnlock()

	// Advance the breaker's schedule via a probe commit, then reconstitute again. The second
	// reconstitution must not roll back the advanced schedule. breakerCommit only advances once the
	// scheduled probe is due, so wait out the initial probe delay before committing.
	time.Sleep(breakerInitialProbeDelay + 10*time.Millisecond)
	svc.breakerCommit("stuck.task")
	svc.breakersLock.RLock()
	advancedProbeAt := svc.breakers["stuck.task"].nextProbeAt
	advancedAttempt := svc.breakers["stuck.task"].probeAttempt
	svc.breakersLock.RUnlock()
	assert.True(advancedProbeAt.After(firstProbeAt))
	assert.Expect(advancedAttempt, 1)

	if !assert.NoError(svc.reconstituteBreakers(ctx)) {
		return
	}
	svc.breakersLock.RLock()
	afterReconstitute := svc.breakers["stuck.task"]
	svc.breakersLock.RUnlock()
	// Schedule is preserved: probeAttempt did not reset, nextProbeAt was not rolled back.
	assert.Expect(afterReconstitute.probeAttempt, advancedAttempt)
	assert.Expect(afterReconstitute.nextProbeAt, advancedProbeAt)
}

// TestBreaker_ReconstituteSkipsTerminalRows confirms the SELECT only picks up rows that are
// genuinely held back. Terminal/non-pending rows must not contribute to the breaker map.
func TestBreaker_ReconstituteSkipsTerminalRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)

	db, err := svc.shard(1)
	if !assert.NoError(err) {
		return
	}
	// A parked-breaker row in a non-pending status should be ignored. The terminal-implies-
	// parkedNone invariant says such a row shouldn't exist in practice, but the SELECT's
	// status filter is the second line of defense.
	_, err = db.ExecContext(ctx,
		"INSERT INTO microbus_steps (flow_id, step_depth, step_token, task_name, state, status, parked, time_budget_ms, lease_expires, not_before, priority, fairness_key, fairness_weight)"+
			" VALUES (?, 1, ?, ?, '{}', ?, ?, 60000, NOW_UTC(), NOW_UTC(), 1, '', 1)",
		1, "tok", "wrongly.parked.task", workflow.StatusCompleted, parkedBreaker,
	)
	if !assert.NoError(err) {
		return
	}

	if !assert.NoError(svc.reconstituteBreakers(ctx)) {
		return
	}
	assert.Expect(svc.BreakerTripped("wrongly.parked.task"), false)
	assert.Expect(svc.BreakerCount(), 0)
}
