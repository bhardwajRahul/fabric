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

package transport

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestTransport_ConcurrentSub(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var trie trie

	handler := func(msg *Msg) {}
	n := 1024
	unsubs := make(chan func(), n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			unsubs <- trie.Sub("my.subject", "myqueue", handler)
			wg.Done()
		}()
	}
	wg.Wait()

	// Ring should have all n stones
	found := map[int]bool{}
	ring := trie.children["my"].children["subject"].queues["myqueue"]
	for range 2 * n {
		h := ring.Rotate()
		found[h.Index] = true
	}
	assert.Len(found, n)

	// Unsub from all but the last one
	for range n - 1 {
		wg.Add(1)
		go func() {
			(<-unsubs)()
			wg.Done()
		}()
	}
	wg.Wait()

	// Ring should have 1 stone
	found = map[int]bool{}
	ring = trie.children["my"].children["subject"].queues["myqueue"]
	for range 2 * n {
		h := ring.Rotate()
		found[h.Index] = true
	}
	assert.Len(found, 1)

	// Unsub from all should trim the trie back to empty
	(<-unsubs)()
	assert.Nil(trie.children["my"])
}

func TestTransport_Handlers(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var trie trie
	var msg *Msg

	id := 0
	h1 := func(msg *Msg) { id = 1 }
	h2 := func(msg *Msg) { id = 2 }
	h3 := func(msg *Msg) { id = 3 }
	h4 := func(msg *Msg) { id = 4 }
	h5 := func(msg *Msg) { id = 5 }
	h6 := func(msg *Msg) { id = 6 }

	trie.Sub("my.subject", "myqueue", h1)
	trie.Sub("my.subject", "myqueue", h2)
	trie.Sub("my.subject", "myqueue", h3)
	trie.Sub("my.subject", "", h4)
	trie.Sub("my.subject", "", h5)
	trie.Sub("my.subject", "", h6)

	// Serial
	for range 4096 {
		handlers := trie.Handlers("my.subject")
		found := map[int]bool{}
		for _, h := range handlers {
			h(msg)
			found[id] = true
		}
		assert.Len(handlers, 4)
		assert.Len(found, 4)
		assert.True(found[1] || found[2] || found[3])
		assert.False(found[1] && found[2] && found[3])
		assert.True(found[4] && found[5] && found[6])
	}

	// Concurrent
	var wg sync.WaitGroup
	for range 4096 {
		wg.Add(1)
		go func() {
			handlers := trie.Handlers("my.subject")
			if !assert.Len(handlers, 4) {
				for _, h := range handlers {
					h(msg)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestTransport_Wildcards(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var trie trie
	var msg *Msg

	id := 0
	h1 := func(msg *Msg) { id = 1 }
	h2 := func(msg *Msg) { id = 2 }
	h3 := func(msg *Msg) { id = 3 }

	// >
	trie.Sub("alpha.443.com.example.|.POST.foo.>", "alpha", h1)
	assert.Len(trie.Handlers("alpha.443.com.example.|.POST.foo"), 0)
	assert.Len(trie.Handlers("alpha.443.com.example.|.POST.foo.bar"), 1)
	assert.Len(trie.Handlers("alpha.443.com.example.|.POST.foo.bar.baz"), 1)
	assert.Len(trie.Handlers("alpha.123.com.example.|.POST.foo.bar"), 0)
	trie.Handlers("alpha.443.com.example.|.POST.foo.bar.baz")[0](msg)
	assert.Equal(1, id)

	// *
	trie.Sub("beta.*.com.example.|.GET.foo", "beta", h2)
	assert.Len(trie.Handlers("beta.443.com.example.|.POST.foo"), 0)
	assert.Len(trie.Handlers("beta.443.com.example.|.GET.foo"), 1)
	assert.Len(trie.Handlers("beta.123.com.example.|.GET.foo"), 1)
	assert.Len(trie.Handlers("beta.123.com.example.|.GET.foo.bar"), 0)
	trie.Handlers("beta.443.com.example.|.GET.foo")[0](msg)
	assert.Equal(2, id)

	// * *
	trie.Sub("gamma.*.com.example.|.*.foo", "gamma", h3)
	trie.Sub("gamma.888.com.example.|.*.foo", "gamma", h3)
	assert.Len(trie.Handlers("gamma.456.com.example.|.PATCH.foo"), 1)
	assert.Len(trie.Handlers("gamma.888.com.example.|.PATCH.foo"), 2)
	assert.Len(trie.Handlers("gamma.456.edu.example.|.PATCH.foo"), 0)
	trie.Handlers("gamma.456.com.example.|.PATCH.foo")[0](msg)
	assert.Equal(3, id)
}

func TestTransport_SubUnsub(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var trie trie
	var msg *Msg

	id := 0
	h1 := func(msg *Msg) { id = 1 }
	h2 := func(msg *Msg) { id = 2 }
	h3 := func(msg *Msg) { id = 3 }

	unsub1 := trie.Sub("foo.bar", "myqueue", h1)
	assert.NotNil(unsub1)
	unsub2 := trie.Sub("foo.bar", "myqueue", h2)
	assert.NotNil(unsub2)

	handlers := trie.Handlers("foo.bar")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(1, id)
	}
	handlers = trie.Handlers("foo.baz")
	assert.Len(handlers, 0)

	unsub3 := trie.Sub("foo.bar.moo.baz", "myqueue", h3)
	assert.NotNil(unsub3)

	handlers = trie.Handlers("foo.bar.moo.baz")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(3, id)
	}
	handlers = trie.Handlers("foo.bar.moo")
	assert.Len(handlers, 0)
	handlers = trie.Handlers("foo.bar")
	assert.Len(handlers, 1)
	handlers = trie.Handlers("foo")
	assert.Len(handlers, 0)
	handlers = trie.Handlers("")
	assert.Len(handlers, 0)
}

func TestTransport_Trim(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	var trie trie
	var msg *Msg

	id := 0
	h1 := func(msg *Msg) { id = 1 }
	h2 := func(msg *Msg) { id = 2 }
	h3 := func(msg *Msg) { id = 3 }

	unsub1 := trie.Sub("foo.bar", "myqueue", h1)
	unsub1()
	assert.True(trie.IsEmpty())

	unsub1 = trie.Sub("foo.bar", "myqueue", h1)
	unsub2 := trie.Sub("foo.bar.moo.baz", "myqueue", h2)
	unsub3 := trie.Sub("foo.bar.too.bat", "myqueue", h3)

	handlers := trie.Handlers("foo.bar.moo.baz")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(2, id)
	}
	handlers = trie.Handlers("foo.bar.too.bat")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(3, id)
	}
	handlers = trie.Handlers("foo.bar.moo")
	assert.Len(handlers, 0)
	handlers = trie.Handlers("foo.bar")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(1, id)
	}

	unsub2()
	unsub2()
	handlers = trie.Handlers("foo.bar.moo.baz")
	assert.Len(handlers, 0)
	handlers = trie.Handlers("foo.bar.too.bat")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(3, id)
	}
	handlers = trie.Handlers("foo.bar.moo")
	assert.Len(handlers, 0)
	handlers = trie.Handlers("foo.bar")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(1, id)
	}

	unsub1()
	handlers = trie.Handlers("foo.bar.too.bat")
	if assert.Len(handlers, 1) {
		handlers[0](msg)
		assert.Equal(3, id)
	}
	handlers = trie.Handlers("foo.bar")
	assert.Len(handlers, 0)

	unsub3()
	assert.True(trie.IsEmpty())
}

