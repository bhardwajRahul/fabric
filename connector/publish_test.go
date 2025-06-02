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

package connector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"
)

func TestConnector_Echo(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.echo.connector")

	beta := New("beta.echo.connector")
	beta.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		body, err := io.ReadAll(r.Body)
		tt.NoError(err)
		_, err = w.Write(body)
		tt.NoError(err)
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// Send message and validate that it's echoed back
	response, err := alpha.POST(ctx, "https://beta.echo.connector/echo", []byte("Hello"))
	tt.NoError(err)
	body, err := io.ReadAll(response.Body)
	tt.NoError(err)
	tt.Equal([]byte("Hello"), body)
}

func BenchmarkConnector_EchoSerial(b *testing.B) {
	ctx := context.Background()

	// Create the microservice
	alpha := New("alpha.echo.serial.connector")
	var echoCount atomic.Int32
	alpha.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		echoCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
		return nil
	})

	beta := New("beta.echo.serial.connector")

	// Startup the microservice
	alpha.Startup()
	defer alpha.Shutdown()
	beta.Startup()
	defer beta.Shutdown()

	// The bottleneck is waiting on the network i/o
	var errCount int
	b.ResetTimer()
	for b.Loop() {
		_, err := beta.POST(ctx, "https://alpha.echo.serial.connector/echo", []byte("Hello"))
		if err != nil {
			errCount++
		}
	}
	b.StopTimer()
	testarossa.Equal(b, 0, errCount)
	testarossa.Equal(b, int32(b.N), echoCount.Load())

	// On 2021 MacBook M1 Pro 16":
	// N=12117
	// 94226 ns/op (10612 ops/sec)
	// 19672 B/op
	// 277 allocs/op
}

func BenchmarkConnector_EchoSerialHTTP(b *testing.B) {
	// Create the web server
	var echoCount atomic.Int32
	httpServer := &http.Server{
		Addr: ":5555",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			echoCount.Add(1)
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}),
	}
	go httpServer.ListenAndServe()
	defer httpServer.Close()

	// The bottleneck is waiting on the network i/o
	var errCount int
	b.ResetTimer()
	for b.Loop() {
		res, err := http.Post("http://localhost:5555/", "", strings.NewReader("Hello"))
		if err != nil {
			errCount++
		}
		if res != nil && res.Body != nil {
			res.Body.Close()
		}
	}
	b.StopTimer()
	testarossa.Equal(b, 0, errCount)
	testarossa.Equal(b, int32(b.N), echoCount.Load())

	// On 2021 MacBook M1 Pro 16":
	// N=5540
	// 261968 ns/op (3817 ops/sec) = approx 1/3 vs via messaging bus
	// 26667 B/op
	// 173 allocs/op
}

func BenchmarkConnector_SerialChain(b *testing.B) {
	ctx := context.Background()

	// Create the microservice
	con := New("serial.chain.connector")
	var echoCount atomic.Int32
	con.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		if frame.Of(r).CallDepth() < 10 {
			// Go one level deeper
			res, err := con.POST(r.Context(), "https://serial.chain.connector/echo", r.Body)
			if err != nil {
				return errors.Trace(err)
			}
			body, _ := io.ReadAll(res.Body)
			w.Write(body)
		} else {
			// Echo back the request
			echoCount.Add(1)
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}
		return nil
	})

	// Startup the microservice
	con.Startup()
	defer con.Shutdown()

	// The bottleneck is waiting on the network i/o
	var errCount int
	b.ResetTimer()
	for b.Loop() {
		_, err := con.POST(ctx, "https://serial.chain.connector/echo", []byte("Hello"))
		if err != nil {
			errCount++
		}
	}
	b.StopTimer()
	testarossa.Equal(b, 0, errCount)
	testarossa.Equal(b, int32(b.N), echoCount.Load())

	// On 2021 MacBook M1 Pro 16":
	// N=1174
	// 988411 ns/op (1012 ops/sec)
	// 247735 B/op
	// 3013 allocs/op
}

func BenchmarkConnector_EchoParallelMax(b *testing.B) {
	echoParallel(b, b.N)

	// On 2021 MacBook M1 Pro 16":
	// N=91160 concurrent
	// 12577 ns/op (79510 ops/sec) = approx 8x that of serial
	// 19347 B/op
	// 280 allocs/op
}

