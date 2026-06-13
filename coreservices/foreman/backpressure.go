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
	"math"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/sequel"
	"github.com/microbus-io/throttle"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
)

// taskValve is the per-task controller state. wCong (ops/sec) and tCong are gossiped via
// SyncValve; the throttle is per-replica. wCong == 0 means no 429 has anchored yet - the
// throttle counts dispatches but does not gate them.
type taskValve struct {
	wCong    int
	tCong    time.Time
	throttle *throttle.Throttle
}

// recoverRate is the TCP CUBIC recovery curve:
// w(t) = cubicC*(t-K)^3 + wMax, K = cbrt(wMax*cubicBeta/cubicC), wMax = wCong/(1-cubicBeta).
// Clamped to [1, MaxInt].
func (v *taskValve) recoverRate(now time.Time) int {
	const cubicBeta = 0.01 // wMax = wCong / (1 - cubicBeta); recovery target is just above wCong
	const cubicC = 0.05    // CUBIC curve coefficient; K ~1.7s at wMax=25

	elapsed := now.Sub(v.tCong).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	wMax := float64(v.wCong) / (1 - cubicBeta)
	k := math.Cbrt(wMax * cubicBeta / cubicC)
	delta := elapsed - k
	w := cubicC*delta*delta*delta + wMax
	if w >= float64(math.MaxInt) {
		return math.MaxInt
	}
	return max(int(w), 1)
}

// valvePeek reports whether the task is currently admissible without consuming a slot.
// Unanchored tasks (no valve, or wCong == 0) admit unconditionally.
func (svc *Service) valvePeek(taskName string, now time.Time) bool {
	svc.valvesLock.RLock()
	v, ok := svc.valves[taskName]
	svc.valvesLock.RUnlock()
	if !ok || v.wCong == 0 {
		return true
	}
	v.throttle.SetLimit(v.recoverRate(now))
	admit, _ := v.throttle.Peek()
	return admit
}

// valveCommit consumes a throttle slot, creating the valve lazily on first dispatch.
func (svc *Service) valveCommit(taskName string, now time.Time) {
	svc.valvesLock.Lock()
	v, ok := svc.valves[taskName]
	if !ok {
		v = &taskValve{throttle: throttle.New(time.Second, math.MaxInt32)}
		svc.valves[taskName] = v
	}
	svc.valvesLock.Unlock()
	if v.wCong > 0 {
		v.throttle.SetLimit(v.recoverRate(now))
	}
	v.throttle.Allow()
}

// handleBackpressure responds to a 429: regulate the valve, bounce the step back to pending
// with no not_before delay. Steps are never failed by this path.
func (svc *Service) handleBackpressure(ctx context.Context, shardNum, stepID int, taskName, workflowName string) error {
	observed := svc.valveRegulate(ctx, taskName)
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
	svc.LogDebug(ctx, "Task backpressured (429)", "task", taskName, "step", stepID, "flow", workflowName, "observedRate", observed)
	svc.shortenNextPoll(time.Now())
	return nil
}

// valveRegulate cuts wCong by 1 (re-anchored to throttle.observed at the start of a fresh
// burst) and gossips the new point. Returns observed, for logging. Invariant: only called
// from handleBackpressure, so the valve already exists.
func (svc *Service) valveRegulate(ctx context.Context, taskName string) int {
	now := time.Now()
	svc.valvesLock.Lock()
	v := svc.valves[taskName]
	_, observed := v.throttle.Peek()
	if v.tCong.IsZero() || now.Sub(v.tCong) > time.Second {
		if observed > v.wCong {
			v.wCong = observed
		}
	}
	v.wCong = max(v.wCong-1, 1)
	v.tCong = now
	newW := v.wCong
	svc.valvesLock.Unlock()
	v.throttle.SetLimit(v.recoverRate(now))
	svc.IncrementTaskRateCuts(ctx, 1, taskName)
	foremanapi.NewMulticastClient(svc).SyncValve(ctx, taskName, newW, now) // Gossip to peers
	return observed
}

// countTasks sums rows in the given status grouped by task_name across every shard in parallel.
// Feeds the TaskConcurrencyRunning observable gauge; off the admission hot path. Parked steps
// (parked!=0) are excluded - the saturation index physically partitions them out, and they do
// not consume an executing slot. Any shard's failure fails the whole call - the foreman is not
// shard-fault-tolerant.
func (svc *Service) countTasks(ctx context.Context, status string) (map[string]int, error) {
	numShards := svc.numDBShards()
	perShard := make([]map[string]int, numShards+1)
	err := svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		rows, err := db.QueryContext(ctx,
			"SELECT task_name, COUNT(*) FROM microbus_steps WHERE status=? AND parked=0 GROUP BY task_name",
			status,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		m := map[string]int{}
		for rows.Next() {
			var name string
			var count int
			err := rows.Scan(&name, &count)
			if err != nil {
				return errors.Trace(err)
			}
			m[name] = count
		}
		err = rows.Err()
		if err != nil {
			return errors.Trace(err)
		}
		perShard[shard] = m
		return nil
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	sum := map[string]int{}
	for i := 1; i <= numShards; i++ {
		for name, c := range perShard[i] {
			sum[name] += c
		}
	}
	return sum, nil
}

// ValveCount returns the number of tasks that have a valve allocated. Intended for fixtures.
func (svc *Service) ValveCount() int {
	svc.valvesLock.RLock()
	defer svc.valvesLock.RUnlock()
	return len(svc.valves)
}
