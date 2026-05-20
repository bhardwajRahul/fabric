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
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// TestBackpressure_FirstCutAnchorsAndDecrements verifies the first 429 of a
// fresh burst: it snaps wCong to the throttle's observed emission rate, then
// decrements by 1. The throttle is primed with N dispatches via commitValve
// before any 429 fires, so by the time regulate runs Peek() reports the true
// emission rate.
func TestBackpressure_FirstCutAnchorsAndDecrements(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)
	const taskName = "task.first.cut"

	now := time.Now()
	for range 1000 {
		svc.valveCommit(taskName, now)
	}

	observed := svc.valveRegulate(ctx, taskName)
	assert.True(observed >= 990 && observed <= 1010)
	svc.valvesLock.RLock()
	v, ok := svc.valves[taskName]
	svc.valvesLock.RUnlock()
	assert.True(ok)
	// Fresh burst: wCong snapped to observed, then decremented by 1.
	assert.Equal(observed-1, v.wCong)
	// And the throttle's limit was tightened to the new wCong (peerCount=1
	// in test, so per-replica share equals cluster-wide rate).
	assert.Equal(observed-1, v.throttle.Limit())
}

// TestBackpressure_BurstCompoundsLinearly verifies the additive-decrease
// semantic: N 429s in a tight burst within one throttle window produce a
// cumulative cut of N from the observed rate. The first regulate snaps and
// decrements; subsequent regulates in the same burst keep decrementing.
func TestBackpressure_BurstCompoundsLinearly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)
	const taskName = "task.burst.linear"

	now := time.Now()
	for range 1000 {
		svc.valveCommit(taskName, now)
	}

	// First regulate snaps + decrements.
	observed := svc.valveRegulate(ctx, taskName)
	svc.valvesLock.RLock()
	w0 := svc.valves[taskName].wCong
	svc.valvesLock.RUnlock()
	assert.Equal(observed-1, w0)

	// 50 more regulates in tight succession (same burst, tCong stays recent).
	// Each one decrements wCong by 1, floored at minLimit.
	for range 50 {
		svc.valveRegulate(ctx, taskName)
	}
	svc.valvesLock.RLock()
	w1 := svc.valves[taskName].wCong
	svc.valvesLock.RUnlock()
	// 51 total decrements from observed (the first snap-then-decrement
	// counts as one decrement off observed).
	assert.Equal(observed-51, w1)
}

// TestBackpressure_BurstFloorsAtMinLimit verifies that a burst large enough
// to drive wCong below the floor stops at minLimit rather than going to zero
// (or negative).
func TestBackpressure_BurstFloorsAtMinLimit(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	assert := testarossa.For(t)
	const taskName = "task.burst.floor"

	now := time.Now()
	for range 10 {
		svc.valveCommit(taskName, now)
	}

	// Burst of 100 regulates against an observed of 10 floors well past 0.
	for range 100 {
		svc.valveRegulate(ctx, taskName)
	}
	svc.valvesLock.RLock()
	v := svc.valves[taskName]
	svc.valvesLock.RUnlock()
	assert.Equal(1, v.wCong)
}

// TestBackpressure_ConvergentMerge verifies the latest-tCong-wins / smaller-wCong-on-tie
// merge rule applied by SyncValve. The merge is the convergent-register CRDT
// that makes per-task valve gossip safe across reorder, duplication, and loss.
func TestBackpressure_ConvergentMerge(t *testing.T) {
	t.Parallel()

	svc := NewService()
	app := application.New()
	app.Add(svc)
	app.RunInTest(t)

	// SyncValve gates on the verified source being a foreman replica. The
	// connector's HTTP path sets this from the wire; sub-tests set it directly.
	t.Run("later_tCong_wins", func(t *testing.T) {
		assert := testarossa.For(t)
		const task = "merge.later"
		t0 := time.Now().UTC().Truncate(time.Millisecond)
		t1 := t0.Add(time.Second)

		ctx := frame.ContextWithFrame(t.Context())
		frame.Of(ctx).SetFromHost(foremanapi.Hostname)

		// Seed (wCong=5, t0), then push (wCong=100, t1) - later time even though
		// the wCong is larger; the convergent rule must adopt the newer point.
		assert.NoError(svc.SyncValve(ctx, task, 5, t0))
		assert.NoError(svc.SyncValve(ctx, task, 100, t1))

		svc.valvesLock.RLock()
		v, ok := svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)
		assert.Equal(100, v.wCong)
		assert.True(v.tCong.Equal(t1))
	})

	t.Run("tie_smaller_w_wins", func(t *testing.T) {
		assert := testarossa.For(t)
		const task = "merge.tie"
		t0 := time.Now().UTC().Truncate(time.Millisecond)

		ctx := frame.ContextWithFrame(t.Context())
		frame.Of(ctx).SetFromHost(foremanapi.Hostname)

		// Same tCong, smaller wCong on the second arrival - tie-break adopts it.
		assert.NoError(svc.SyncValve(ctx, task, 10, t0))
		assert.NoError(svc.SyncValve(ctx, task, 5, t0))

		svc.valvesLock.RLock()
		v, ok := svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)
		assert.Equal(5, v.wCong)
		assert.True(v.tCong.Equal(t0))

		// And the opposite direction is rejected (smaller-wCong-wins is one-way).
		assert.NoError(svc.SyncValve(ctx, task, 99, t0))
		svc.valvesLock.RLock()
		v, ok = svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)
		assert.Equal(5, v.wCong) // still 5
	})

	t.Run("idempotent", func(t *testing.T) {
		assert := testarossa.For(t)
		const task = "merge.idempotent"
		t0 := time.Now().UTC().Truncate(time.Millisecond)

		ctx := frame.ContextWithFrame(t.Context())
		frame.Of(ctx).SetFromHost(foremanapi.Hostname)

		assert.NoError(svc.SyncValve(ctx, task, 42, t0))
		svc.valvesLock.RLock()
		v1, ok := svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)

		// Replay the same arrival - the merge rule keeps the same point.
		assert.NoError(svc.SyncValve(ctx, task, 42, t0))
		svc.valvesLock.RLock()
		v2, ok := svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)
		assert.Equal(v1.wCong, v2.wCong)
		assert.True(v1.tCong.Equal(v2.tCong))
	})

	t.Run("older_tCong_rejected", func(t *testing.T) {
		assert := testarossa.For(t)
		const task = "merge.older.rejected"
		t0 := time.Now().UTC().Truncate(time.Millisecond)
		tEarlier := t0.Add(-time.Second)

		ctx := frame.ContextWithFrame(t.Context())
		frame.Of(ctx).SetFromHost(foremanapi.Hostname)

		// Establish a current point, then push an older-tCong arrival even with a
		// much smaller wCong; the convergent rule must keep the newer point.
		assert.NoError(svc.SyncValve(ctx, task, 50, t0))
		assert.NoError(svc.SyncValve(ctx, task, 1, tEarlier))

		svc.valvesLock.RLock()
		v, ok := svc.valves[task]
		svc.valvesLock.RUnlock()
		assert.True(ok)
		assert.Equal(50, v.wCong)
		assert.True(v.tCong.Equal(t0))
	})
}

