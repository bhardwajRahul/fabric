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
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/mem"
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

func TestConnector_Error(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.error.connector")

	beta := New("beta.error.connector")
	beta.Subscribe("GET", "err", func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("pattern %s %d", "XYZ", 123,
			http.StatusBadRequest,
			errors.New("original error"),
			"int", 888,
			"str", "ABC",
			"bool", true,
			"dur", time.Second,
			"time", time.Now(),
		)
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// Send message and validate that the error is received as sent
	_, err = alpha.GET(ctx, "https://beta.error.connector/err")
	tt.Error(err)
	tracedErr := errors.Convert(err)
	tt.Equal("pattern XYZ 123: original error", tracedErr.Error())
	tt.Equal(http.StatusBadRequest, tracedErr.StatusCode)
	tt.Equal(888.0, tracedErr.Properties["int"])
	tt.Equal("ABC", tracedErr.Properties["str"])
	tt.Equal(true, tracedErr.Properties["bool"])
	tt.Equal(float64(time.Second), tracedErr.Properties["dur"])
	_, parseErr := time.Parse(time.RFC3339, tracedErr.Properties["time"].(string))
	tt.NoError(parseErr)
}

func BenchmarkConnector_EchoSerial(b *testing.B) {
	ctx := context.Background()
	tt := testarossa.For(b)

	// Create the microservice
	alpha := New("alpha.echo.serial.connector")
	var echoCount atomic.Int32
	var errCount atomic.Int32
	alpha.Subscribe("POST", "echo", func(w http.ResponseWriter, r *http.Request) error {
		echoCount.Add(1)
		sz := 32 << 10
		if r.ContentLength > 0 {
			sz = int(r.ContentLength)
		}
		block := mem.Alloc(sz)
		for {
			n, err := io.ReadFull(r.Body, block[:sz])
			if n == 0 || err == io.EOF {
				break
			}
			if err != nil {
				errCount.Add(1)
			}
			w.Write(block[:n])
		}
		mem.Free(block)
		r.Body.Close()
		return nil
	})

	beta := New("beta.echo.serial.connector")

	for _, sc := range []int{1, 0} {
		env.Push("MICROBUS_SHORT_CIRCUIT", strconv.Itoa(sc))
		defer env.Pop("MICROBUS_SHORT_CIRCUIT")
		scDesc := "ShortCircuit"
		if sc == 0 {
			scDesc = "NATS"
		}
		b.Run(scDesc, func(b *testing.B) {
			// Startup the microservice
			err := alpha.Startup()
			tt.NoError(err)
			err = beta.Startup()
			tt.NoError(err)

			for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
				b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
					errCount.Store(0)
					echoCount.Store(0)
					var payload []byte
					if kb > 0 {
						payload = []byte(rand.AlphaNum64(kb << 10))
					}
					b.ResetTimer()
					for b.Loop() {
						res, err := beta.POST(ctx, "https://alpha.echo.serial.connector/echo", payload)
						if err != nil {
							errCount.Add(1)
						}
						if res != nil && res.Body != nil {
							buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
							io.Copy(buf, res.Body)
							mem.Free(buf.Bytes())
						}
					}
					b.StopTimer()
					testarossa.Zero(b, errCount.Load())
					testarossa.Equal(b, b.N, int(echoCount.Load()))
				})
			}

			alpha.Shutdown()
			beta.Shutdown()
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EchoSerial/ShortCircuit/0KB-10     	   96007	     11909 ns/op	    8232 B/op	     140 allocs/op
	// BenchmarkConnector_EchoSerial/ShortCircuit/1KB-10     	   94502	     12669 ns/op	    9481 B/op	     152 allocs/op
	// BenchmarkConnector_EchoSerial/ShortCircuit/16KB-10    	   72182	     16890 ns/op	   27124 B/op	     152 allocs/op
	// BenchmarkConnector_EchoSerial/ShortCircuit/256KB-10   	   20193	     59984 ns/op	  303974 B/op	     154 allocs/op
	// BenchmarkConnector_EchoSerial/ShortCircuit/960KB-10   	    9333	    110925 ns/op	 1094017 B/op	     155 allocs/op
	// BenchmarkConnector_EchoSerial/NATS/0KB-10             	    9336	    124917 ns/op	   15056 B/op	     233 allocs/op
	// BenchmarkConnector_EchoSerial/NATS/1KB-10             	    9142	    126795 ns/op	   18707 B/op	     247 allocs/op
	// BenchmarkConnector_EchoSerial/NATS/16KB-10            	    7446	    162489 ns/op	   81983 B/op	     250 allocs/op
	// BenchmarkConnector_EchoSerial/NATS/256KB-10           	    2427	    497678 ns/op	 1253900 B/op	     261 allocs/op
	// BenchmarkConnector_EchoSerial/NATS/960KB-10           	    1002	   1175914 ns/op	 3858598 B/op	     267 allocs/op
}

func BenchmarkConnector_EchoSerialHTTP(b *testing.B) {
	tt := testarossa.For(b)

	// Create the web server
	var echoCount atomic.Int32
	var errCount atomic.Int32
	httpServer := &http.Server{
		Addr: ":5555",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			echoCount.Add(1)
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				errCount.Add(1)
			}
			w.Write(body)
		}),
	}
	go httpServer.ListenAndServe()
	defer httpServer.Close()

	for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
		b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
			errCount.Store(0)
			echoCount.Store(0)
			payload := []byte(rand.AlphaNum64(kb << 10))
			b.ResetTimer()
			for b.Loop() {
				var payloadReader io.Reader
				if kb > 0 {
					payloadReader = bytes.NewReader(payload)
				}
				res, err := http.Post("http://127.0.0.1:5555/", "", payloadReader)
				if err != nil {
					errCount.Add(1)
				}
				if res != nil && res.Body != nil {
					buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
					io.Copy(buf, res.Body)
					mem.Free(buf.Bytes())
					res.Body.Close()
				}
			}
			b.StopTimer()
			tt.Zero(errCount.Load())
			tt.Equal(b.N, int(echoCount.Load()))
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EchoSerialHTTP/0KB-10         	   29139	     39602 ns/op	    6002 B/op	      62 allocs/op
	// BenchmarkConnector_EchoSerialHTTP/1KB-10         	   27632	     43538 ns/op	   12467 B/op	      84 allocs/op
	// BenchmarkConnector_EchoSerialHTTP/16KB-10        	   10000	    101181 ns/op	  156075 B/op	     111 allocs/op
	// BenchmarkConnector_EchoSerialHTTP/256KB-10       	    2814	    424454 ns/op	 2307765 B/op	     140 allocs/op
	// BenchmarkConnector_EchoSerialHTTP/960KB-10       	    1328	    905384 ns/op	 7419094 B/op	     155 allocs/op
}

