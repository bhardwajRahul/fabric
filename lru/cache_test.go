/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package lru

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestLRU_Load(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[string, string](1024, time.Hour)
	cache.Store("a", "aaa")
	cache.Store("b", "bbb")
	cache.Store("c", "ccc")
	tt.True(integrity(cache))

	v, ok := cache.Load("a")
	tt.True(ok)
	tt.Equal("aaa", v)

	v, ok = cache.Load("b")
	tt.True(ok)
	tt.Equal("bbb", v)

	v, ok = cache.Load("c")
	tt.True(ok)
	tt.Equal("ccc", v)

	v, ok = cache.Load("d")
	tt.False(ok)
	tt.Equal("", v)

	m := cache.ToMap()
	tt.NotEqual(0, len(m["a"]))
	tt.NotEqual(0, len(m["b"]))
	tt.NotEqual(0, len(m["c"]))
	tt.Zero(len(m["d"]))

	tt.True(integrity(cache))
}

func TestLRU_LoadOrStore(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[string, string](1024, time.Hour)
	cache.Store("a", "aaa")

	v, found := cache.LoadOrStore("a", "AAA")
	tt.True(found)
	tt.Equal("aaa", v)

	cache.Delete("a")

	v, found = cache.LoadOrStore("a", "AAA")
	tt.False(found)
	tt.Equal("AAA", v)

	v, found = cache.Load("a")
	tt.True(found)
	tt.Equal("AAA", v)

	tt.True(integrity(cache))
}

func TestLRU_MaxWeight(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	maxWt := 16
	cache := New[int, string](maxWt, time.Hour)

	cache.Store(999, "Too Big", Weight(maxWt+1))
	_, ok := cache.Load(999)
	tt.False(ok)
	tt.Zero(cache.Weight())

	// Fill in the cache
	// head> 16 15 14 13 12 11 10 9 8 7 6 5 4 3 2 1 <tail
	for i := 1; i <= maxWt; i++ {
		cache.Store(i, "Light", Weight(1))
	}
	for i := 1; i <= maxWt; i++ {
		tt.True(cache.Exists(i), "%d", i)
	}
	tt.Equal(maxWt, cache.Weight())

	// One more element causes an eviction
	// head> 101 16 15 14 13 12 11 10 9 8 7 6 5 4 3 2 <tail
	cache.Store(101, "Light", Weight(1))
	tt.False(cache.Exists(1), "%d", 1)
	for i := 2; i <= maxWt; i++ {
		tt.True(cache.Exists(i), "%d", i)
	}
	tt.True(cache.Exists(101), "%d", 101)
	tt.Equal(maxWt, cache.Weight())

	// Heavy element will cause eviction of 2 elements
	// head> 103! 101 16 15 14 13 12 11 10 9 8 7 6 5 4 <tail
	cache.Store(103, "Heavy", Weight(2))
	for i := 1; i < 3; i++ {
		tt.False(cache.Exists(i), "%d", i)
	}
	for i := 4; i <= maxWt; i++ {
		tt.True(cache.Exists(i), "%d", i)
	}
	tt.True(cache.Exists(101), "%d", 101)
	tt.True(cache.Exists(103), "%d", 103)
	tt.Equal(maxWt, cache.Weight())

	// Super heavy element will cause eviction of 5 elements
	// head> 104!! 103! 101 16 15 14 13 12 11 10 9 <tail
	cache.Store(104, "Super heavy", Weight(5))
	for i := 1; i < 9; i++ {
		tt.False(cache.Exists(i), "%d", i)
	}
	for i := 9; i <= maxWt; i++ {
		tt.True(cache.Exists(i), "%d", i)
	}
	tt.True(cache.Exists(101), "%d", 101)
	tt.True(cache.Exists(103), "%d", 103)
	tt.True(cache.Exists(104), "%d", 104)
	tt.Equal(maxWt, cache.Weight())

	tt.True(integrity(cache))
}

func TestLRU_ChangeMaxWeight(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	maxWt := 16
	cache := New[int, string](maxWt, time.Hour)

	for i := 1; i <= maxWt; i++ {
		cache.Store(i, "1", Weight(1))
	}
	tt.Equal(maxWt, cache.Weight())

	// Halve the weight limit
	cache.SetMaxWeight(maxWt / 2)

	tt.Equal(maxWt/2, cache.Weight())

	tt.True(integrity(cache))
}

