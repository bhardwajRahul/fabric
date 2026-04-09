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

package shell

import (
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestShell_CappedBufferUnderCapacity(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(100)
	n, err := b.Write([]byte("hello world"))
	assert.Expect(n, 11, err, nil)
	assert.Equal(b.String(), "hello world")
}

func TestShell_CappedBufferAtCapacity(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(10)
	n, err := b.Write([]byte("0123456789"))
	assert.Expect(n, 10, err, nil)
	assert.Equal(b.String(), "0123456789")
	assert.NotContains(b.String(), "truncated")
}

func TestShell_CappedBufferOverflow(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(10) // head=5, tail=5
	n, err := b.Write([]byte("AAAAA-MIDDLE-ZZZZZ"))
	assert.Expect(n, 18, err, nil)
	out := b.String()
	assert.Contains(out, "AAAAA")
	assert.Contains(out, "ZZZZZ")
	assert.Contains(out, "truncated 8 bytes")
	assert.NotContains(out, "MIDDLE")
}

func TestShell_CappedBufferManySmallWrites(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(10) // head=5, tail=5
	for i := 0; i < 100; i++ {
		_, _ = b.Write([]byte{byte('A' + i%26)})
	}
	out := b.String()
	assert.Contains(out, "truncated 90 bytes")
	// Head must be the first 5 bytes ever written: A,B,C,D,E.
	headEnd := strings.Index(out, "\n")
	assert.Equal(out[:headEnd], "ABCDE")
	// Tail must be the last 5 bytes ever written. i=95..99 -> 95%26=17(R),18(S),19(T),20(U),21(V).
	assert.True(strings.HasSuffix(out, "RSTUV"))
}

func TestShell_CappedBufferEmpty(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(100)
	assert.Equal(b.String(), "")
}

func TestShell_CappedBufferLargeSingleWrite(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	b := newCappedBuffer(20) // head=10, tail=10
	payload := make([]byte, 1000)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}
	n, err := b.Write(payload)
	assert.Expect(n, 1000, err, nil)
	out := b.String()
	assert.Contains(out, "truncated 980 bytes")
	// First 10 bytes: a..j
	assert.True(strings.HasPrefix(out, "abcdefghij"))
	// Last 10 bytes: payload[990..999] -> 990%26=2(c)..11%26=11(l) -> "cdefghijkl"
	assert.True(strings.HasSuffix(out, "cdefghijkl"))
}