func BenchmarkConnector_SerialChain(b *testing.B) {
	for _, sc := range []int{1, 0} {
		env.Push("MICROBUS_SHORT_CIRCUIT", strconv.Itoa(sc))
		defer env.Pop("MICROBUS_SHORT_CIRCUIT")
		scDesc := "ShortCircuit"
		if sc == 0 {
			scDesc = "NATS"
		}
		b.Run(scDesc, func(b *testing.B) {
			for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
				b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
					serialChain(b, kb<<10)
				})
			}
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_SerialChain/ShortCircuit/0KB-10     	    8552	    138005 ns/op	   78370 B/op	    1438 allocs/op
	// BenchmarkConnector_SerialChain/ShortCircuit/1KB-10     	    7393	    161923 ns/op	  124542 B/op	    1504 allocs/op
	// BenchmarkConnector_SerialChain/ShortCircuit/16KB-10    	    3991	    299136 ns/op	  823003 B/op	    1517 allocs/op
	// BenchmarkConnector_SerialChain/ShortCircuit/256KB-10   	     958	   1053469 ns/op	11707859 B/op	    1543 allocs/op
	// BenchmarkConnector_SerialChain/ShortCircuit/960KB-10   	     710	   1465405 ns/op	12669310 B/op	    1513 allocs/op
	// BenchmarkConnector_SerialChain/NATS/0KB-10             	     915	   1300616 ns/op	  156600 B/op	    2420 allocs/op
	// BenchmarkConnector_SerialChain/NATS/1KB-10             	     752	   1350357 ns/op	  213458 B/op	    2541 allocs/op
	// BenchmarkConnector_SerialChain/NATS/16KB-10            	     542	   1943614 ns/op	 1224985 B/op	    2631 allocs/op
	// BenchmarkConnector_SerialChain/NATS/256KB-10           	     177	   5752736 ns/op	17261974 B/op	    2741 allocs/op
	// BenchmarkConnector_SerialChain/NATS/960KB-10           	      61	  18121212 ns/op	57571406 B/op	    4227 allocs/op
}

