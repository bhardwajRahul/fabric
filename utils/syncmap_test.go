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

package utils

import (
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestUtils_SyncMapStoreLoadDelete(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var sm SyncMap[string, int]

	// Load and Delete on an empty map.
	_, ok := sm.Load("a")
	assert.False(ok)
	assert.Zero(sm.Len())
	_, deleted := sm.Delete("a")
	assert.False(deleted)

	// Store then Load.
	sm.Store("a", 1)
	v, ok := sm.Load("a")
	assert.True(ok)
	assert.Equal(1, v)
	assert.Equal(1, sm.Len())

	// Store overwrites.
	sm.Store("a", 2)
	v, _ = sm.Load("a")
	assert.Equal(2, v)
	assert.Equal(1, sm.Len())

	// Delete returns the removed value.
	v, deleted = sm.Delete("a")
	assert.True(deleted)
	assert.Equal(2, v)
	_, ok = sm.Load("a")
	assert.False(ok)
	assert.Zero(sm.Len())
}

func TestUtils_SyncMapLoadOrStore(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var sm SyncMap[string, int]

	// Stores when absent.
	actual, loaded := sm.LoadOrStore("a", 1)
	assert.False(loaded)
	assert.Equal(1, actual)

	// Loads when present and does not overwrite.
	actual, loaded = sm.LoadOrStore("a", 99)
	assert.True(loaded)
	assert.Equal(1, actual)
	v, _ := sm.Load("a")
	assert.Equal(1, v)
}

func TestUtils_SyncMapLoadOrStoreFunc(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var sm SyncMap[string, int]

	calls := 0
	makeVal := func() int { calls++; return 7 }

	// Stores and invokes the factory exactly once.
	actual, loaded := sm.LoadOrStoreFunc("a", makeVal)
	assert.False(loaded)
	assert.Equal(7, actual)
	assert.Equal(1, calls)

	// Present: returns the existing value and does not invoke the factory again.
	actual, loaded = sm.LoadOrStoreFunc("a", makeVal)
	assert.True(loaded)
	assert.Equal(7, actual)
	assert.Equal(1, calls)
}

func TestUtils_SyncMapKeysValuesSnapshot(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var sm SyncMap[string, int]
	sm.Store("a", 1)
	sm.Store("b", 2)
	sm.Store("c", 3)

	keys := sm.Keys()
	assert.Len(keys, 3)
	for _, k := range []string{"a", "b", "c"} {
		assert.Contains(keys, k)
	}

	values := sm.Values()
	assert.Len(values, 3)
	for _, v := range []int{1, 2, 3} {
		assert.Contains(values, v)
	}

	// Snapshot is a detached copy: mutating it does not affect the map.
	snap := sm.Snapshot()
	assert.Len(snap, 3)
	assert.Equal(2, snap["b"])
	snap["b"] = 200
	v, _ := sm.Load("b")
	assert.Equal(2, v)
}

func TestUtils_SyncMapDoUnderLock(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var sm SyncMap[string, int]
	sm.Store("a", 1)

	sm.DoUnderLock(func(m map[string]int) {
		m["a"] = 10 // mutate existing
		m["b"] = 20 // add new
	})

	v, _ := sm.Load("a")
	assert.Equal(10, v)
	v, _ = sm.Load("b")
	assert.Equal(20, v)
	assert.Equal(2, sm.Len())
}

func TestUtils_SyncMapConcurrent(t *testing.T) {
	t.Parallel()
	// Run under -race: many goroutines mutating and reading concurrently must not race.
	var sm SyncMap[int, int]
	var wg sync.WaitGroup

	// Per-key churn: each goroutine owns a disjoint key range and runs the full lifecycle.
	for g := range runtime.NumCPU() * 4 {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := range 1000 {
				k := base*1000 + i
				sm.Store(k, k)
				sm.LoadOrStore(k, k)
				sm.LoadOrStoreFunc(k, func() int { return k })
				sm.Load(k)
				sm.Delete(k)
			}
		}(g)
	}

	// Whole-map operations racing the per-key churn.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				_ = sm.Keys()
				_ = sm.Values()
				_ = sm.Snapshot()
				_ = sm.Len()
				sm.DoUnderLock(func(m map[int]int) { _ = len(m) })
			}
		}()
	}
	wg.Wait()
}

// The connector's in-flight request registry (connector.reqs) is a SyncMap keyed by a short message id. Per
// request it does one LoadOrStore (insert on send), one Load (lookup on the inbound response), and one Delete
// (on completion). These benchmarks measure that op mix under the single mutex the review flagged, on a map
// pre-populated to a realistic in-flight population. Keys are precomputed so the hot loop allocates nothing and
// the numbers reflect the map/mutex, not string building.

const benchSyncMapMask = 4096 - 1

func benchSyncMapKeys() []string {
	keys := make([]string, benchSyncMapMask+1)
	for i := range keys {
		keys[i] = "msg-" + strconv.Itoa(i)
	}
	return keys
}

// BenchmarkSyncMap_RequestLifecycle runs one full request through the registry: LoadOrStore + Load + Delete.
func BenchmarkSyncMap_RequestLifecycle(b *testing.B) {
	var sm SyncMap[string, int]
	keys := benchSyncMapKeys()
	for i, k := range keys {
		sm.Store(k, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := keys[i&benchSyncMapMask]
		sm.LoadOrStore(k, i)
		sm.Load(k)
		sm.Delete(k)
	}
	// Apple M1 Pro, 4096 entries: ~64 ns/op, 0 allocs (~15.6M lifecycles/sec). The registry is nowhere near a
	// bottleneck at any realistic request rate.
}

// BenchmarkSyncMap_RequestLifecycleParallel measures the same mix under contention: every goroutine takes the
// one mutex, which is the ceiling the review flagged.
func BenchmarkSyncMap_RequestLifecycleParallel(b *testing.B) {
	var sm SyncMap[string, int]
	keys := benchSyncMapKeys()
	for i, k := range keys {
		sm.Store(k, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := keys[i&benchSyncMapMask]
			sm.LoadOrStore(k, i)
			sm.Load(k)
			sm.Delete(k)
			i++
		}
	})
	// Apple M1 Pro, -cpu 8: ~503 ns/op (~2M full lifecycles/sec aggregate under 8-way contention). Even fully
	// contended the registry sustains millions of requests/sec, orders of magnitude above any real request rate,
	// so the single mutex is not a practical bottleneck.
}

// BenchmarkSyncMap_Load isolates the inbound-response lookup, the single hottest op.
func BenchmarkSyncMap_Load(b *testing.B) {
	var sm SyncMap[string, int]
	keys := benchSyncMapKeys()
	for i, k := range keys {
		sm.Store(k, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Load(keys[i&benchSyncMapMask])
	}
	// Apple M1 Pro: ~14.5 ns/op, 0 allocs (~69M lookups/sec).
}