func BenchmarkConnector_EchoParallel100(b *testing.B) {
	echoParallel(b, 100)

	// On 2021 MacBook M1 Pro 16":
	// N=58006
	// 19499 ns/op (51284 ops/sec) = approx 5x that of serial
	// 19314 B/op
	// 277 allocs/op
}

func BenchmarkConnector_EchoParallel1K(b *testing.B) {
	echoParallel(b, 1000)

	// On 2021 MacBook M1 Pro 16":
	// N=94744
	// 12102 ns/op (82630 ops/sec) = approx 8x that of serial
	// 19451 B/op
	// 278 allocs/op
}

func BenchmarkConnector_EchoParallel10K(b *testing.B) {
	echoParallel(b, 10000)

	// On 2021 MacBook M1 Pro 16":
	// N=107904
	// 10575 ns/op (94562 ops/sec) = approx 9x that of serial
	// 19412 B/op
	// 278 allocs/op
}

func echoParallel(b *testing.B, concurrency int) {
	ctx := context.Background()

	// Create the microservice
	alpha := New("alpha.echo.parallel.connector")
	var echoCount atomic.Int32
	alpha.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		echoCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
		return nil
	})

	beta := New("beta.echo.parallel.connector")
	beta.ackTimeout = time.Second

	// Startup the microservice
	alpha.Startup()
	defer alpha.Shutdown()
	beta.Startup()
	defer beta.Shutdown()

	var wg sync.WaitGroup
	wg.Add(b.N)
	b.ResetTimer()
	var errCount atomic.Int32
	for i := range concurrency {
		tot := b.N / concurrency
		if i < b.N%concurrency {
			tot++
		} // do remainder
		go func() {
			for range tot {
				_, err := beta.POST(ctx, "https://alpha.echo.parallel.connector/echo", []byte("Hello"))
				if err != nil {
					errCount.Add(1)
				}
				wg.Done()
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
	testarossa.Equal(b, int32(0), errCount.Load())
	testarossa.Equal(b, int32(b.N), echoCount.Load())
}

func BenchmarkConnector_EchoParallelHTTP100(b *testing.B) {
	echoParallelHTTP(b, 100)

	// On 2021 MacBook M1 Pro 16":
	// N=18675
	// 183849 ns/op (5439 ops/sec) = approx 1/10 vs via messaging bus
	// 20136 B/op
	// 156 allocs/op
}

func echoParallelHTTP(b *testing.B, concurrency int) {
	// Create the web server
	var echoCount atomic.Int32
	httpServer := &http.Server{
		Addr: ":5555",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			echoCount.Add(1)
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}),
	}
	go httpServer.ListenAndServe()
	defer httpServer.Close()

	var wg sync.WaitGroup
	wg.Add(b.N)
	b.ResetTimer()
	var errCount atomic.Int32
	for i := range concurrency {
		tot := b.N / concurrency
		if i < b.N%concurrency {
			tot++
		} // do remainder
		go func() {
			for range tot {
				res, err := http.Post("http://localhost:5555/", "", strings.NewReader("Hello"))
				if err != nil {
					errCount.Add(1)
				}
				if res != nil && res.Body != nil {
					res.Body.Close()
				}
				wg.Done()
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
	testarossa.Equal(b, int32(0), errCount.Load())
	testarossa.Equal(b, int32(b.N), echoCount.Load())
}

func TestConnector_EchoParallelCapacity(t *testing.T) {
	t.Skip() // Dependent on strength of CPU running the test
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservice
	alpha := New("alpha.echo.parallel.capacity.connector")
	var echoCount atomic.Int32
	alpha.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		echoCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
		return nil
	})

	beta := New("beta.echo.parallel.capacity.connector")

	// Startup the microservice
	alpha.Startup()
	defer alpha.Shutdown()
	beta.Startup()
	defer beta.Shutdown()

	// Goroutines can take as much as 1s to start in very high load situations or slow CPUs
	n := 10000
	var wg sync.WaitGroup
	wg.Add(n)
	t0 := time.Now()
	var totalTime atomic.Int64
	var maxTime atomic.Int32
	var errCount atomic.Int32
	for range n {
		go func() {
			tts := int(time.Since(t0).Milliseconds())
			totalTime.Add(int64(tts))
			currentMax := maxTime.Load()
			if int32(tts) > currentMax {
				maxTime.Store(int32(tts))
			}
			_, err := beta.POST(ctx, "https://alpha.echo.parallel.capacity.connector/echo", []byte("Hello"))
			if err != nil {
				errCount.Add(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	tt.Zero(errCount.Load())
	tt.Equal(int32(n), echoCount.Load())

	fmt.Printf("errs %d\n", errCount.Load())
	fmt.Printf("echo %d\n", echoCount.Load())
	fmt.Printf("avg time to start %d\n", totalTime.Load()/int64(n))
	fmt.Printf("max time to start %d\n", maxTime.Load())

	// On 2021 MacBook M1 Pro 16":
	// n=10000 avg=56 max=133
	// n=20000 avg=148 max=308 ackTimeout=1s
	// n=40000 avg=318 max=569 ackTimeout=1s
	// n=60000 avg=501 max=935 ackTimeout=1s
}

func TestConnector_QueryArgs(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	con := New("query.args.connector")
	con.Subscribe("GET", "arg", func(w http.ResponseWriter, r *http.Request) error {
		arg := r.URL.Query().Get("arg")
		tt.Equal("not_empty", arg)
		return nil
	})

	// Startup the microservices
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	// Send request with a query argument
	_, err = con.GET(ctx, "https://query.args.connector/arg?arg=not_empty")
	tt.NoError(err)
}

func TestConnector_LoadBalancing(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.load.balancing.connector")

	count1 := int32(0)
	count2 := int32(0)

	beta1 := New("beta.load.balancing.connector")
	beta1.Subscribe("GET", "lb", func(w http.ResponseWriter, r *http.Request) error {
		atomic.AddInt32(&count1, 1)
		return nil
	})

	beta2 := New("beta.load.balancing.connector")
	beta2.Subscribe("GET", "lb", func(w http.ResponseWriter, r *http.Request) error {
		atomic.AddInt32(&count2, 1)
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta1.Startup()
	tt.NoError(err)
	defer beta1.Shutdown()
	err = beta2.Startup()
	tt.NoError(err)
	defer beta2.Shutdown()

	// Send messages
	var wg sync.WaitGroup
	for range 256 {
		wg.Add(1)
		go func() {
			_, err := alpha.GET(ctx, "https://beta.load.balancing.connector/lb")
			tt.NoError(err)
			wg.Done()
		}()
	}
	wg.Wait()

	// The requests should be more or less evenly distributed among the server microservices
	tt.Equal(int32(256), count1+count2)
	tt.True(count1 > 64)
	tt.True(count2 > 64)
}

func TestConnector_Concurrent(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.concurrent.connector")

	beta := New("beta.concurrent.connector")
	beta.Subscribe("GET", "wait", func(w http.ResponseWriter, r *http.Request) error {
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		time.Sleep(time.Millisecond * time.Duration(ms))
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// Send messages
	var wg sync.WaitGroup
	for i := 50; i <= 500; i += 50 {
		i := i
		wg.Add(1)
		go func() {
			start := time.Now()
			_, err := alpha.GET(ctx, "https://beta.concurrent.connector/wait?ms="+strconv.Itoa(i))
			end := time.Now()
			tt.NoError(err)
			dur := start.Add(time.Millisecond * time.Duration(i)).Sub(end)
			tt.True(dur.Abs() <= time.Millisecond*49)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestConnector_CallDepth(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()
	depth := 0

	// Create the microservice
	con := New("call.depth.connector")
	con.maxCallDepth = 8
	con.Subscribe("GET", "next", func(w http.ResponseWriter, r *http.Request) error {
		depth++

		step, _ := strconv.Atoi(r.URL.Query().Get("step"))
		tt.Equal(depth, step)
		tt.Equal(depth, frame.Of(r).CallDepth())

		_, err := con.GET(r.Context(), "https://call.depth.connector/next?step="+strconv.Itoa(step+1))
		tt.Error(err)
		tt.Contains(err.Error(), "call depth overflow")
		return errors.Trace(err)
	})

	// Startup the microservices
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	_, err = con.GET(ctx, "https://call.depth.connector/next?step=1")
	tt.Error(err)
	tt.Contains(err.Error(), "call depth overflow")
	tt.Equal(con.maxCallDepth, depth)
}

func TestConnector_TimeoutDrawdown(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()
	depth := 0

	// Create the microservice
	con := New("timeout.drawdown.connector")
	budget := con.networkHop * 8
	con.Subscribe("GET", "next", func(w http.ResponseWriter, r *http.Request) error {
		depth++
		_, err := con.GET(r.Context(), "https://timeout.drawdown.connector/next")
		return errors.Trace(err)
	})

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	budgetedCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	_, err = con.Request(
		budgetedCtx,
		pub.GET("https://timeout.drawdown.connector/next"),
	)
	tt.Error(err)
	tt.Equal(http.StatusRequestTimeout, errors.Convert(err).StatusCode)
	tt.True(depth >= 7 && depth <= 8, "%d", depth)
}

func TestConnector_TimeoutContext(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Create the microservice
	con := New("timeout.context.connector")
	var deadline time.Time
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		deadline, _ = r.Context().Deadline()
		return nil
	})

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err = con.Request(
		ctx,
		pub.GET("https://timeout.context.connector/ok"),
	)
	if tt.NoError(err) {
		tt.False(deadline.IsZero())
		tt.True(time.Until(deadline) > time.Second*8, time.Until(deadline))
	}
}

func TestConnector_TimeoutNotFound(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservice
	con := New("timeout.not.found.connector")

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	// Set a time budget in the request
	shortCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	t0 := time.Now()
	_, err = con.Request(
		shortCtx,
		pub.GET("https://timeout.not.found.connector/nowhere"),
	)
	dur := time.Since(t0)
	tt.Error(err)
	tt.True(dur >= con.ackTimeout && dur < con.ackTimeout+time.Second)

	// Use the default time budget
	t0 = time.Now()
	_, err = con.Request(
		ctx,
		pub.GET("https://timeout.not.found.connector/nowhere"),
	)
	dur = time.Since(t0)
	tt.Error(err)
	tt.Equal(http.StatusNotFound, errors.Convert(err).StatusCode)
	tt.True(dur >= con.ackTimeout && dur < con.ackTimeout+time.Second)
}

func TestConnector_TimeoutSlow(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservice
	con := New("timeout.slow.connector")
	con.Subscribe("GET", "slow", func(w http.ResponseWriter, r *http.Request) error {
		time.Sleep(time.Second)
		return nil
	})

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	shortCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()
	t0 := time.Now()
	_, err = con.Request(
		shortCtx,
		pub.GET("https://timeout.slow.connector/slow"),
	)
	tt.Error(err)
	dur := time.Since(t0)
	tt.True(dur >= 500*time.Millisecond && dur < 600*time.Millisecond)
}

func TestConnector_ContextTimeout(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("context.timeout.connector")

	done := false
	con.Subscribe("GET", "timeout", func(w http.ResponseWriter, r *http.Request) error {
		<-r.Context().Done()
		done = true
		return r.Context().Err()
	})

	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	shortCtx, cancel := context.WithTimeout(con.Lifetime(), time.Second)
	defer cancel()
	_, err = con.Request(
		shortCtx,
		pub.GET("https://context.timeout.connector/timeout"),
	)
	tt.Error(err)
	tt.True(done)
}

func TestConnector_Multicast(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	noqueue1 := New("multicast.connector")
	noqueue1.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("noqueue1"))
		return nil
	}, sub.NoQueue())

	noqueue2 := New("multicast.connector")
	noqueue2.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("noqueue2"))
		return nil
	}, sub.NoQueue())

	named1 := New("multicast.connector")
	named1.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("named1"))
		return nil
	}, sub.Queue("MyQueue"))

	named2 := New("multicast.connector")
	named2.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("named2"))
		return nil
	}, sub.Queue("MyQueue"))

	def1 := New("multicast.connector")
	def1.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("def1"))
		return nil
	}, sub.DefaultQueue())

	def2 := New("multicast.connector")
	def2.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("def2"))
		return nil
	}, sub.DefaultQueue())

	ackTimeout := New("").ackTimeout

	// Startup the microservices
	for _, i := range []*Connector{noqueue1, noqueue2, named1, named2, def1, def2} {
		err := i.Startup()
		tt.NoError(err)
		defer i.Shutdown()
	}

	// Make the first request
	client := named1
	t0 := time.Now()
	responded := map[string]bool{}
	ch := client.Publish(ctx, pub.GET("https://multicast.connector/cast"), pub.Multicast())
	for i := range ch {
		res, err := i.Get()
		if tt.NoError(err) {
			body, err := io.ReadAll(res.Body)
			tt.NoError(err)
			responded[string(body)] = true
		}
	}
	dur := time.Since(t0)
	tt.True(dur >= ackTimeout && dur < ackTimeout+time.Second)
	tt.Len(responded, 4)
	tt.True(responded["noqueue1"])
	tt.True(responded["noqueue2"])
	tt.True(responded["named1"] || responded["named2"])
	tt.False(responded["named1"] && responded["named2"])
	tt.True(responded["def1"] || responded["def2"])
	tt.False(responded["def1"] && responded["def2"])

	// Make the second request, should be quicker due to known responders optimization
	t0 = time.Now()
	responded = map[string]bool{}
	ch = client.Publish(ctx, pub.GET("https://multicast.connector/cast"), pub.Multicast())
	for i := range ch {
		res, err := i.Get()
		if tt.NoError(err) {
			body, err := io.ReadAll(res.Body)
			tt.NoError(err)
			responded[string(body)] = true
		}
	}
	dur = time.Since(t0)
	tt.True(dur < ackTimeout)
	tt.Len(responded, 4)
	tt.True(responded["noqueue1"])
	tt.True(responded["noqueue2"])
	tt.True(responded["named1"] || responded["named2"])
	tt.False(responded["named1"] && responded["named2"])
	tt.True(responded["def1"] || responded["def2"])
	tt.False(responded["def1"] && responded["def2"])
}