func serialChain(b *testing.B, payloadSize int) {
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
			if res.Body != nil {
				buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
				io.Copy(buf, res.Body)
				w.Write(buf.Bytes())
				mem.Free(buf.Bytes())
			}
		} else {
			// Echo back the request
			echoCount.Add(1)
			if r.Body != nil {
				buf := bytes.NewBuffer(mem.Alloc(int(r.ContentLength)))
				io.Copy(buf, r.Body)
				w.Write(buf.Bytes())
				mem.Free(buf.Bytes())
			}
		}
		return nil
	})

	// Startup the microservice
	con.Startup()
	defer con.Shutdown()

	var errCount atomic.Int32
	echoCount.Store(0)
	payload := []byte(rand.AlphaNum64(payloadSize))
	b.ResetTimer()
	for b.Loop() {
		res, err := con.POST(ctx, "https://serial.chain.connector/echo", payload)
		if err != nil {
			errCount.Add(1)
		}
		if res != nil && res.Body != nil {
			buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
			io.Copy(buf, res.Body)
			mem.Free(buf.Bytes())
		}
	}
	b.StopTimer()
	testarossa.Zero(b, errCount.Load())
	testarossa.Equal(b, b.N, int(echoCount.Load()))
}

func BenchmarkConnector_EchoParallelNATS(b *testing.B) {
	env.Push("MICROBUS_SHORT_CIRCUIT", "0")
	defer env.Pop("MICROBUS_SHORT_CIRCUIT")
	for _, concurrency := range []int{100, 1000, 10000} {
		b.Run(strconv.Itoa(concurrency), func(b *testing.B) {
			for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
				b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
					echoParallel(b, concurrency, kb<<10)
				})
			}
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EchoParallelNATS/100/0KB-10     	   59695	     19347 ns/op	   14232 B/op	     242 allocs/op
	// BenchmarkConnector_EchoParallelNATS/100/1KB-10     	   49276	     20920 ns/op	   17872 B/op	     249 allocs/op
	// BenchmarkConnector_EchoParallelNATS/100/16KB-10    	   35426	     30551 ns/op	   72559 B/op	     250 allocs/op
	// BenchmarkConnector_EchoParallelNATS/100/256KB-10   	    6244	    183513 ns/op	  898301 B/op	     250 allocs/op
	// BenchmarkConnector_EchoParallelNATS/100/960KB-10   	    1514	    722379 ns/op	 3117985 B/op	     250 allocs/op
	// BenchmarkConnector_EchoParallelNATS/1000/0KB-10    	  110432	     10726 ns/op	   14044 B/op	     242 allocs/op
	// BenchmarkConnector_EchoParallelNATS/1000/1KB-10    	   88454	     11390 ns/op	   17444 B/op	     249 allocs/op
	// BenchmarkConnector_EchoParallelNATS/1000/16KB-10   	   59472	     19346 ns/op	   70074 B/op	     249 allocs/op
	// BenchmarkConnector_EchoParallelNATS/1000/256KB-10  	    5665	    195763 ns/op	  918751 B/op	     249 allocs/op
	// BenchmarkConnector_EchoParallelNATS/1000/960KB-10  	    1594	    866156 ns/op	 3619096 B/op	     251 allocs/op
	// BenchmarkConnector_EchoParallelNATS/10000/0KB-10   	  120158	      9422 ns/op	   14046 B/op	     242 allocs/op
	// BenchmarkConnector_EchoParallelNATS/10000/1KB-10   	  110713	     10119 ns/op	   17623 B/op	     249 allocs/op
	// BenchmarkConnector_EchoParallelNATS/10000/16KB-10  	   56052	     21480 ns/op	   72994 B/op	     250 allocs/op
	// BenchmarkConnector_EchoParallelNATS/10000/256KB-10 	    5626	    288611 ns/op	 1304787 B/op	     252 allocs/op
	// BenchmarkConnector_EchoParallelNATS/10000/960KB-10 	    1402	    741374 ns/op	 3914731 B/op	     252 allocs/op
}

