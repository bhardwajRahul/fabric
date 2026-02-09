# Package `mem`

The `mem` package uses multiple `sync.Pool`s of byte slices to provide allocation-free buffers of various sizes. Thirteen pools cover powers of two from 1KB to 4MB. Requests larger than 4MB fall back to a standard allocation.

`Alloc(byteSize int) []byte` returns a pooled buffer with at least the requested capacity and a length of 0. The caller must release it with `Free` when done.

`Free(block []byte)` returns the buffer to its pool for reuse.

`Copy(original []byte) []byte` allocates a new pooled buffer and copies the contents of the original into it.

Typical usage:

```go
block := mem.Alloc(4096)
defer mem.Free(block)
block = append(block, data...)
```