func TestConnector_MulticastPartialTimeout(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()
	delay := time.Millisecond * 500

	// Create the microservices
	slow := New("multicast.partial.timeout.connector")
	slow.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		time.Sleep(delay * 2)
		w.Write([]byte("slow"))
		return nil
	}, sub.NoQueue())

	fast := New("multicast.partial.timeout.connector")
	fast.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("fast"))
		return nil
	}, sub.NoQueue())

	tooSlow := New("multicast.partial.timeout.connector")
	tooSlow.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		time.Sleep(delay * 4)
		w.Write([]byte("too slow"))
		return nil
	}, sub.NoQueue())

	// Startup the microservice
	err := slow.Startup()
	tt.NoError(err)
	defer slow.Shutdown()
	err = fast.Startup()
	tt.NoError(err)
	defer fast.Shutdown()
	err = tooSlow.Startup()
	tt.NoError(err)
	defer tooSlow.Shutdown()

	// Send the message
	shortCtx, cancel := context.WithTimeout(ctx, delay*3)
	defer cancel()
	var respondedOK, respondedErr int
	t0 := time.Now()
	ch := slow.Publish(
		shortCtx,
		pub.GET("https://multicast.partial.timeout.connector/cast"),
		pub.Multicast(),
	)
	dur := time.Since(t0)
	tt.True(dur >= 3*delay && dur < 4*delay)
	tt.Len(ch, 3)
	tt.Equal(3, cap(ch))
	for i := range ch {
		res, err := i.Get()
		if err == nil {
			body, err := io.ReadAll(res.Body)
			tt.NoError(err)
			tt.True(string(body) == "fast" || string(body) == "slow")
			respondedOK++
		} else {
			tt.Equal(http.StatusRequestTimeout, errors.Convert(err).StatusCode)
			respondedErr++
		}
	}
	tt.Equal(2, respondedOK)
	tt.Equal(1, respondedErr)
}

