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
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"
)

func TestConnector_Frag(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservice
	var bodyReceived []byte
	con := New("frag.connector")
	con.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		var err error
		bodyReceived, err = io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(bodyReceived)
		return nil
	})

	// Startup the microservice
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()
	con.maxFragmentSize = 128

	// Prepare the body to send
	bodySent := []byte(rand.AlphaNum64(int(con.maxFragmentSize)*2 + 16))

	// Send message and validate that it was received whole
	res, err := con.POST(ctx, "https://frag.connector/big", bodySent)
	if tt.NoError(err) {
		tt.Equal(bodySent, bodyReceived)
		bodyResponded, err := io.ReadAll(res.Body)
		if tt.NoError(err) {
			tt.Equal(bodySent, bodyResponded)
		}
	}
}

func TestConnector_FragMulticast(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	var alphaBodyReceived []byte
	alpha := New("frag.multicast.connector")
	alpha.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		var err error
		alphaBodyReceived, err = io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(alphaBodyReceived)
		return nil
	}, sub.NoQueue())

	var betaBodyReceived []byte
	beta := New("frag.multicast.connector")
	beta.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		var err error
		betaBodyReceived, err = io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(betaBodyReceived)
		return nil
	}, sub.NoQueue())

	// Startup the microservice
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	alpha.maxFragmentSize = 1024
	beta.maxFragmentSize = 1024

	// Prepare the body to send
	bodySent := rand.AlphaNum64(int(alpha.maxFragmentSize)*2 + 16)

	// Send message and validate that it was received whole
	ch := alpha.Publish(
		ctx,
		pub.POST("https://frag.multicast.connector/big"),
		pub.Body(bodySent),
	)
	for r := range ch {
		res, err := r.Get()
		tt.NoError(err)
		bodyResponded, err := io.ReadAll(res.Body)
		tt.NoError(err)
		tt.Equal(bodySent, string(bodyResponded))
	}
	tt.Equal(bodySent, string(alphaBodyReceived))
	tt.Equal(bodySent, string(betaBodyReceived))
}

func TestConnector_FragLoadBalanced(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	var alphaBodyReceived []byte
	alpha := New("frag.load.balanced.connector")
	alpha.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		var err error
		alphaBodyReceived, err = io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(alphaBodyReceived)
		return nil
	}, sub.LoadBalanced())

	var betaBodyReceived []byte
	beta := New("frag.load.balanced.connector")
	beta.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		var err error
		betaBodyReceived, err = io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(betaBodyReceived)
		return nil
	}, sub.LoadBalanced())

	// Startup the microservice
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	alpha.maxFragmentSize = 128
	beta.maxFragmentSize = 128

	// Prepare the body to send
	bodySent := []byte(rand.AlphaNum64(int(alpha.maxFragmentSize)*2 + 16))

	// Send message and validate that it was received whole
	ch := alpha.Publish(
		ctx,
		pub.POST("https://frag.load.balanced.connector/big"),
		pub.Body(bodySent),
	)
	for r := range ch {
		res, err := r.Get()
		if tt.NoError(err) {
			bodyResponded, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal(bodySent, bodyResponded)
			}
		}
	}
	if alphaBodyReceived != nil {
		tt.Equal(bodySent, alphaBodyReceived)
		tt.Nil(betaBodyReceived)
	} else {
		tt.Equal(bodySent, betaBodyReceived)
		tt.Nil(alphaBodyReceived)
	}
}

