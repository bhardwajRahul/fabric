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

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// flowSchedule carries the resolved priority and fairness of a flow, denormalized
// onto the flow row and every one of its step rows.
type flowSchedule struct {
	priority       int
	fairnessKey    string
	fairnessWeight float64
}

// resolveFlowOptions resolves caller-supplied flow options against the foreman's
// defaults: priority falls back to the DefaultPriority config, the fairness key
// falls back to the tid/tenant actor claim then the "" bucket, and the fairness
// weight falls back to 1.
func (svc *Service) resolveFlowOptions(ctx context.Context, opts *workflow.FlowOptions) flowSchedule {
	sched := flowSchedule{priority: svc.DefaultPriority(), fairnessWeight: 1}
	if opts != nil {
		if opts.Priority > 0 {
			sched.priority = opts.Priority
		}
		if opts.FairnessWeight > 0 {
			sched.fairnessWeight = opts.FairnessWeight
		}
		sched.fairnessKey = opts.FairnessKey
	}
	if sched.fairnessKey == "" {
		if tid, _ := frame.Of(ctx).Tenant(); tid != 0 {
			sched.fairnessKey = strconv.Itoa(tid)
		}
	}
	return sched
}

// runRefill performs the two-level priority+fairness selection and replaces the
// candidate cache with a fresh fair batch. It runs only on the single refiller
// goroutine, so at most one selection scan is ever in flight per replica.
//
// Each shard is scanned independently (independent databases) for its own
// strict-minimum priority band's due pending rows. The shards' rows are then
// aggregated into one population: the global minimum band is taken across
// shards (strict priority is cluster-wide, not per shard), and only rows at
// that global band participate - shards whose band is worse contribute nothing
// this batch. Over that single population, repeatedly weighted-random pick a
// fairness key (Efraimidis-Spirakis over the keys, not the rows) and take that
// key's globally-oldest remaining step, until the batch is full or exhausted.
// Re-picking per step makes the expected dispatch share proportional to weight
// and independent of backlog depth or shard layout - fairness is over
// fairness_key, never over shards. "Oldest" is by created_at (step_id is
// per-shard and not comparable across shards); ties broken by (shard, step_id).
//
// Acquisition stays the atomic CAS in processStep, so the cache only reorders;
// it never grants ownership.
func (svc *Service) runRefill(ctx context.Context) error {
	type candRow struct {
		stepID int
		shard  int
		key    string
		weight float64
		ageMs  float64 // DATE_DIFF_MILLIS(NOW, created_at); larger = older
	}
	type shardResult struct {
		band int
		rows []candRow
	}

	numShards := svc.numDBShards()
	results := make([]*shardResult, numShards)
	jobs := make([]func() error, numShards)
	for i := range jobs {
		shardIdx := i
		jobs[i] = func() (err error) {
			db, err := svc.shard(shardIdx)
			if err != nil {
				return errors.Trace(err)
			}
			// One statement: the subquery selects the strict-minimum priority band
			// over due pending steps, the outer reads that band's candidate rows
			// (every returned row's priority equals the band). The subquery and
			// outer are self-consistent within the statement. It is still not
			// transactional vs concurrent worker CAS claims (a step may be
			// claimed right after this read), which is self-correcting via the
			// post-completion refill request and the backlog poll, so no
			// transaction is needed. When nothing is due, MIN is NULL, the
			// priority= comparison is UNKNOWN, and zero rows return. created_at is
			// read as an age (ms) so it is comparable across shards: it both fixes
			// each fairness_key's weight from that key's oldest step and orders
			// dispatch oldest-first within the key (step_id is per-shard only).
			rows, err := db.QueryContext(ctx,
				"SELECT step_id, fairness_key, fairness_weight, priority, DATE_DIFF_MILLIS(NOW_UTC(), created_at) FROM microbus_steps"+
					" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC()"+
					" AND priority=(SELECT MIN(priority) FROM microbus_steps"+
					" WHERE status=? AND not_before<=NOW_UTC() AND lease_expires<=NOW_UTC())"+
					" ORDER BY step_id",
				foremanapi.StatusPending, foremanapi.StatusPending,
			)
			if err != nil {
				return errors.Trace(err)
			}
			defer rows.Close()
			var sr *shardResult
			for rows.Next() {
				var c candRow
				var prio int
				if err := rows.Scan(&c.stepID, &c.key, &c.weight, &prio, &c.ageMs); err != nil {
					return errors.Trace(err)
				}
				if c.weight <= 0 {
					c.weight = 1
				}
				c.shard = shardIdx
				if sr == nil {
					sr = &shardResult{band: prio} // every row's priority == the band (subquery MIN)
				}
				sr.rows = append(sr.rows, c)
			}
			if err := rows.Err(); err != nil {
				return errors.Trace(err)
			}
			if sr != nil {
				results[shardIdx] = sr // nothing due => sr nil => shard contributes nothing
			}
			return nil
		}
	}
	err := svc.Parallel(jobs...)
	if err != nil {
		return errors.Trace(err)
	}

	// Aggregate across shards. Strict priority is cluster-wide: take the global
	// minimum band; shards whose band is worse contribute nothing this batch
	// (their lower-priority work waits). Only rows at the global band form the
	// fairness population.
	globalBand := math.MaxInt
	for _, sr := range results {
		if sr != nil && len(sr.rows) > 0 && sr.band < globalBand {
			globalBand = sr.band
		}
	}
	if globalBand == math.MaxInt {
		svc.cache.refill(nil, math.MaxInt) // nothing due on any shard (no-op)
		return nil
	}

	// One global fairness-key population, built only from rows at the global
	// minimum band (worse bands contribute nothing - see above). A key's weight
	// is fixed by its globally-oldest step (largest ageMs), so a tenant cannot
	// self-promote by submitting newer high-weight tasks; that same oldest step
	// is also dispatched first (FIFO within the key, see the sort below).
	type keyBucket struct {
		weight    float64
		oldestAge float64
		steps     []candRow
	}
	byKey := map[string]*keyBucket{}
	order := []string{}
	for _, sr := range results {
		if sr == nil || sr.band != globalBand {
			continue
		}
		for _, c := range sr.rows {
			kb := byKey[c.key]
			if kb == nil {
				kb = &keyBucket{weight: c.weight, oldestAge: c.ageMs}
				byKey[c.key] = kb
				order = append(order, c.key)
			} else if c.ageMs > kb.oldestAge {
				kb.oldestAge = c.ageMs
				kb.weight = c.weight // the globally-oldest step fixes the key's weight
			}
			kb.steps = append(kb.steps, c)
		}
	}
	for _, kb := range byKey {
		// FIFO within a fairness key: oldest step first, across all shards.
		// ageMs = DATE_DIFF_MILLIS(NOW_UTC(), created_at) is computed per shard
		// with both terms on that shard's own clock, so the per-shard offset
		// cancels and ageMs is directly comparable across shards. step_id is
		// per-shard and cannot order this. Same-age ties break by
		// (shard, step_id) for determinism.
		sort.Slice(kb.steps, func(a, b int) bool {
			x, y := kb.steps[a], kb.steps[b]
			if x.ageMs != y.ageMs {
				return x.ageMs > y.ageMs // oldest (largest age) first
			}
			if x.shard != y.shard {
				return x.shard < y.shard
			}
			return x.stepID < y.stepID
		})
	}
	distinctKeys := len(order)
	svc.lastDistinctKeys.Store(int64(distinctKeys))
	svc.LogDebug(ctx, "Refill selecting", "band", globalBand, "distinctKeys", distinctKeys)

	// Weighted-random key pick (Efraimidis-Spirakis over the keys, re-rolled per
	// step) over the single global population; take the chosen key's oldest
	// remaining step (FIFO within the key). Fairness is over fairness_key, not
	// shards.
	capacity := svc.cache.capacity()
	batch := make([]job, 0, capacity)
	for len(batch) < capacity {
		bestKey, bestScore := "", -1.0
		for _, k := range order {
			kb := byKey[k]
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
			break // all keys exhausted
		}
		kb := byKey[bestKey]
		c := kb.steps[0]
		kb.steps = kb.steps[1:]
		batch = append(batch, job{stepID: c.stepID, shard: c.shard})
	}

	svc.LogDebug(ctx, "Refill batch", "size", len(batch), "floor", globalBand)
	svc.cache.refill(batch, globalBand)
	return nil
}