func TestConnector_MulticastError(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	bad := New("multicast.error.connector")
	bad.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("bad situation")
	}, sub.NoQueue())

	good := New("multicast.error.connector")
	good.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("good situation"))
		return nil
	}, sub.NoQueue())

	// Startup the microservice
	err := bad.Startup()
	tt.NoError(err)
	defer bad.Shutdown()
	err = good.Startup()
	tt.NoError(err)
	defer good.Shutdown()

	// Send the message
	var countErrs, countOKs int
	t0 := time.Now()
	ch := bad.Publish(ctx, pub.GET("https://multicast.error.connector/cast"), pub.Multicast())
	for i := range ch {
		_, err := i.Get()
		if err != nil {
			countErrs++
		} else {
			countOKs++
		}
	}
	dur := time.Since(t0)
	tt.True(dur >= good.ackTimeout && dur <= good.ackTimeout+time.Second)
	tt.Equal(1, countErrs)
	tt.Equal(1, countOKs)
}

func TestConnector_MulticastNotFound(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	con := New("multicast.not.found.connector")

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	// Send the message
	var count int
	t0 := time.Now()
	ch := con.Publish(ctx, pub.GET("https://multicast.not.found.connector/nowhere"), pub.Multicast())
	for i := range ch {
		i.Get()
		count++
	}
	dur := time.Since(t0)
	tt.True(dur >= con.ackTimeout && dur < con.ackTimeout+time.Second)
	tt.Zero(count)
}

