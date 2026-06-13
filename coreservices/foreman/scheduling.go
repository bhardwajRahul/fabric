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
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/sequel"

)

// resolveFlowOptions applies the foreman's defaults to caller-supplied options and returns a
// populated copy.
func (svc *Service) resolveFlowOptions(ctx context.Context, opts *workflow.FlowOptions) *workflow.FlowOptions {
	resolved := &workflow.FlowOptions{Priority: svc.DefaultPriority(), FairnessWeight: 1}
	if opts != nil {
		if opts.Priority > 0 {
			resolved.Priority = opts.Priority
		}
		if opts.FairnessWeight > 0 {
			resolved.FairnessWeight = opts.FairnessWeight
		}
		resolved.FairnessKey = opts.FairnessKey
		resolved.StartAt = opts.StartAt
	}
	if resolved.FairnessKey == "" {
		if tid, _ := frame.Of(ctx).Tenant(); tid != 0 {
			resolved.FairnessKey = strconv.Itoa(tid)
		}
	}
	return resolved
}

// candidateRow is a candidate step considered for admission.
type candidateRow struct {
	stepID int
	shard  int
	task   string
	key    string
	weight float64
	ageMs  float64 // DATE_DIFF_MILLIS(NOW, created_at); larger = older
}

// scanPriorityBand returns the rows of the next priority band above prevBand, summed
// across shards. Returns math.MaxInt when no work remains. Any shard's error fails the whole
// call; the refiller retries on its next cycle.
func (svc *Service) scanPriorityBand(ctx context.Context, prevBand int) (int, []candidateRow, error) {
	type shardResult struct {
		band int
		rows []candidateRow
	}
	numShards := svc.numDBShards()
	results := make([]*shardResult, numShards+1)
	err := svc.eachShard(ctx, func(ctx context.Context, db *sequel.DB, shard int) error {
		rows, err := db.QueryContext(ctx,
			"SELECT step_id, task_name, fairness_key, fairness_weight, priority, DATE_DIFF_MILLIS(NOW_UTC(), created_at) FROM microbus_steps"+
				" WHERE status=? AND parked=0 AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() AND priority>?"+
				" AND priority=(SELECT MIN(priority) FROM microbus_steps"+
				" WHERE status=? AND parked=0 AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC() AND priority>?)"+
				" ORDER BY step_id",
			workflow.StatusPending, prevBand, workflow.StatusPending, prevBand,
		)
		if err != nil {
			return errors.Trace(err)
		}
		defer rows.Close()
		var sr *shardResult
		for rows.Next() {
			var c candidateRow
			var prio int
			err := rows.Scan(&c.stepID, &c.task, &c.key, &c.weight, &prio, &c.ageMs)
			if err != nil {
				return errors.Trace(err)
			}
			if c.weight <= 0 {
				c.weight = 1
			}
			c.shard = shard
			if sr == nil {
				sr = &shardResult{band: prio} // every row's priority == the band (subquery MIN)
			}
			sr.rows = append(sr.rows, c)
		}
		err = rows.Err()
		if err != nil {
			return errors.Trace(err)
		}
		if sr != nil {
			results[shard] = sr
		}
		return nil
	})
	if err != nil {
		return 0, nil, errors.Trace(err)
	}
	globalBand := math.MaxInt
	for _, sr := range results {
		if sr != nil && len(sr.rows) > 0 && sr.band < globalBand {
			globalBand = sr.band
		}
	}
	if globalBand == math.MaxInt {
		return globalBand, nil, nil
	}
	var atBand []candidateRow
	for _, sr := range results {
		if sr == nil || sr.band != globalBand {
			continue
		}
		atBand = append(atBand, sr.rows...)
	}
	return globalBand, atBand, nil
}

