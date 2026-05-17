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
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestCandidateCache_InitLowWater(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// size = 2x workers; lowWater = size/2 = workers.
	var c1 candidateCache
	c1.init(1)
	assert.Expect(c1.size, 2)
	assert.Expect(c1.capacity(), 2)
	assert.Expect(c1.lowWater, 1)
	assert.Expect(c1.floor, math.MaxInt)

	var c8 candidateCache
	c8.init(8)
	assert.Expect(c8.size, 16)
	assert.Expect(c8.lowWater, 8)

	// A zero or negative worker count clamps size to 1, never 0 (a 0 cache
	// never admits work).
	var c0 candidateCache
	c0.init(0)
	assert.Expect(c0.size, 1)
	assert.Expect(c0.lowWater, 1)
}

func TestCandidateCache_RefillPopFIFOAndFloorReset(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(4)
	c.refill([]job{{stepID: 1, shard: 0}, {stepID: 2, shard: 1}, {stepID: 3, shard: 0}}, 5)
	assert.Expect(c.len(), 3)
	assert.Expect(c.floor, 5)

	j, ok, _ := c.pop()
	assert.True(ok)
	assert.Expect(j, job{stepID: 1, shard: 0})
	j, ok, _ = c.pop()
	assert.True(ok)
	assert.Expect(j, job{stepID: 2, shard: 1})
	j, ok, _ = c.pop()
	assert.True(ok)
	assert.Expect(j, job{stepID: 3, shard: 0})

	// Drained: floor resets so the doorbell treats an empty cache as idle.
	assert.Expect(c.len(), 0)
	assert.Expect(c.floor, math.MaxInt)
}

func TestCandidateCache_NeedRefillAtLowWater(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(4) // size 8, lowWater 4
	c.refill([]job{
		{stepID: 1}, {stepID: 2}, {stepID: 3}, {stepID: 4},
		{stepID: 5}, {stepID: 6}, {stepID: 7}, {stepID: 8},
	}, 5)

	_, _, need := c.pop() // 7 remain, 7 > 4
	assert.False(need)
	_, _, need = c.pop() // 6 remain
	assert.False(need)
	_, _, need = c.pop() // 5 remain
	assert.False(need)
	_, _, need = c.pop() // 4 remain, 4 <= 4
	assert.True(need)
	_, _, need = c.pop() // 3 remain
	assert.True(need)
}

func TestCandidateCache_PopBlocksThenRefillWakes(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(2)
	done := make(chan job, 1)
	go func() {
		j, _, _ := c.pop()
		done <- j
	}()

	// The pop must block while the cache is empty.
	select {
	case <-done:
		t.Fatal("pop returned before any refill")
	case <-time.After(100 * time.Millisecond):
	}

	c.refill([]job{{stepID: 99, shard: 2}}, 7)
	select {
	case j := <-done:
		assert.Expect(j, job{stepID: 99, shard: 2})
	case <-time.After(2 * time.Second):
		t.Fatal("pop did not wake after refill")
	}
}

func TestCandidateCache_CloseUnblocksBlockedPop(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(2)
	done := make(chan bool, 1)
	go func() {
		_, ok, _ := c.pop()
		done <- ok
	}()
	select {
	case <-done:
		t.Fatal("pop returned before close")
	case <-time.After(100 * time.Millisecond):
	}

	c.close()
	select {
	case ok := <-done:
		assert.False(ok) // closed + empty -> ok=false
	case <-time.After(2 * time.Second):
		t.Fatal("close did not unblock pop")
	}
}

func TestCandidateCache_OfferEmptyWakesIdleNoInsert(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(4)
	// An empty cache means this replica is idle: request a refill so the
	// refiller selects the strictly-best pending step. Do NOT head-insert the
	// arbitrary first arrival - that could run before a more important step.
	assert.True(c.offer(job{stepID: 7, shard: 1}, 5))
	assert.Expect(c.len(), 0)
	assert.Expect(c.floor, math.MaxInt)
}

func TestCandidateCache_OfferPriorityJumpNoFlush(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(4)
	c.refill([]job{{stepID: 1}, {stepID: 2}}, 5)

	// No more urgent than what is cached: no-op, cache intact, no requery.
	assert.False(c.offer(job{stepID: 8}, 7))
	assert.False(c.offer(job{stepID: 9}, 5)) // equal is not strictly better
	assert.Expect(c.len(), 2)

	// Strictly better priority jumps that step to the HEAD without flushing the
	// existing candidates (throughput over exact ordering), lowers the floor,
	// and asks for a refill.
	assert.True(c.offer(job{stepID: 99, shard: 3}, 3))
	assert.Expect(c.len(), 3)
	assert.Expect(c.floor, 3)
	j, _, _ := c.pop()
	assert.Expect(j, job{stepID: 99, shard: 3}) // head, runs next
}

func TestCandidateCache_OfferBoundsToSize(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(1) // size 2
	c.refill([]job{{stepID: 1}, {stepID: 2}}, 5)
	// A more-urgent offer prepends and trims the tail so the cache stays bounded.
	assert.True(c.offer(job{stepID: 99}, 1))
	assert.Expect(c.len(), 2)
	j, _, _ := c.pop()
	assert.Expect(j, job{stepID: 99}) // head kept
	j, _, _ = c.pop()
	assert.Expect(j, job{stepID: 1}) // tail (stepID 2) trimmed; it stays pending in the DB
}

func TestCandidateCache_OfferClosedIsNoop(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(2)
	c.close()
	assert.False(c.offer(job{stepID: 1}, 0))
}

func TestCandidateCache_RefillEmptyOrClosedIsNoop(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var c candidateCache
	c.init(2)

	// An empty batch is a no-op: parked workers stay parked.
	c.refill(nil, 5)
	assert.Expect(c.len(), 0)
	assert.Expect(c.floor, math.MaxInt)

	// A refill into a closed cache is a harmless no-op (a refiller scan may
	// still be in flight during shutdown drain).
	c.close()
	c.refill([]job{{stepID: 1}}, 1)
	assert.Expect(c.len(), 0)
}

func TestCandidateCache_CloseZeroValueDoesNotPanic(t *testing.T) {
	t.Parallel()
	// close before init must not panic (cond is nil).
	var c candidateCache
	c.close()
}