func BenchmarkConnector_EchoParallelShortCircuit(b *testing.B) {
	env.Push("MICROBUS_SHORT_CIRCUIT", "1")
	defer env.Pop("MICROBUS_SHORT_CIRCUIT")
	for _, concurrency := range []int{100, 1000, 10000} {
		b.Run(strconv.Itoa(concurrency), func(b *testing.B) {
			for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
				b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
					echoParallel(b, concurrency, kb<<10)
				})
			}
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EchoParallelShortCircuit/100/0KB-10     	  137668	      8848 ns/op	    8461 B/op	     150 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/100/1KB-10     	   91471	     11961 ns/op	   15931 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/100/16KB-10    	   55020	     19815 ns/op	  125466 B/op	     162 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/100/256KB-10   	   24858	     41754 ns/op	 1852003 B/op	     162 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/100/960KB-10   	   30968	     36537 ns/op	 1051782 B/op	     155 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/1000/0KB-10    	  180338	      6565 ns/op	    8423 B/op	     150 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/1000/1KB-10    	  153175	      7736 ns/op	   15857 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/1000/16KB-10   	   77114	     14116 ns/op	  125330 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/1000/256KB-10  	   32976	     33442 ns/op	 1851825 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/1000/960KB-10  	   43832	     27346 ns/op	 1013508 B/op	     155 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/10000/0KB-10   	  181414	      6494 ns/op	    8451 B/op	     150 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/10000/1KB-10   	  150331	      7381 ns/op	   15875 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/10000/16KB-10  	   76665	     14356 ns/op	  125368 B/op	     161 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/10000/256KB-10 	   23038	     48433 ns/op	 1852014 B/op	     163 allocs/op
	// BenchmarkConnector_EchoParallelShortCircuit/10000/960KB-10 	   36306	     32730 ns/op	 1007266 B/op	     156 allocs/op
}