func BenchmarkConnector_Frag(b *testing.B) {
	ctx := context.Background()
	tt := testarossa.For(b)

	// Create the microservice
	alpha := New("alpha.frag.benchmark.connector")
	alpha.Subscribe("POST", "big", func(w http.ResponseWriter, r *http.Request) error {
		body, err := io.ReadAll(r.Body)
		tt.NoError(err)
		w.Write(body)
		return nil
	})
	beta := New("beta.frag.benchmark.connector")

	for _, sc := range []int{1, 0} {
		env.Push("MICROBUS_SHORT_CIRCUIT", strconv.Itoa(sc))
		defer env.Pop("MICROBUS_SHORT_CIRCUIT")
		scDesc := "ShortCircuit"
		if sc == 0 {
			scDesc = "NATS"
		}

		// Startup the microservice
		err := alpha.Startup()
		tt.NoError(err)
		err = beta.Startup()
		tt.NoError(err)

		b.Run(scDesc, func(b *testing.B) {
			for i := 16; i <= 256; i *= 2 {
				b.Run(fmt.Sprintf("%dMB", i), func(b *testing.B) {
					payload := []byte(rand.AlphaNum64(i << 20))
					for b.Loop() {
						// Send message and validate that it was received whole
						res, err := beta.POST(ctx, "https://alpha.frag.benchmark.connector/big", payload)
						tt.NoError(err)
						_, err = io.ReadAll(res.Body)
						tt.NoError(err)
					}
				})
			}
		})

		alpha.Shutdown()
		beta.Shutdown()
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_Frag/ShortCircuit/16MB-10    	     104	  10836000 ns/op	239087003 B/op	    1786 allocs/op
	// BenchmarkConnector_Frag/ShortCircuit/32MB-10    	      68	  17150017 ns/op	469799678 B/op	    3242 allocs/op
	// BenchmarkConnector_Frag/ShortCircuit/64MB-10    	      34	  32327444 ns/op	922086133 B/op	    6152 allocs/op
	// BenchmarkConnector_Frag/ShortCircuit/128MB-10   	      20	  61615944 ns/op	1808722773 B/op	   12304 allocs/op
	// BenchmarkConnector_Frag/ShortCircuit/256MB-10   	       9	 139992468 ns/op	3546831160 B/op	   24521 allocs/op
	// BenchmarkConnector_Frag/NATS/16MB-10            	      51	  22125905 ns/op	274420436 B/op	    3664 allocs/op
	// BenchmarkConnector_Frag/NATS/32MB-10            	      30	  38609486 ns/op	539496518 B/op	    6890 allocs/op
	// BenchmarkConnector_Frag/NATS/64MB-10            	      15	  73157689 ns/op	1060707498 B/op	   13339 allocs/op
	// BenchmarkConnector_Frag/NATS/128MB-10           	       7	 144222399 ns/op	2082628163 B/op	   26517 allocs/op
	// BenchmarkConnector_Frag/NATS/256MB-10           	       4	 320048896 ns/op	4094226782 B/op	   52890 allocs/op
}

func TestConnector_DefragRequest(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("defrag.request.connector")
	makeChunk := func(msgID string, fragIndex int, fragMax int, content string) *http.Request {
		r, err := http.NewRequest("GET", "", strings.NewReader(content))
		tt.NoError(err)
		f := frame.Of(r)
		f.SetFromID("12345678")
		f.SetMessageID(msgID)
		f.SetFragment(fragIndex, fragMax)
		return r
	}

	// One chunk only: should return the exact same object
	r := makeChunk("one", 1, 1, strings.Repeat("1", 1024))
	integrated, err := con.defragRequest(r)
	if tt.NoError(err) {
		tt.Equal(r, integrated)
	}

	// Three chunks: should return the integrated chunk after the final chunk
	r = makeChunk("three", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragRequest(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("three", 2, 3, strings.Repeat("2", 1024))
	integrated, err = con.defragRequest(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("three", 3, 3, strings.Repeat("3", 1024))
	integrated, err = con.defragRequest(r)
	if tt.NoError(err) && tt.NotNil(integrated) {
		body, err := io.ReadAll(integrated.Body)
		if tt.NoError(err) {
			tt.Equal(strings.Repeat("1", 1024)+strings.Repeat("2", 1024)+strings.Repeat("3", 1024), string(body))
		}
	}

	// Three chunks not in order: should return the integrated chunk after the final chunk
	r = makeChunk("outoforder", 3, 3, strings.Repeat("3", 1024))
	integrated, err = con.defragRequest(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("outoforder", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragRequest(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("outoforder", 2, 3, strings.Repeat("2", 1024))
	integrated, err = con.defragRequest(r)
	if tt.NoError(err) && tt.NotNil(integrated) {
		body, err := io.ReadAll(integrated.Body)
		if tt.NoError(err) {
			tt.Equal(strings.Repeat("1", 1024)+strings.Repeat("2", 1024)+strings.Repeat("3", 1024), string(body))
		}
	}

	// Taking too long: should timeout
	r = makeChunk("delayed", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragRequest(r)
	tt.NoError(err)
	tt.Nil(integrated)
	time.Sleep(con.networkHop * (fragTimeoutMultiplier + 2))
	r = makeChunk("delayed", 2, 3, strings.Repeat("2", 1024))
	_, err = con.defragRequest(r)
	tt.Error(err)
	r = makeChunk("delayed", 3, 3, strings.Repeat("3", 1024))
	_, err = con.defragRequest(r)
	tt.Error(err)
}

func TestConnector_DefragResponse(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("defrag.response.connector")
	makeChunk := func(msgID string, fragIndex int, fragMax int, content string) *http.Response {
		w := httptest.NewRecorder()
		f := frame.Of(w)
		f.SetFromID("12345678")
		f.SetMessageID(msgID)
		f.SetFragment(fragIndex, fragMax)
		w.Write([]byte(content))
		return w.Result()
	}

	// One chunk only: should return the exact same object
	r := makeChunk("one", 1, 1, strings.Repeat("1", 1024))
	integrated, err := con.defragResponse(r)
	if tt.NoError(err) {
		tt.Equal(r, integrated)
	}

	// Three chunks: should return the integrated chunk after the final chunk
	r = makeChunk("three", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragResponse(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("three", 2, 3, strings.Repeat("2", 1024))
	integrated, err = con.defragResponse(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("three", 3, 3, strings.Repeat("3", 1024))
	integrated, err = con.defragResponse(r)
	if tt.NoError(err) && tt.NotNil(integrated) {
		body, err := io.ReadAll(integrated.Body)
		if tt.NoError(err) {
			tt.Equal(strings.Repeat("1", 1024)+strings.Repeat("2", 1024)+strings.Repeat("3", 1024), string(body))
		}
	}

	// Three chunks not in order: should return the integrated chunk after the final chunk
	r = makeChunk("outoforder", 3, 3, strings.Repeat("3", 1024))
	integrated, err = con.defragResponse(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("outoforder", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragResponse(r)
	tt.NoError(err)
	tt.Nil(integrated)
	r = makeChunk("outoforder", 2, 3, strings.Repeat("2", 1024))
	integrated, err = con.defragResponse(r)
	if tt.NoError(err) && tt.NotNil(integrated) {
		body, err := io.ReadAll(integrated.Body)
		if tt.NoError(err) {
			tt.Equal(strings.Repeat("1", 1024)+strings.Repeat("2", 1024)+strings.Repeat("3", 1024), string(body))
		}
	}

	// Taking too long: should timeout
	r = makeChunk("delayed", 1, 3, strings.Repeat("1", 1024))
	integrated, err = con.defragResponse(r)
	tt.NoError(err)
	tt.Nil(integrated)
	time.Sleep(con.networkHop * (fragTimeoutMultiplier + 2))
	r = makeChunk("delayed", 2, 3, strings.Repeat("2", 1024))
	_, err = con.defragResponse(r)
	tt.Error(err)
	r = makeChunk("delayed", 3, 3, strings.Repeat("3", 1024))
	_, err = con.defragResponse(r)
	tt.Error(err)
}