func TestConnector_MassMulticast(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	ctx := context.Background()
	randomPlane := rand.AlphaNum64(12)
	N := 128

	// Create the client microservice
	client := New("client.mass.multicast.connector")
	client.SetDeployment(TESTING)
	client.SetPlane(randomPlane)

	err := client.Startup()
	tt.NoError(err)
	defer client.Shutdown()

	// Create the server microservices in parallel
	var wg sync.WaitGroup
	cons := make([]*Connector, N)
	for i := range N {
		wg.Add(1)
		go func() {
			cons[i] = New("mass.multicast.connector")
			cons[i].SetDeployment(TESTING)
			cons[i].SetPlane(randomPlane)
			cons[i].Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			}, sub.NoQueue())

			err := cons[i].Startup()
			tt.NoError(err)
			wg.Done()
		}()
	}
	wg.Wait()
	defer func() {
		var wg sync.WaitGroup
		for i := range N {
			wg.Add(1)
			go func() {
				err := cons[i].Shutdown()
				tt.NoError(err)
				wg.Done()
			}()
		}
		wg.Wait()
	}()

	// Send the message
	var countOKs int
	t0 := time.Now()
	ch := client.Publish(ctx, pub.GET("https://mass.multicast.connector/cast"), pub.Multicast())
	for i := range ch {
		_, err := i.Get()
		if tt.NoError(err) {
			countOKs++
		}
	}
	dur := time.Since(t0)
	tt.True(dur >= cons[0].ackTimeout && dur <= cons[0].ackTimeout+time.Second)
	tt.Equal(N, countOKs)
}