func TestTransport_RandomSubUnsub(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	n := 32000
	var trie trie
	h := func(msg *Msg) {}
	ch := make(chan func(), n*runtime.NumCPU()*4)
	var wg sync.WaitGroup

	for range runtime.NumCPU() * 4 {
		wg.Add(1)
		go func() {
			for range n {
				// Sub
				sub := string(rune('A' + rand.IntN(3)))
				l := rand.IntN(4)
				for range l {
					sub += "." + string(rune('A'+rand.IntN(3)))
				}
				ch <- trie.Sub(sub, "q", h)
				// 25% chance to unsub
				l = rand.IntN(4)
				if l == 0 {
					(<-ch)()
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(ch)
	for f := range ch {
		assert.False(trie.IsEmpty())
		f()
	}
	assert.True(trie.IsEmpty())
}

// buildBenchTrie populates a trie mimicking a multi-service bundle: `services` hostnames, each exposing
// `endpointsPerService` routes. Subjects use the real Microbus subscription shape (8 segments, source slot
// wildcarded with `*` as actual subs are). It also returns the matching lookup subjects (concrete source), as
// a publisher on the short-circuit path would emit.
func buildBenchTrie(services, endpointsPerService int) (*trie, []string) {
	tr := &trie{}
	handler := func(*Msg) {}
	var lookups []string
	for s := range services {
		host := fmt.Sprintf("svc%d_core", s)
		for e := range endpointsPerService {
			tr.Sub(fmt.Sprintf("microbus.safe.443.*.%s._.GET.path%d", host, e), host, handler)
			lookups = append(lookups, fmt.Sprintf("microbus.safe.443.caller_core.%s._.GET.path%d", host, e))
		}
	}
	return tr, lookups
}

// BenchmarkTrie_Handlers measures a single short-circuit route match (the locked traversal in Handlers).
func BenchmarkTrie_Handlers(b *testing.B) {
	tr, lookups := buildBenchTrie(50, 20) // 1000 subscriptions
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(tr.Handlers(lookups[i%len(lookups)])) == 0 {
			b.Fatal("no handler matched")
		}
	}
	// Apple M1 Pro, 1000 subscriptions: ~358 ns/op, 8 B/op, 1 alloc/op (~2.8M matches/sec). A route lookup is a
	// rounding error next to the work a handler does per request, so the short-circuit trie is never the bottleneck.
}

// BenchmarkTrie_HandlersParallel measures route matching under contention: every goroutine hits the same
// process-global root mutex, which is the ceiling the review flagged. Throughput here is the aggregate the
// whole bundle can sustain on the short-circuit path.
func BenchmarkTrie_HandlersParallel(b *testing.B) {
	tr, lookups := buildBenchTrie(50, 20)
	b.ReportAllocs()
	b.ResetTimer()
	var seq atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		i := int(seq.Add(1))
		for pb.Next() {
			if len(tr.Handlers(lookups[i%len(lookups)])) == 0 {
				b.Fatal("no handler matched")
			}
			i++
		}
	})
	// Apple M1 Pro, 1000 subscriptions, -cpu 8: ~456 ns/op (~2.2M matches/sec aggregate) - only ~1.3x the serial
	// cost, not a collapse, because the critical section is a short trie walk. The single mutex is a real but very
	// high ceiling, orders of magnitude above any achievable microservice request rate.
}
