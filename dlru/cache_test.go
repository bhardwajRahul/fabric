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

package dlru_test

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/dlru"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestDLRU_Lookup(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("lookup.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("lookup.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	gamma := connector.New("lookup.dlru")
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()
	gammaLRU := gamma.DistribCache()

	// Insert to alpha cache
	err = alphaLRU.Store(ctx, "A", []byte("AAA"))
	assert.NoError(err)
	jsonObject := struct {
		Num int    `json:"num"`
		Str string `json:"str"`
	}{
		123,
		"abc",
	}
	err = alphaLRU.StoreJSON(ctx, "B", jsonObject)
	assert.NoError(err)
	err = alphaLRU.StoreCompressedJSON(ctx, "C", jsonObject)
	assert.NoError(err)

	assert.Equal(3, alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Zero(gammaLRU.LocalCache().Len())

	// Should be loadable from all caches
	for _, c := range []*dlru.Cache{gammaLRU, betaLRU, alphaLRU} {
		val, ok, err := c.Load(ctx, "A")
		assert.NoError(err)
		assert.True(ok)
		assert.Equal("AAA", string(val))

		var jval struct {
			Num int    `json:"num"`
			Str string `json:"str"`
		}
		ok, err = c.LoadJSON(ctx, "B", &jval)
		assert.NoError(err)
		assert.True(ok)
		assert.Equal(jsonObject, jval)
		ok, err = c.LoadCompressedJSON(ctx, "C", &jval)
		assert.NoError(err)
		assert.True(ok)
		assert.Equal(jsonObject, jval)
	}

	// Delete from gamma cache
	err = gammaLRU.Delete(ctx, "A")
	assert.NoError(err)

	assert.Equal(2, alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Zero(gammaLRU.LocalCache().Len())

	// Should not be loadable from any of the caches
	for _, c := range []*dlru.Cache{gammaLRU, betaLRU, alphaLRU} {
		val, ok, err := c.Load(ctx, "A")
		assert.NoError(err)
		assert.False(ok)
		assert.Equal("", string(val))

		val, ok, err = c.Load(ctx, "B")
		assert.NoError(err)
		assert.True(ok)
		assert.Equal(`{"num":123,"str":"abc"}`, string(val))
	}

	// Clear the cache via beta
	err = betaLRU.Clear(ctx)
	assert.NoError(err)

	assert.Zero(alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Zero(gammaLRU.LocalCache().Len())

	// Should not be loadable from any of the caches
	for _, c := range []*dlru.Cache{gammaLRU, betaLRU, alphaLRU} {
		val, ok, err := c.Load(ctx, "B")
		assert.NoError(err)
		assert.False(ok)
		assert.Equal("", string(val))
	}
}

func TestDLRU_Replicate(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("replicate.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("replicate.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	gamma := connector.New("replicate.dlru")
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()
	gammaLRU := gamma.DistribCache()

	// Insert to alpha cache
	err = alphaLRU.Store(ctx, "A", []byte("AAA"), dlru.Replicate(true))
	assert.NoError(err)
	jsonObject := struct {
		Num int    `json:"num"`
		Str string `json:"str"`
	}{
		123,
		"abc",
	}
	err = alphaLRU.StoreJSON(ctx, "B", jsonObject, dlru.Replicate(true))
	assert.NoError(err)
	err = alphaLRU.StoreCompressedJSON(ctx, "C", jsonObject, dlru.Replicate(true))
	assert.NoError(err)

	assert.Equal(3, alphaLRU.LocalCache().Len())
	assert.Equal(3, betaLRU.LocalCache().Len())
	assert.Equal(3, gammaLRU.LocalCache().Len())

	// Delete from gamma cache
	err = gammaLRU.Delete(ctx, "A")
	assert.NoError(err)

	assert.Equal(2, alphaLRU.LocalCache().Len())
	assert.Equal(2, betaLRU.LocalCache().Len())
	assert.Equal(2, gammaLRU.LocalCache().Len())

	// Clear the cache via beta
	err = betaLRU.Clear(ctx)
	assert.NoError(err)

	assert.Zero(alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Zero(gammaLRU.LocalCache().Len())
}

func TestDLRU_Rescue(t *testing.T) {
	// No parallel: sensitive to timeouts
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("rescue.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	alphaLRU := alpha.DistribCache()

	// Store values in alpha before starting beta and gamma
	n := 2048
	numChan := make(chan int, n)
	for i := range n {
		numChan <- i
	}
	close(numChan)
	var wg sync.WaitGroup
	for range runtime.NumCPU() * 4 {
		wg.Add(1)
		go func() {
			for i := range numChan {
				err := alphaLRU.Store(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)))
				assert.NoError(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(n, alphaLRU.LocalCache().Len())

	beta := connector.New("rescue.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	gamma := connector.New("rescue.dlru")
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()
	gammaLRU := gamma.DistribCache()

	assert.Zero(betaLRU.LocalCache().Len())
	assert.Zero(gammaLRU.LocalCache().Len())

	// Should distribute the elements to beta and gamma
	err = alpha.Shutdown()
	assert.NoError(err)
	assert.Equal(n, betaLRU.LocalCache().Len()+gammaLRU.LocalCache().Len())

	numChan = make(chan int, n)
	for i := range n {
		numChan <- i
	}
	close(numChan)
	for range runtime.NumCPU() * 4 {
		for i := range numChan {
			wg.Add(1)
			go func() {
				val, ok, err := betaLRU.Load(ctx, strconv.Itoa(i))
				if assert.NoError(err) && assert.True(ok, i) {
					assert.Equal(strconv.Itoa(i), string(val))
				}
				val, ok, err = gammaLRU.Load(ctx, strconv.Itoa(i))
				if assert.NoError(err) && assert.True(ok, i) {
					assert.Equal(strconv.Itoa(i), string(val))
				}
				wg.Done()
			}()
		}
	}
	wg.Wait()
}

func TestDLRU_MaxMemory(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()
	maxMem := 4096

	alpha := connector.New("max.memory.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()
	alphaLRU.SetMaxMemory(maxMem)

	beta := connector.New("max.memory.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()
	betaLRU.SetMaxMemory(maxMem)

	// Insert enough to max out the memory limit
	payload := rand.AlphaNum64(maxMem / 4)
	err = alphaLRU.Store(ctx, "A", []byte(payload))
	assert.NoError(err)
	err = alphaLRU.Store(ctx, "B", []byte(payload))
	assert.NoError(err)
	err = alphaLRU.Store(ctx, "C", []byte(payload))
	assert.NoError(err)
	err = alphaLRU.Store(ctx, "D", []byte(payload))
	assert.NoError(err)

	// Should be stored in alpha
	// alpha: D C B A
	// beta:
	assert.Equal(4, alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Equal(maxMem, alphaLRU.LocalCache().Weight())
	assert.Zero(betaLRU.LocalCache().Weight())

	// Insert another 1/4
	err = alphaLRU.Store(ctx, "E", []byte(payload))
	assert.NoError(err)

	// Alpha will have A evicted
	// alpha: E D C B
	// beta:
	assert.Equal(4, alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
	assert.Equal(maxMem, alphaLRU.LocalCache().Weight())
	assert.Zero(betaLRU.LocalCache().Weight())

	for _, k := range []string{"A", "B", "C", "D", "E"} {
		val, ok, err := betaLRU.Load(ctx, k)
		assert.NoError(err)
		assert.Equal(k != "A", ok)
		if ok {
			assert.Equal(payload, string(val))
		}
	}
}

func TestDLRU_WeightAndLen(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("weight.and.len.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("weight.and.len.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	payload := rand.AlphaNum64(1024)
	err = alphaLRU.Store(ctx, "A", []byte(payload))
	assert.NoError(err)

	wt, _ := alphaLRU.Weight(ctx)
	assert.Equal(1024, wt)
	len, _ := alphaLRU.Len(ctx)
	assert.Equal(1, len)

	wt, _ = betaLRU.Weight(ctx)
	assert.Equal(1024, wt)
	len, _ = betaLRU.Len(ctx)
	assert.Equal(1, len)

	err = betaLRU.Store(ctx, "B", []byte(payload))
	assert.NoError(err)

	wt, _ = alphaLRU.Weight(ctx)
	assert.Equal(2048, wt)
	len, _ = alphaLRU.Len(ctx)
	assert.Equal(2, len)

	wt, _ = betaLRU.Weight(ctx)
	assert.Equal(2048, wt)
	len, _ = betaLRU.Len(ctx)
	assert.Equal(2, len)
}

func TestDLRU_Options(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	dlru, err := dlru.NewCache(context.Background(), connector.New("www.example.com"), "/path")
	dlru.SetMaxAge(5 * time.Hour)
	dlru.SetMaxMemoryMB(8)
	assert.NoError(err)

	assert.Equal(5*time.Hour, dlru.MaxAge())
	assert.Equal(8*1024*1024, dlru.MaxMemory())
}

func TestDLRU_MulticastOptim(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("multicast.optim.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("multicast.optim.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()

	// First operation is slow because of being the first broadcast
	t0 := time.Now()
	err = alphaLRU.Store(ctx, "Foo", []byte("Bar"))
	assert.NoError(err)
	durSlow := time.Since(t0)

	// Second operation is fast, even if not the same action, because of the known responders optimization
	t0 = time.Now()
	err = alphaLRU.Clear(ctx)
	assert.NoError(err)
	durFast := time.Since(t0)
	assert.True(durFast*2 < durSlow)
}

func TestDLRU_InvalidRequests(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	con := connector.New("invalid.requests.dlru")
	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	cache, err := dlru.NewCache(ctx, con, "/cache")
	assert.NoError(err)
	defer cache.Close(ctx)

	_, _, err = cache.Load(ctx, "")
	assert.Equal("missing key", err.Error())
	_, err = cache.LoadJSON(ctx, "", nil)
	assert.Equal("missing key", err.Error())
	err = cache.Store(ctx, "", nil)
	assert.Equal("missing key", err.Error())
	err = cache.StoreJSON(ctx, "", nil)
	assert.Equal("missing key", err.Error())
	err = cache.Delete(ctx, "")
	assert.Equal("missing key", err.Error())
}

func TestDLRU_Inconsistency(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("inconsistency.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("inconsistency.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	// Store an element in the cache
	err = alphaLRU.Store(ctx, "Foo", []byte("Bar"))
	assert.NoError(err)

	// Should be stored in alpha
	assert.Equal(1, alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())

	// Should be loadable from either caches
	val, ok, err := alphaLRU.Load(ctx, "Foo")
	assert.NoError(err)
	assert.True(ok)
	assert.Equal("Bar", string(val))
	val, ok, err = betaLRU.Load(ctx, "Foo")
	assert.NoError(err)
	assert.True(ok)
	assert.Equal("Bar", string(val))

	// Store a different value in beta
	betaLRU.LocalCache().Store("Foo", []byte("Bad"))

	// Loading without the consistency check should succeed and return different results
	val, ok, err = alphaLRU.Load(ctx, "Foo", dlru.ConsistencyCheck(false))
	assert.NoError(err)
	assert.True(ok)
	assert.Equal("Bar", string(val))
	val, ok, err = betaLRU.Load(ctx, "Foo", dlru.ConsistencyCheck(false))
	assert.NoError(err)
	assert.True(ok)
	assert.Equal("Bad", string(val))

	// Loading with a consistency check should fail from either caches
	_, ok, err = alphaLRU.Load(ctx, "Foo")
	assert.NoError(err)
	assert.False(ok)
	_, ok, err = betaLRU.Load(ctx, "Foo")
	assert.NoError(err)
	assert.False(ok)

	// The inconsistent values should be removed
	assert.Zero(alphaLRU.LocalCache().Len())
	assert.Zero(betaLRU.LocalCache().Len())
}

func TestDLRU_MaxAge(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("maxage.actions.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("maxage.actions.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	// Store an element in the cache
	err = alphaLRU.Store(ctx, "Foo", []byte("Bar"))
	assert.NoError(err)

	// Wait a second and load it back
	// Do not bump so that the life of the element is not renewed
	time.Sleep(time.Second)
	cached, ok, err := betaLRU.Load(ctx, "Foo", dlru.NoBump())
	assert.NoError(err)
	if assert.True(ok) {
		assert.Equal(string(cached), "Bar")
	}

	// Use a max age option when loading
	_, ok, err = betaLRU.Load(ctx, "Foo", dlru.MaxAge(time.Millisecond*990))
	assert.NoError(err)
	assert.False(ok)
}

func TestDLRU_DeletePrefix(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("delete.prefix.actions.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("delete.prefix.actions.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	for i := 1; i <= 10; i++ {
		alphaLRU.Store(ctx, fmt.Sprintf("prefix.%d", i), []byte("X"))
	}
	for i := 1; i <= 10; i++ {
		betaLRU.Store(ctx, fmt.Sprintf("other.%d", i), []byte("X"))
	}

	for i := 1; i <= 10; i++ {
		_, ok, err := betaLRU.Load(ctx, fmt.Sprintf("prefix.%d", i))
		assert.NoError(err)
		assert.True(ok)
		_, ok, err = alphaLRU.Load(ctx, fmt.Sprintf("other.%d", i))
		assert.NoError(err)
		assert.True(ok)
	}

	err = betaLRU.DeletePrefix(ctx, "prefix.")
	assert.NoError(err)

	for i := 1; i <= 10; i++ {
		_, ok, err := betaLRU.Load(ctx, fmt.Sprintf("prefix.%d", i))
		assert.NoError(err)
		assert.False(ok)
		_, ok, err = alphaLRU.Load(ctx, fmt.Sprintf("other.%d", i))
		assert.NoError(err)
		assert.True(ok)
	}
}

func TestDLRU_DeleteContains(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("delete.contains.actions.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("delete.contains.actions.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	for i := 1; i <= 10; i++ {
		alphaLRU.Store(ctx, fmt.Sprintf("alpha.%d.end", i), []byte("X"))
	}
	for i := 1; i <= 10; i++ {
		betaLRU.Store(ctx, fmt.Sprintf("beta.%d.end", i), []byte("X"))
	}

	for i := 1; i <= 10; i++ {
		_, ok, err := betaLRU.Load(ctx, fmt.Sprintf("alpha.%d.end", i))
		assert.NoError(err)
		assert.True(ok)
		_, ok, err = alphaLRU.Load(ctx, fmt.Sprintf("beta.%d.end", i))
		assert.NoError(err)
		assert.True(ok)
	}

	err = betaLRU.DeleteContains(ctx, ".1")
	assert.NoError(err)

	for i := 1; i <= 10; i++ {
		_, ok, err := betaLRU.Load(ctx, fmt.Sprintf("alpha.%d.end", i))
		assert.NoError(err)
		assert.Equal(i != 1 && i != 10, ok)
		_, ok, err = alphaLRU.Load(ctx, fmt.Sprintf("beta.%d.end", i))
		assert.NoError(err)
		assert.Equal(i != 1 && i != 10, ok)
	}
}

func TestDLRU_RandomActions(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("random.actions.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()

	beta := connector.New("random.actions.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()

	gamma := connector.New("random.actions.dlru")
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()

	caches := []*dlru.Cache{
		alpha.DistribCache(),
		beta.DistribCache(),
		gamma.DistribCache(),
	}

	state := map[string][]byte{}
	for range 10000 {
		cache := caches[rand.IntN(len(caches))]
		key := strconv.Itoa(rand.IntN(20))
		switch rand.IntN(4) {
		case 1, 2: // Load
			bump := rand.IntN(2) == 1
			val1, ok1, err := cache.Load(ctx, key, dlru.Bump(bump))
			assert.NoError(err)
			val2, ok2 := state[key]
			assert.Equal(ok2, ok1)
			assert.Equal(val2, val1)

		case 3: // Store
			val := []byte(rand.AlphaNum32(15))
			err := cache.Store(ctx, key, val)
			assert.NoError(err)
			state[key] = val

		case 4: // Delete
			err := cache.Delete(ctx, key)
			assert.NoError(err)
			delete(state, key)
		}
	}
}

func BenchmarkDLRU_Store(b *testing.B) {
	ctx := context.Background()

	alpha := connector.New("benchmark.store.dlru")
	err := alpha.Startup()
	testarossa.NoError(b, err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("benchmark.store.dlru")
	err = beta.Startup()
	testarossa.NoError(b, err)
	defer beta.Shutdown()

	b.ResetTimer()
	for b.Loop() {
		err = alphaLRU.Store(ctx, "Foo", []byte("Bar"))
		testarossa.NoError(b, err)
	}
	b.StopTimer()

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/dlru
	// cpu: Apple M1 Pro
	// BenchmarkDLRU_Store-10    	    9290	    119185 ns/op	   17602 B/op	     300 allocs/op
}

func BenchmarkDLRU_Load(b *testing.B) {
	ctx := context.Background()

	alpha := connector.New("benchmark.load.dlru")
	err := alpha.Startup()
	testarossa.NoError(b, err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("benchmark.load.dlru")
	err = beta.Startup()
	testarossa.NoError(b, err)
	defer beta.Shutdown()

	err = alphaLRU.Store(ctx, "Foo", []byte("Bar"))
	testarossa.NoError(b, err)

	b.ResetTimer()
	for b.Loop() {
		_, ok, err := alphaLRU.Load(ctx, "Foo")
		testarossa.NoError(b, err)
		testarossa.True(b, ok)
	}
	b.StopTimer()

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/dlru
	// cpu: Apple M1 Pro
	// BenchmarkDLRU_Load-10    	    9517	    116841 ns/op	   19462 B/op	     320 allocs/op
}

func BenchmarkDLRU_LoadNoConsistencyCheck(b *testing.B) {
	ctx := context.Background()

	alpha := connector.New("benchmark.load.dlru")
	err := alpha.Startup()
	testarossa.NoError(b, err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("benchmark.load.dlru")
	err = beta.Startup()
	testarossa.NoError(b, err)
	defer beta.Shutdown()

	err = alphaLRU.Store(ctx, "Foo", []byte("Bar"), dlru.Replicate(true))
	testarossa.NoError(b, err)

	b.ResetTimer()
	for b.Loop() {
		_, ok, err := alphaLRU.Load(ctx, "Foo", dlru.ConsistencyCheck(false))
		testarossa.NoError(b, err)
		testarossa.True(b, ok)
	}
	b.StopTimer()

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/dlru
	// cpu: Apple M1 Pro
	// BenchmarkDLRU_LoadNoConsistencyCheck-10    	 5620533	       190.4 ns/op	     120 B/op	       4 allocs/op
}

func TestDLRU_Interface(t *testing.T) {
	t.Parallel()

	c := connector.New("example")
	_ = dlru.Service(c)
}

func TestDLRU_Compression(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := context.Background()

	alpha := connector.New("compression.dlru")
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	alphaLRU := alpha.DistribCache()

	beta := connector.New("compression.dlru")
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	betaLRU := beta.DistribCache()

	// Insert to alpha cache
	payload := []byte("The quick brown fox jumps over the lazy dog")
	err = alphaLRU.Store(ctx, "Fox", payload, dlru.Compress(true))
	assert.NoError(err)

	// Read from beta cache
	value, ok, err := betaLRU.Load(ctx, "Fox")
	if assert.NoError(err) && assert.True(ok) {
		assert.Equal(payload, value)
	}

	// Insert to alpha cache
	payload = []byte(rand.AlphaNum64(10 << 10)) // 10KiB
	err = alphaLRU.Store(ctx, "Random", payload, dlru.Compress(true))
	assert.NoError(err)

	// Read from beta cache
	value, ok, err = betaLRU.Load(ctx, "Random")
	if assert.NoError(err) && assert.True(ok) {
		assert.Equal(payload, value)
	}
}

func TestDLRU_AvailableInOnStartup(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	alpha := connector.New("availeable.in.on.startup.dlru")
	alpha.SetOnStartup(func(ctx context.Context) error {
		alpha.DistribCache().Set(ctx, "Foo", "Bar")
		return nil
	})
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()

	beta := connector.New("availeable.in.on.startup.dlru")
	beta.SetOnStartup(func(ctx context.Context) error {
		var val string
		beta.DistribCache().Get(ctx, "Foo", &val)
		assert.Equal("Bar", val)
		beta.DistribCache().Set(ctx, "Foo", "Baz")
		return nil
	})
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()

	gamma := connector.New("availeable.in.on.startup.dlru")
	gamma.SetOnStartup(func(ctx context.Context) error {
		var val string
		gamma.DistribCache().Get(ctx, "Foo", &val)
		assert.Equal("Baz", val)
		return nil
	})
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()
}
