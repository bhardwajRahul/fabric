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

package mem

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/microbus-io/testarossa"
)

func TestMem_Recycle(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	block1 := Alloc(1024)
	tt.Equal(0, len(block1))
	tt.Equal(1024, cap(block1))
	block1[:1][0] = '1'
	tt.Equal([]byte("1"), block1[:1])

	block2 := Alloc(1024)
	tt.Equal(0, len(block2))
	tt.Equal(1024, cap(block2))
	block2[:1][0] = '2'
	tt.Equal([]byte("2"), block2[:1])
	tt.NotEqual([]byte("2"), block1[:1])

	Free(block1)

	block3 := Alloc(1024)
	tt.Equal(0, len(block3))
	tt.Equal(1024, cap(block3))
	tt.Equal(unsafe.SliceData(block1), unsafe.SliceData(block3))
	tt.Equal([]byte("1"), block1[:1])
}

func TestMem_Grow(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	block1 := Alloc(2<<10 + 1)
	tt.Len(block1, 0)
	tt.Equal(4<<10, cap(block1))

	buf := append(block1, bytes.Repeat([]byte{'1'}, 1<<10)...)
	tt.Len(buf, 1<<10)
	tt.Equal(4<<10, cap(buf))
	tt.Equal(byte('1'), block1[:1][0])
	buf[0] = '2'
	tt.Equal(byte('2'), block1[:1][0])

	buf = append(buf, bytes.Repeat([]byte{'1'}, 4<<10)...)
	tt.Len(buf, 5<<10)
	tt.True(cap(buf) >= 5<<10)
	tt.Len(block1, 0)
	tt.Equal(byte('2'), block1[:1][0])
	buf[0] = '3'
	tt.Equal(byte('2'), block1[:1][0])

	buf = nil
	Free(block1)

	block2 := Alloc(2<<10 + 2)
	tt.Len(block2, 0)
	tt.Equal(4<<10, cap(block2))
	tt.Equal(byte('2'), block2[:1][0])
}

func TestMem_TooLarge(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	block1 := Alloc(1 << (12 + 1 + 10 + 1)) // 8MB
	block1 = append(block1, []byte("X2865374563X")...)
	Free(block1)
	block2 := Alloc(1 << (12 + 1 + 10 + 1)) // Same 8MB
	tt.NotEqual([]byte("X2865374563X"), block2[:12])
	Free(block2)
}

func TestMem_Copy(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	src := Alloc(1<<10 + 16)
	src = append(src, bytes.Repeat([]byte{'1'}, 1<<10)...)

	dest := Copy(src)
	tt.Equal(src, dest)
	tt.Equal(len(src), len(dest))

	src[0] = byte('2')
	tt.Equal(byte('1'), dest[0])

	Free(src)
	tt.Equal(bytes.Repeat([]byte{'1'}, 1<<10), dest)
	Free(dest)
}
