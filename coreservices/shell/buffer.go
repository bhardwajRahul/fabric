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
	"fmt"
)

// cappedBuffer is a bounded io.Writer that retains the first headCap bytes and the last
// tailCap bytes of all data written to it. Bytes in between are discarded but counted, so
// the rendered output can indicate how much was elided. Writes never block or fail.
type cappedBuffer struct {
	head    []byte
	tail    []byte
	headCap int
	tailCap int
	tailPos int
	tailLen int
	written int
}

func newCappedBuffer(capacity int) *cappedBuffer {
	if capacity < 2 {
		capacity = 2
	}
	half := capacity / 2
	return &cappedBuffer{
		headCap: half,
		tailCap: capacity - half,
	}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	b.written += n
	if len(b.head) < b.headCap {
		space := b.headCap - len(b.head)
		if space >= len(p) {
			b.head = append(b.head, p...)
			return n, nil
		}
		b.head = append(b.head, p[:space]...)
		p = p[space:]
	}
	for len(p) > 0 {
		if b.tailLen < b.tailCap {
			avail := b.tailCap - b.tailLen
			chunk := p
			if avail < len(chunk) {
				chunk = p[:avail]
			}
			b.tail = append(b.tail, chunk...)
			b.tailLen += len(chunk)
			b.tailPos = (b.tailPos + len(chunk)) % b.tailCap
			p = p[len(chunk):]
			continue
		}
		avail := b.tailCap - b.tailPos
		chunk := p
		if avail < len(chunk) {
			chunk = p[:avail]
		}
		copy(b.tail[b.tailPos:], chunk)
		b.tailPos = (b.tailPos + len(chunk)) % b.tailCap
		p = p[len(chunk):]
	}
	return n, nil
}

func (b *cappedBuffer) String() string {
	truncated := b.written - len(b.head) - b.tailLen
	var tail []byte
	if b.tailLen < b.tailCap {
		tail = b.tail[:b.tailLen]
	} else {
		tail = make([]byte, 0, b.tailCap)
		tail = append(tail, b.tail[b.tailPos:]...)
		tail = append(tail, b.tail[:b.tailPos]...)
	}
	if truncated <= 0 {
		return string(b.head) + string(tail)
	}
	return fmt.Sprintf("%s\n... [truncated %d bytes] ...\n%s", b.head, truncated, tail)
}