// runRefill replaces the candidate cache with a fresh priority+fairness batch
// honoring per-task adaptive dispatch-rate limits. See "Adaptive Per-Task
// Concurrency" in coreservices/foreman/CLAUDE.md.
func (svc *Service) runRefill(ctx context.Context) error {
	now := time.Now()
	capacity := svc.cache.capacity()
	batch := make([]job, 0, capacity)

	// valveDropped is set when any candidate row was refused by valvePeek (rate-limited task,
	// throttle full this window). At the end of the refill, if true, we advance the next poll to
	// the throttle window boundary so the foreman wakes when the throttle has rotated. Without
	// this, valve-saturated work would wait the full backlogPollInterval (default 2s) instead of
	// the throttle window (1s).
	valveDropped := false
	// Breaker admission is no longer a per-pop predicate: parked rows are physically excluded
	// from idx_microbus_steps_selection. On trip, breakerBulkPark moves all but one step for the
	// task to parked=2 and sets the surviving probe's not_before to the schedule, so the
	// candidate scan never sees the held-back backlog.
	admittable := func(task string) bool {
		if !svc.valvePeek(task, now) {
			valveDropped = true
			return false
		}
		return true
	}

	// Walk priority bands ascending. Exits when scanPriorityBand returns MaxInt
	// (no more pending work anywhere) or admittable work is found at the
	// current band. Per refill, worst case is O(distinct priority values)
	// scanPriorityBand calls, each one a parallel shard query on the saturation
	// index.
	prevBand := -1
	chosenBand := math.MaxInt
	for {
		band, rows, err := svc.scanPriorityBand(ctx, prevBand)
		if err != nil {
			return errors.Trace(err)
		}
		if band == math.MaxInt {
			break // nothing due on any shard
		}
		// Group by fairness_key; key weight is fixed by its oldest member;
		// rows whose task has no headroom drop out, taking empty keys with them.
		type keyBucket struct {
			weight    float64
			oldestAge float64
			steps     []candidateRow
		}
		byKey := map[string]*keyBucket{}
		order := []string{}
		dropped := 0
		for _, c := range rows {
			if !admittable(c.task) {
				dropped++
				svc.IncrementStepsSkippedSaturated(ctx, 1, c.task)
				continue
			}
			kb := byKey[c.key]
			if kb == nil {
				kb = &keyBucket{weight: c.weight, oldestAge: c.ageMs}
				byKey[c.key] = kb
				order = append(order, c.key)
			} else if c.ageMs > kb.oldestAge {
				kb.oldestAge = c.ageMs
				kb.weight = c.weight
			}
			kb.steps = append(kb.steps, c)
		}
		if len(byKey) == 0 {
			svc.LogDebug(ctx, "Refill band saturated, advancing", "band", band, "rows", len(rows))
			prevBand = band
			continue
		}
		// FIFO within key: oldest first; ties broken by (shard, step_id).
		for _, kb := range byKey {
			sort.Slice(kb.steps, func(a, b int) bool {
				x, y := kb.steps[a], kb.steps[b]
				if x.ageMs != y.ageMs {
					return x.ageMs > y.ageMs
				}
				if x.shard != y.shard {
					return x.shard < y.shard
				}
				return x.stepID < y.stepID
			})
		}
		distinctKeys := len(order)
		// Clear the previous band's series if the band changed, so an inactive band doesn't show stale counts on the dashboard.
		if svc.lastRefillPriority != 0 && svc.lastRefillPriority != band {
			_ = svc.RecordStepsFairnessKeys(ctx, 0, strconv.Itoa(svc.lastRefillPriority))
		}
		_ = svc.RecordStepsFairnessKeys(ctx, distinctKeys, strconv.Itoa(band))
		svc.lastRefillPriority = band
		svc.LogDebug(ctx, "Refill selecting", "band", band, "distinctKeys", distinctKeys, "saturatedDrops", dropped)

		// Weighted-random key pick (Efraimidis-Spirakis), re-rolled per step.
		for len(batch) < capacity {
			bestKey, bestScore := "", -1.0
			for _, k := range order {
				kb := byKey[k]
				// Skip past head steps whose task has hit its in-batch cap.
				// A fairness key (per-tenant) may carry steps for multiple tasks;
				// saturating one task must not block admissible work for another task in the same bucket.
				for len(kb.steps) > 0 && !admittable(kb.steps[0].task) {
					svc.IncrementStepsSkippedSaturated(ctx, 1, kb.steps[0].task)
					kb.steps = kb.steps[1:]
				}
				if len(kb.steps) == 0 {
					continue
				}
				score := math.Pow(rand.Float64(), 1/kb.weight)
				if score > bestScore {
					bestScore = score
					bestKey = k
				}
			}
			if bestScore < 0 {
				break
			}
			kb := byKey[bestKey]
			c := kb.steps[0]
			kb.steps = kb.steps[1:]
			batch = append(batch, job{stepID: c.stepID, shard: c.shard})
			// Consume one throttle slot for this dispatch (no-op when the task has no valve recorded yet).
			svc.valveCommit(c.task, now)
		}
		chosenBand = band
		break
	}

	// If no band was chosen (system idle or every band fully saturated), clear the previously
	// recorded series so the dashboard returns to 0 instead of showing a stale count.
	if chosenBand == math.MaxInt && svc.lastRefillPriority != 0 {
		_ = svc.RecordStepsFairnessKeys(ctx, 0, strconv.Itoa(svc.lastRefillPriority))
		svc.lastRefillPriority = 0
	}
	svc.LogDebug(ctx, "Refill batch", "size", len(batch), "floor", chosenBand)
	svc.cache.refill(batch, chosenBand)
	if valveDropped {
		// At least one candidate was refused by a rate-limited valve; the throttle's window
		// rotates within 1 second, so wake the poll at that boundary rather than waiting for the
		// default backlogPollInterval. shortenNextPoll only advances; sibling tasks whose work is
		// driven by the completion doorbell are unaffected.
		svc.shortenNextPoll(now.Add(time.Second))
	}
	return nil
}