func BenchmarkConnector_NATSDirectPublishing(b *testing.B) {
	con := New("nats.direct.publishing.connector")

	err := con.Startup()
	testarossa.NoError(b, err)
	defer con.Shutdown()

	body := make([]byte, 512*1024)
	b.ResetTimer()
	for b.Loop() {
		con.natsConn.Publish("somewhere", body)
	}
	b.StopTimer()

	// On 2021 MacBook M1 Pro 16":
	// 128B: 82 ns/op
	// 256B: 104 ns/op
	// 512B: 153 ns/op
	// 1KB: 247 ns/op
	// 2KB: 410 ns/op
	// 4KB: 746 ns/op
	// 8KB: 1480 ns/op
	// 16KB: 2666 ns/op
	// 32KB: 5474 ns/op
	// 64KB: 9173 ns/op
	// 128KB: 16307 ns/op
	// 256KB: 32700 ns/op
	// 512KB: 63429 ns/op
}

func TestConnector_KnownResponders(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("known.responders.connector")
	alpha.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())

	beta := New("known.responders.connector")
	beta.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())

	gamma := New("known.responders.connector")
	gamma.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())

	delta := New("known.responders.connector")
	delta.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()
	err = gamma.Startup()
	tt.NoError(err)
	defer gamma.Shutdown()

	check := func() (count int, quick bool) {
		responded := map[string]bool{}
		t0 := time.Now()
		ch := alpha.Publish(ctx, pub.GET("https://known.responders.connector/cast"), pub.Multicast())
		for i := range ch {
			res, err := i.Get()
			if tt.NoError(err) {
				responded[frame.Of(res).FromID()] = true
			}
		}
		dur := time.Since(t0)
		return len(responded), dur < alpha.ackTimeout
	}

	// First request should be slower, consecutive requests should be quick
	count, quick := check()
	tt.Equal(3, count)
	tt.False(quick)
	count, quick = check()
	tt.Equal(3, count)
	tt.True(quick)
	count, quick = check()
	tt.Equal(3, count)
	tt.True(quick)

	// Add a new microservice
	err = delta.Startup()
	tt.NoError(err)

	// Should most likely get slow again once the new instance is discovered,
	// consecutive requests should be quick
	for count != 4 || !quick {
		count, quick = check()
	}
	count, quick = check()
	tt.Equal(4, count)
	tt.True(quick)

	// Remove a microservice
	delta.Shutdown()

	// Should get slow again, consecutive requests should be quick
	count, quick = check()
	tt.Equal(3, count)
	tt.False(quick)
	count, quick = check()
	tt.Equal(3, count)
	tt.True(quick)
}