func TestLRU_Clear(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Hour)
	tt.Zero(cache.Len())
	tt.Zero(cache.Weight())

	n := 6
	sum := 0
	for i := 1; i <= n; i++ {
		cache.Store(i, "X", Weight(i))
		sum += i
	}
	for i := 1; i <= n; i++ {
		v, ok := cache.Load(i)
		tt.True(ok)
		tt.Equal("X", v)
	}
	tt.Equal(n, cache.Len())
	tt.Equal(sum, cache.Weight())

	cache.Clear()
	for i := 1; i <= n; i++ {
		v, ok := cache.Load(i)
		tt.False(ok)
		tt.Equal("", v)
	}
	tt.Zero(cache.Len())
	tt.Zero(cache.Weight())

	tt.True(integrity(cache))
}

func TestLRU_Delete(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	span := 10
	sim := map[int]string{}
	cache := New[int, string](1024, time.Hour)
	for range 2048 {
		n := rand.IntN(span * 2)
		if n >= span {
			delete(sim, n-span)
			cache.Delete(n - span)
			tt.False(cache.Exists(n - span))
		} else {
			sim[n] = "X"
			cache.Store(n, "X")
			tt.True(cache.Exists(n))
		}
	}

	for i := range span {
		v, _ := cache.Load(i)
		tt.Equal(sim[i], v)
	}

	tt.True(integrity(cache))
}

func TestLRU_DeletePredicate(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Hour)
	for i := 1; i <= 10; i++ {
		cache.Store(i, "X")
	}
	tt.Equal(10, cache.Len())
	cache.DeletePredicate(func(key int) bool {
		return key <= 5
	})
	tt.Equal(5, cache.Len())
	for i := 1; i <= 10; i++ {
		tt.Equal(i > 5, cache.Exists(i))
	}

	tt.True(integrity(cache))
}

func TestLRU_MaxAge(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Second*35)

	cache.Store(0, "X")
	cache.timeOffset += time.Second * 30 // t=30
	cache.Store(30, "X")
	tt.True(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.Equal(2, cache.Len())

	// Elements older than the max age of the cache should expire
	cache.timeOffset += time.Second * 10 // t=40
	cache.Store(40, "X")
	tt.Equal(3, cache.Len()) // 0 element is still cached
	tt.False(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.True(cache.Exists(40))
	tt.Equal(2, cache.Len()) // 0 element was evicted on failed load

	cache.timeOffset += time.Second * 30 // t=70
	tt.False(cache.Exists(30))
	tt.True(cache.Exists(40))
	tt.Equal(1, cache.Len()) // 30 element was evicted on failed load

	// The load option overrides the cache's default max age
	_, ok := cache.Load(40, MaxAge(29*time.Second))
	tt.False(ok)

	tt.True(integrity(cache))
}

func TestLRU_BumpMaxAge(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Second*30)

	cache.Store(0, "X")
	cache.timeOffset += time.Second * 20
	_, ok := cache.Load(0, Bump(true))
	tt.True(ok)
	cache.timeOffset += time.Second * 20
	_, ok = cache.Load(0, Bump(true))
	tt.True(ok)
}

