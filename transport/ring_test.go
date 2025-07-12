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
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestTransport_Ring(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var ring ringList
	called := make([]bool, 5)
	h1 := func(msg *Msg) { called[1] = true }
	h2 := func(msg *Msg) { called[2] = true }
	h3 := func(msg *Msg) { called[3] = true }
	h4 := func(msg *Msg) { called[4] = true }

	// Insert
	tt.True(ring.IsEmpty())
	gem1 := ring.Insert(h1)
	gem2 := ring.Insert(h2)
	gem3 := ring.Insert(h3)
	tt.Equal("{1,2,3}", ring.String())
	tt.False(ring.IsEmpty())

	tt.False(called[1])
	gem1.Handler(nil)
	tt.True(called[1])
	tt.False(called[2])
	gem2.Handler(nil)
	tt.True(called[2])
	tt.False(called[3])
	gem3.Handler(nil)
	tt.True(called[3])

	// Rotate
	tt.Equal(gem1, ring.Head())
	tt.Equal(gem1, ring.Rotate())
	tt.Equal(gem2, ring.Head())

	tt.Equal("{2,3,1}", ring.String())
	tt.Equal(gem2, ring.Rotate())
	tt.Equal("{3,1,2}", ring.String())
	tt.Equal(gem3, ring.Rotate())
	tt.Equal("{1,2,3}", ring.String())
	tt.Equal(gem1, ring.Rotate())
	tt.Equal("{2,3,1}", ring.String())

	// Remove
	tt.True(ring.Remove(gem3))
	tt.Equal("{2,1}", ring.String())
	tt.False(ring.Remove(gem3))
	tt.True(ring.Remove(gem1))
	tt.Equal("{2}", ring.String())
	tt.True(ring.Remove(gem2))
	tt.Equal("{}", ring.String())
	tt.True(ring.IsEmpty())

	// Rotate empty ring
	tt.Nil(ring.Rotate())
	tt.Equal("{}", ring.String())

	// Rotate single gem
	gem4 := ring.Insert(h4)
	tt.Equal("{4}", ring.String())
	tt.Equal(gem4, ring.Rotate())
	tt.Equal("{4}", ring.String())
}

func TestTransport_Remove(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var ring ringList
	h1 := func(msg *Msg) {}
	h2 := func(msg *Msg) {}
	h3 := func(msg *Msg) {}
	h4 := func(msg *Msg) {}

	// Populate the ring
	gem1 := ring.Insert(h1)
	gem2 := ring.Insert(h2)
	gem3 := ring.Insert(h3)
	gem4 := ring.Insert(h4)

	tt.Equal(gem1, ring.Head())
	tt.Equal("{1,2,3,4}", ring.String())
	tt.Equal(gem1, ring.Head())

	// Delete non-head gem
	ring.Remove(gem3)
	tt.Equal(gem1, ring.Head())
	tt.Equal("{1,2,4}", ring.String())
	tt.Equal(gem1, ring.Head())

	// Delete head node
	ring.Remove(gem1)
	tt.Equal(gem2, ring.Head())
	tt.Equal("{2,4}", ring.String())
	tt.Equal(gem2, ring.Head())

	// Delete all nodes
	ring.Remove(gem2)
	ring.Remove(gem4)
	tt.Equal("{}", ring.String())
	tt.Nil(ring.Head())
	ring.Rotate()
	tt.Nil(ring.Head())
}