func TestConnector_LifetimeCancellation(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("lifetime.cancellation.connector")

	done := false
	step := make(chan bool)
	con.Subscribe("GET", "something", func(w http.ResponseWriter, r *http.Request) error {
		step <- true
		<-r.Context().Done()
		done = true
		return r.Context().Err()
	})

	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	t0 := time.Now()
	go func() {
		_, err = con.Request(
			con.Lifetime(),
			pub.GET("https://lifetime.cancellation.connector/something"),
		)
		tt.Error(err)
		step <- true
	}()
	<-step
	con.ctxCancel()
	<-step
	tt.True(done)
	dur := time.Since(t0)
	tt.True(dur < time.Second)
}

func TestConnector_ChannelCapacity(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	n := 8

	// Create microservices that respond in a staggered timeline
	var responses atomic.Int32
	var wg sync.WaitGroup
	cons := make([]*Connector, n)
	for i := range n {
		wg.Add(1)
		go func() {
			cons[i] = New("channel.capacity.connector")
			cons[i].SetDeployment(TESTING)
			cons[i].Subscribe("GET", "multicast", func(w http.ResponseWriter, r *http.Request) error {
				time.Sleep(time.Duration(100*i+100) * time.Millisecond)
				responses.Add(1)
				return nil
			}, sub.NoQueue())
			err := cons[i].Startup()
			tt.NoError(err)
			wg.Done()
		}()
	}
	wg.Wait()
	defer func() {
		for i := range n {
			cons[i].Shutdown()
		}
	}()

	ctx := context.Background()

	// All responses should come in at once after all handlers finished
	responses.Store(0)
	t0 := time.Now()
	cons[0].multicastChanCap = n / 2 // Limited multicast channel capacity should not block
	ch := cons[0].Publish(
		ctx,
		pub.GET("https://channel.capacity.connector/multicast"),
	)
	tt.True(time.Since(t0) > time.Duration(n*100)*time.Millisecond)
	tt.Equal(n, int(responses.Load()))
	tt.Len(ch, n)
	tt.Equal(n, cap(ch))

	// If asking for first response only, it should return immediately when it is produced
	responses.Store(0)
	t0 = time.Now()
	ch = cons[0].Publish(
		ctx,
		pub.GET("https://channel.capacity.connector/multicast"),
		pub.Unicast(),
	)
	tt.True(time.Since(t0) > 100*time.Millisecond && time.Since(t0) < 200*time.Millisecond)
	tt.Equal(1, int(responses.Load()))
	tt.Len(ch, 1)
	tt.Equal(1, cap(ch))

	// The remaining handlers are still called and should finish
	time.Sleep(time.Duration(n*100) * time.Millisecond)
	tt.Equal(n, int(responses.Load()))
}

func TestConnector_UnicastToNoQueue(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	n := 8
	var wg sync.WaitGroup
	wg.Add(n)
	cons := make([]*Connector, n)
	for i := range n {
		go func() {
			cons[i] = New("unicast.to.no.queue.connector")
			cons[i].SetDeployment(TESTING)
			cons[i].Subscribe("GET", "no-queue", func(w http.ResponseWriter, r *http.Request) error {
				return nil
			}, sub.NoQueue())
			err := cons[i].Startup()
			tt.NoError(err)
			wg.Done()
		}()
	}
	wg.Wait()
	defer func() {
		for i := range n {
			cons[i].Shutdown()
		}
	}()

	_, err := cons[0].Request(
		cons[0].Lifetime(),
		pub.GET("https://unicast.to.no.queue.connector/no-queue"),
	)
	tt.NoError(err)
}

