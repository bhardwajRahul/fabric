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

package transport

import (
	"context"
	"fmt"
	"testing"

	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
	"github.com/nats-io/nats.go"
)

func BenchmarkTransport_NATSDirectPublishing(b *testing.B) {
	tt := testarossa.For(b)

	cn, err := nats.Connect("https://127.0.0.1:4222")
	tt.NoError(err)
	defer cn.Close()

	s, err := cn.Subscribe("somewhere", func(msg *nats.Msg) {})
	tt.NoError(err)
	defer s.Unsubscribe()

	for i := 128; i <= 512<<10; i *= 2 {
		desc := fmt.Sprintf("%dB", i)
		if i >= 1<<10 {
			desc = fmt.Sprintf("%dKB", i>>10)
		}
		b.Run(desc, func(b *testing.B) {
			body := []byte(rand.AlphaNum64(i))
			b.ResetTimer()
			for b.Loop() {
				cn.Publish("somewhere", body)
			}
		})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/transport
	// cpu: Apple M1 Pro
	// BenchmarkTransport_NATSDirectPublishing/128B-10         	 2970085	       392.6 ns/op	     255 B/op	       2 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/256B-10         	 2683281	       442.1 ns/op	     384 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/512B-10         	 2212436	       546.4 ns/op	     639 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/1KB-10          	 2101729	       573.4 ns/op	    1132 B/op	       2 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/2KB-10          	 1337725	       896.5 ns/op	    2173 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/4KB-10          	  784214	      1667 ns/op	    4222 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/8KB-10          	  370716	      3179 ns/op	    8298 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/16KB-10         	  193784	      6885 ns/op	   16723 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/32KB-10         	   80478	     15101 ns/op	   32875 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/64KB-10         	   52527	     20811 ns/op	   64890 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/128KB-10        	   31376	     38782 ns/op	  131063 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/256KB-10        	   15942	     75784 ns/op	  262136 B/op	       3 allocs/op
	// BenchmarkTransport_NATSDirectPublishing/512KB-10        	    8608	    146453 ns/op	  524411 B/op	       3 allocs/op
}

func TestTransport_LingeringSubscriptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := testarossa.For(t)

	h := func(msg *Msg) {}
	var c Conn
	err := c.Open(ctx, nil)
	tt.NoError(err)

	s1, err := c.Subscribe("s1.subject", h)
	tt.NoError(err)
	s2, err := c.Subscribe("s2.subject", h)
	tt.NoError(err)
	s3, err := c.Subscribe("s3.subject", h)
	tt.NoError(err)
	// 3 -> 2 -> 1
	tt.Equal(s3, c.head)
	tt.Equal(s2, c.head.next)
	tt.Equal(s1, c.head.next.next)
	tt.Nil(c.head.next.next.next)
	tt.Nil(s3.prev)
	tt.Equal(s3.next, s2)
	tt.Equal(s2.prev, s3)
	tt.Equal(s2.next, s1)
	tt.Equal(s1.prev, s2)
	tt.Nil(s1.next)

	tt.False(s2.done)
	if c.shortCircuitEnabled {
		tt.NotNil(s2.shortCircuitUnsub)
	}
	err = s2.Unsubscribe()
	tt.NoError(err)
	err = s2.Unsubscribe()
	tt.NoError(err)
	tt.True(s2.done)
	if c.shortCircuitEnabled {
		tt.Nil(s2.shortCircuitUnsub)
	}
	// 3 -> 1
	tt.Equal(s3, c.head)
	tt.Equal(s1, c.head.next)
	tt.Nil(c.head.next.next)
	tt.Nil(s3.prev)
	tt.Equal(s3.next, s1)
	tt.Nil(s2.prev)
	tt.Nil(s2.next)
	tt.Equal(s1.prev, s3)
	tt.Nil(s1.next)

	err = s3.Unsubscribe()
	tt.NoError(err)
	s4, err := c.Subscribe("s4.subject", h)
	tt.NoError(err)
	err = s4.Unsubscribe()
	tt.NoError(err)
	// 1
	tt.Equal(s1, c.head)
	tt.Nil(c.head.next)
	tt.Nil(s4.prev)
	tt.Nil(s4.next)
	tt.Nil(s3.prev)
	tt.Nil(s3.next)
	tt.Nil(s2.prev)
	tt.Nil(s2.next)
	tt.Nil(s1.prev)
	tt.Nil(s1.next)

	if c.shortCircuitEnabled {
		tt.False(shortCircuit.IsEmpty())
	}
	err = c.Close()
	tt.NoError(err)
	tt.Nil(c.head)
	if c.shortCircuitEnabled {
		tt.True(shortCircuit.IsEmpty())
	}
}
