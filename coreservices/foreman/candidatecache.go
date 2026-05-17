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
	"math"
	"sync"
)

// job holds a step ID and its shard index for the worker pool.
type job struct {
	stepID int
	shard  int
}

// candidateCache is a small, per-replica bounded set of step candidates produced
// by the refiller's two-level priority+fairness selection. It is NOT a work
// queue: it holds hints, not ownership. Workers pop a candidate and atomically
// CAS-acquire the step before executing, so a stale or duplicated candidate is
// harmless (the loser of the CAS simply pops the next one).
//
// All candidates in the cache come from a single refill batch and therefore
// share the selection's priority band, so floor is single-valued for the batch.
// The doorbell uses floor for priority-floor invalidation: announced work more
// urgent than the cache's floor flushes the whole cache.
//
// init must be called before any pop/refill/doorbell/len; only close tolerates a
// zero-value cache.
type candidateCache struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    []job
	floor    int // best (lowest) priority represented; math.MaxInt when empty
	size     int // capacity, equal to the worker count
	lowWater int // pop below this requests a refill so draining overlaps refill
	closed   bool
}

// init sizes the cache to twice the worker count, with the low-water refill
// mark at half of that (i.e. the worker count). The 2x buffer keeps workers fed
// while the refiller scans, and refilling once the cache drains to one worker
// count remaining overlaps refilling with draining.
func (c *candidateCache) init(workers int) {
	c.cond = sync.NewCond(&c.mu)
	c.items = nil
	c.floor = math.MaxInt
	c.size = max(1, 2*workers)
	c.lowWater = max(1, c.size/2)
	c.closed = false
}

// capacity is the cache's bound, used as the refiller's batch size so a refill
// fills toward the full 2x buffer. Set once in init and never mutated, so it is
// read without the lock.
func (c *candidateCache) capacity() int {
	return c.size
}

// pop removes and returns the candidate at the head of the cache. It blocks
// until a candidate is available or the cache is closed. Returns ok=false if
// the cache is closed and empty. needRefill is true when the cache has drained
// to or below the low-water mark (or emptied), so the caller should ask the
// refiller to top it up; the caller must invoke the refill request outside any
// cache lock.
func (c *candidateCache) pop() (j job, ok bool, needRefill bool) {
	c.mu.Lock()
	for len(c.items) == 0 && !c.closed {
		c.cond.Wait()
	}
	if len(c.items) == 0 {
		c.mu.Unlock()
		return job{}, false, false
	}
	j = c.items[0]
	c.items = c.items[1:]
	if len(c.items) == 0 {
		c.items = nil
		c.floor = math.MaxInt
	}
	needRefill = len(c.items) <= c.lowWater
	c.mu.Unlock()
	return j, true, needRefill
}

// refill replaces the cache contents with a freshly selected fair batch and its
// priority floor (the lowest priority in the batch), then wakes waiting workers.
// An empty batch is a no-op so parked workers stay parked until real work
// exists. A push into a closed cache is a harmless no-op.
func (c *candidateCache) refill(batch []job, floor int) {
	c.mu.Lock()
	if c.closed || len(batch) == 0 {
		c.mu.Unlock()
		return
	}
	c.items = batch
	c.floor = floor
	c.mu.Unlock()
	c.cond.Broadcast()
}

// offer is the doorbell: it reacts to a step becoming pending and returns
// whether the caller should ask the refiller to run.
//
//   - Empty cache: this replica is idle. Return true so the refiller runs and
//     selects the strictly-best pending step; do NOT head-insert whatever rang
//     first, which could let an arbitrary-priority step run before a more
//     important one. The idle-wake then costs one refiller scan, not zero, in
//     exchange for not inverting priority.
//   - Non-empty and the announced priority is not strictly more important than
//     the cached band (priority >= floor, including equal): no-op, return
//     false. A steady same-or-lower-priority stream is pure cache hits with no
//     blanket requery.
//   - Non-empty and strictly more important (priority < floor): jump that exact
//     step to the head so the next pop runs it without waiting a refiller scan,
//     lower the floor, wake one waiter, and return true to also top up the rest
//     of the band. This is the valuable case - an urgent arrival preempting
//     cached lower-priority work - and it does not flush: the existing
//     candidates stay so workers keep busy rather than idle through the refill
//     (throughput is favored over exact ordering, which is soft across replicas
//     anyway). The cache is bound to size by trimming the tail; a trimmed step
//     stays pending and is re-selected. The CAS in processStep still arbitrates
//     acquisition, so a stale or duplicate head entry is harmless.
func (c *candidateCache) offer(j job, priority int) (needRefill bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	if len(c.items) == 0 {
		return true
	}
	if priority >= c.floor {
		return false
	}
	c.items = append([]job{j}, c.items...)
	if len(c.items) > c.size {
		c.items = c.items[:c.size]
	}
	c.floor = priority
	c.cond.Signal()
	return true
}

// len returns the number of candidates currently cached.
func (c *candidateCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// close wakes all waiting workers so they can exit.
func (c *candidateCache) close() {
	if c.cond == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.cond.Broadcast()
}