func TestLRU_ReduceMaxAge(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Minute)

	cache.Store(0, "X")
	cache.timeOffset += time.Second * 30
	cache.Store(30, "X")
	cache.timeOffset += time.Second * 20
	cache.Store(60, "X")
	tt.True(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.True(cache.Exists(60))
	tt.Equal(3, cache.Len())

	// Halve the age limit
	cache.SetMaxAge(30 * time.Second)

	tt.False(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.True(cache.Exists(60))
	tt.Equal(2, cache.Len()) // 0 element was evicted on failed load

	tt.True(integrity(cache))
}

func TestLRU_IncreaseMaxAge(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Minute)

	cache.Store(0, "X")
	cache.timeOffset += time.Second * 30
	cache.Store(30, "X")
	cache.timeOffset += time.Second * 20
	cache.Store(60, "X")
	tt.True(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.True(cache.Exists(60))
	tt.Equal(3, cache.Len())

	// Double the age limit
	cache.SetMaxAge(time.Minute * 2)
	cache.timeOffset += time.Second * 30
	cache.Store(90, "X")

	tt.True(cache.Exists(0))
	tt.True(cache.Exists(30))
	tt.True(cache.Exists(60))
	tt.True(cache.Exists(90))
	tt.Equal(4, cache.Len())

	tt.True(integrity(cache))
}

func TestLRU_Bump(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](8, time.Hour)

	// Fill in the cache
	// head> 8 7 6 5 4 3 2 1 <tail
	for i := 1; i <= 8; i++ {
		cache.Store(i, "X")
	}
	tt.Equal(8, cache.Len())

	// Loading element 2 should bump it to the head of the cache
	// head> 2 8 7 6 5 4 3 1 <tail
	_, ok := cache.Load(2)
	tt.True(ok)
	tt.Equal(8, cache.Len())
	tt.True(cache.Exists(1))

	// Storing element 9 should evict 1
	// head> 9 2 8 7 6 5 4 3 <tail
	cache.Store(9, "X")
	tt.Equal(8, cache.Len())
	tt.False(cache.Exists(1))

	// Storing element 10 evicts 3
	// head> 10 9 2 8 7 6 5 4 <tail
	cache.Store(10, "X")
	tt.Equal(8, cache.Len())
	tt.False(cache.Exists(1))
	tt.False(cache.Exists(3))
	tt.True(cache.Exists(4))

	// Load element 4 without bumping it to the head of the queue
	// Storing element 11 evicts 4
	// head> 11 10 9 2 8 7 6 5 <tail
	cache.Load(4, NoBump())
	cache.Store(11, "X")
	tt.Equal(8, cache.Len())
	tt.False(cache.Exists(4))
	tt.True(cache.Exists(5))

	tt.True(integrity(cache))
}

func TestLRU_RandomCohesion(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	cache := New[int, string](1024, time.Hour)

	for step := 0; step < 100000; step++ {
		key := rand.IntN(8)
		wt := rand.IntN(4) + 1
		maxAge := time.Duration(rand.IntN(30)) * time.Second
		bump := rand.IntN(1) == 0
		op := rand.IntN(7)
		switch op {
		case 0, 1, 2:
			cache.Store(key, "X", Weight(wt))
		case 3, 4:
			cache.Load(key, MaxAge(maxAge), Bump(bump))
		case 5:
			cache.LoadOrStore(key, "Y", Weight(wt), MaxAge(maxAge), Bump(bump))
		case 6:
			cache.Delete(key)
		}
	}

	tt.True(integrity(cache))
}

func BenchmarkLRU_Store(b *testing.B) {
	cache := New[int, int](b.N*2, time.Hour)
	for i := range b.N {
		cache.Store(i, i)
	}

	// On 2021 MacBook M1 Pro 16":
	// 288 ns/op
}

func BenchmarkLRU_LoadNoBump(b *testing.B) {
	cache := New[int, int](b.N*2, time.Hour)
	for i := range b.N {
		cache.Store(i, i)
	}
	b.ResetTimer()
	for i := range b.N {
		cache.Load(i, NoBump())
	}

	// On 2021 MacBook M1 Pro 16":
	// 193 ns/op
}

func BenchmarkLRU_LoadBump(b *testing.B) {
	cache := New[int, int](b.N*2, time.Hour)
	for i := range b.N {
		cache.Store(i, i)
	}
	b.ResetTimer()
	for i := range b.N {
		cache.Load(i)
	}

	// On 2021 MacBook M1 Pro 16":
	// 190 ns/op
}

// integrity checks the internal structure of the cache.
func integrity[K comparable, V any](c *Cache[K, V]) bool {
	a := []K{}
	count := 0
	for nd := c.newest; nd != nil; nd = nd.older {
		a = append(a, nd.key)
		if c.lookup[nd.key] != nd {
			return false
		}
		count++
		if count > 1000000 {
			return false
		}
	}
	if len(a) != len(c.lookup) {
		return false
	}
	count = 0
	for nd := c.oldest; nd != nil; nd = nd.newer {
		if len(a) == 0 {
			return false
		}
		if a[len(a)-1] != nd.key {
			return false
		}
		a = a[:len(a)-1]
		count++
		if count > 1000000 {
			return false
		}
	}
	return true
}