func TestConnector_Baggage(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.baggage.connector")

	betaCalled := false
	beta := New("beta.baggage.connector")
	beta.Subscribe("GET", "noop", func(w http.ResponseWriter, r *http.Request) error {
		betaCalled = true
		tt.Equal("Clothes", frame.Of(r).Baggage("Suitcase"))
		tt.Equal("en-US", r.Header.Get("Accept-Language"))
		tt.Equal("1.2.3.4", r.Header.Get("X-Forwarded-For"))
		// Call gamma without making changes
		beta.Request(r.Context(),
			pub.GET("https://gamma.baggage.connector/noop"),
		)
		return nil
	})

	gammaCalled := false
	gamma := New("gamma.baggage.connector")
	gamma.Subscribe("GET", "noop", func(w http.ResponseWriter, r *http.Request) error {
		gammaCalled = true
		tt.Equal("Clothes", frame.Of(r).Baggage("Suitcase"))
		tt.Equal("en-US", r.Header.Get("Accept-Language"))
		tt.Equal("1.2.3.4", r.Header.Get("X-Forwarded-For"))
		gamma.Request(r.Context(),
			// Call delta making changes to non-empty values
			pub.GET("https://delta.baggage.connector/noop"),
			pub.Baggage("Suitcase", "Books"),
			pub.Header("Accept-Language", "en-UK"),
			pub.Header("X-Forwarded-For", "11.22.33.44"),
		)
		return nil
	})

	deltaCalled := false
	delta := New("delta.baggage.connector")
	delta.Subscribe("GET", "noop", func(w http.ResponseWriter, r *http.Request) error {
		deltaCalled = true
		tt.Equal("Books", frame.Of(r).Baggage("Suitcase"))
		tt.Equal("en-UK", r.Header.Get("Accept-Language"))
		tt.Equal("11.22.33.44", r.Header.Get("X-Forwarded-For"))
		delta.Request(r.Context(),
			// Call epsilon making changes to empty values
			pub.GET("https://epsilon.baggage.connector/noop"),
			pub.Baggage("Suitcase", ""),
			pub.Header("Accept-Language", ""),
			pub.Header("X-Forwarded-For", ""),
		)
		return nil
	})

	epsilonCalled := false
	epsilon := New("epsilon.baggage.connector")
	epsilon.Subscribe("GET", "noop", func(w http.ResponseWriter, r *http.Request) error {
		epsilonCalled = true
		tt.Equal("", frame.Of(r).Baggage("Suitcase"))
		tt.Equal("", r.Header.Get("Accept-Language"))
		tt.Equal("", r.Header.Get("X-Forwarded-For"))
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()
	err = gamma.Startup()
	tt.NoError(err)
	defer gamma.Shutdown()
	err = delta.Startup()
	tt.NoError(err)
	defer delta.Shutdown()
	err = epsilon.Startup()
	tt.NoError(err)
	defer epsilon.Shutdown()

	// Send message and validate that it's echoed back
	_, err = alpha.Request(ctx,
		pub.GET("https://beta.baggage.connector/noop"),
		pub.Baggage("Suitcase", "Clothes"),
		pub.Baggage("Glass", "Full"),
		pub.Header("Accept-Language", "en-US"),
		pub.Header("X-Forwarded-For", "1.2.3.4"),
	)
	tt.NoError(err)
	tt.True(betaCalled)
	tt.True(gammaCalled)
	tt.True(deltaCalled)
	tt.True(epsilonCalled)
}

func TestConnector_MultiValueHeader(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.multi.value.header.connector")

	beta := New("beta.multi.value.header.connector")
	beta.Subscribe("GET", "receive", func(w http.ResponseWriter, r *http.Request) error {
		tt.Len(r.Header["Multi-Value-In"], 3)
		w.Header()["Multi-Value-Out"] = []string{"1", "2", "3"}
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// Send message and validate that it's echoed back
	response, err := alpha.Request(ctx,
		pub.GET("https://beta.multi.value.header.connector/receive"),
		pub.AddHeader("Multi-Value-In", "1"),
		pub.AddHeader("Multi-Value-In", "2"),
		pub.AddHeader("Multi-Value-In", "3"),
	)
	tt.NoError(err)
	tt.Len(response.Header["Multi-Value-Out"], 3)
}