func echoParallel(b *testing.B, concurrency int, payloadSize int) {
	ctx := context.Background()
	if concurrency <= 0 || concurrency > b.N {
		concurrency = b.N
	}

	// Create the microservice
	alpha := New("alpha.echo.parallel.connector")
	var echoCount atomic.Int32
	var errCount atomic.Int32
	alpha.Subscribe("ANY", "echo", func(w http.ResponseWriter, r *http.Request) error {
		echoCount.Add(1)
		buf := bytes.NewBuffer(mem.Alloc(int(r.ContentLength)))
		io.Copy(buf, r.Body)
		w.Write(buf.Bytes())
		mem.Free(buf.Bytes())
		return nil
	})

	beta := New("beta.echo.parallel.connector")
	beta.ackTimeout = time.Second

	// Startup the microservice
	alpha.Startup()
	defer alpha.Shutdown()
	beta.Startup()
	defer beta.Shutdown()

	var payload []byte
	if payloadSize > 0 {
		payload = []byte(rand.AlphaNum64(payloadSize))
	}
	var wg sync.WaitGroup
	wg.Add(b.N)
	b.ResetTimer()
	for i := range concurrency {
		tot := b.N / concurrency
		if i < b.N%concurrency {
			tot++
		} // do remainder
		go func() {
			for range tot {
				var err error
				var res *http.Response
				res, err = beta.POST(ctx, "https://alpha.echo.parallel.connector/echo", payload)
				if err != nil {
					errCount.Add(1)
				}
				if res != nil && res.Body != nil {
					buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
					io.Copy(buf, res.Body)
					mem.Free(buf.Bytes())
				}
				wg.Done()
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
	testarossa.Zero(b, errCount.Load())
	testarossa.Equal(b, b.N, int(echoCount.Load()))
}

func BenchmarkConnector_EchoParallelHTTP(b *testing.B) {
	for _, concurrency := range []int{100, 1000} {
		b.Run(strconv.Itoa(concurrency), func(b *testing.B) {
			for _, kb := range []int{0, 1, 16, 256, 1024 - 64} {
				b.Run(strconv.Itoa(kb)+"KB", func(b *testing.B) {
					echoParallelHTTP(b, concurrency, kb<<10)
				})
			}
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EchoParallelHTTP/100/0KB-10     	   39246	     70404 ns/op	   10326 B/op	      87 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/100/1KB-10     	    1731	    735920 ns/op	   22520 B/op	     179 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/100/16KB-10    	    1590	    809513 ns/op	   37576 B/op	     184 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/100/256KB-10   	    1228	   1027616 ns/op	   57610 B/op	     185 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/100/960KB-10   	     990	   1031351 ns/op	   59558 B/op	     199 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/1000/0KB-10    	    1249	    828319 ns/op	   21197 B/op	     141 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/1000/1KB-10    	   18120	    193431 ns/op	   21159 B/op	     171 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/1000/16KB-10   	   17271	    202879 ns/op	   34985 B/op	     175 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/1000/256KB-10  	    1176	    932505 ns/op	   70952 B/op	     187 allocs/op
	// BenchmarkConnector_EchoParallelHTTP/1000/960KB-10  	    1027	   1063508 ns/op	  161415 B/op	     204 allocs/op
}

func echoParallelHTTP(b *testing.B, concurrency int, payloadSize int) {
	ctx := context.Background()
	if concurrency <= 0 || concurrency > b.N {
		concurrency = b.N
	}
	// Create the web server
	var echoCount atomic.Int32
	var errCount atomic.Int32
	httpServer := &http.Server{
		Addr: ":5555",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			echoCount.Add(1)
			if r.Body != nil {
				buf := bytes.NewBuffer(mem.Alloc(int(r.ContentLength)))
				io.Copy(buf, r.Body)
				w.Write(buf.Bytes())
				mem.Free(buf.Bytes())
				r.Body.Close()
			}
		}),
	}
	go httpServer.ListenAndServe()
	defer httpServer.Shutdown(ctx)

	payload := []byte(rand.AlphaNum64(payloadSize))
	var wg sync.WaitGroup
	wg.Add(b.N)
	b.ResetTimer()
	for i := range concurrency {
		tot := b.N / concurrency
		if i < b.N%concurrency {
			tot++
		} // do remainder
		go func() {
			for range tot {
				var err error
				var res *http.Response
				if payloadSize > 0 {
					res, err = http.Post("http://127.0.0.1:5555/", "", bytes.NewReader(payload))
				} else {
					res, err = http.Get("http://127.0.0.1:5555/")
				}
				if err != nil {
					errCount.Add(1)
				}
				if res != nil && res.Body != nil {
					buf := bytes.NewBuffer(mem.Alloc(int(res.ContentLength)))
					io.Copy(buf, res.Body)
					mem.Free(buf.Bytes())
					res.Body.Close()
				}
				wg.Done()
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
	testarossa.Zero(b, errCount.Load())
	testarossa.Equal(b, b.N, int(echoCount.Load()))
}

func TestConnector_EchoParallelCapacity(t *testing.T) {
	// No parallel
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
			res, err := beta.POST(ctx, "https://alpha.echo.parallel.capacity.connector/echo", []byte("Hello"))
			if err != nil {
				errCount.Add(1)
			}
			if res != nil && res.Body != nil {
				io.ReadAll(res.Body)
				res.Body.Close()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	avgTime := totalTime.Load() / int64(n)
	tt.Zero(errCount.Load())
	tt.Equal(int32(n), echoCount.Load())
	tt.True(avgTime <= 250, avgTime)                // 250ms
	tt.True(maxTime.Load() <= 1000, maxTime.Load()) // 1 sec

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
	tt.True(count1 > 64, count1)
	tt.True(count2 > 64, count2)
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
		i.SetDeployment(LAB) // Known responders optimization is disabled in TESTING so use LAB

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
	N := 256

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

func TestConnector_KnownResponders(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("known.responders.connector")
	alpha.SetDeployment(LAB) // Known responders optimization is disabled in TESTING so use LAB
	alpha.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()

	beta := New("known.responders.connector")
	beta.SetDeployment(LAB) // Known responders optimization is disabled in TESTING so use LAB
	beta.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	gamma := New("known.responders.connector")
	gamma.SetDeployment(LAB) // Known responders optimization is disabled in TESTING so use LAB
	gamma.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())
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
	delta := New("known.responders.connector")
	delta.SetDeployment(LAB) // Known responders optimization is disabled in TESTING so use LAB
	delta.Subscribe("GET", "cast", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}, sub.NoQueue())
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
	tt.True(time.Since(t0) > 100*time.Millisecond && time.Since(t0) < 200*time.Millisecond, time.Since(t0))
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
