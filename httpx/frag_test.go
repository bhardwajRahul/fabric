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

package httpx

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestHttpx_FragRequest(t *testing.T) {
	t.Parallel()

	// Using BodyReader
	request(t, 128*1024, 1024, true)
	request(t, 128*1024+16, 1024, true)
	request(t, 1024, 32*1024, true)

	// Using ByteReader
	request(t, 128*1024, 1024, false)
	request(t, 128*1024+16, 1024, false)
	request(t, 1024, 32*1024, false)
}

func request(t *testing.T, bodySize int64, fragmentSize int64, optimized bool) {
	assert := testarossa.For(t)

	body := []byte(rand.AlphaNum64(int(bodySize)))
	var bodyReader io.Reader
	if optimized {
		bodyReader = NewBodyReader(body)
	} else {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest("GET", "https://www.example.com", bodyReader)
	req.Header.Add("Foo", "Bar 1")
	req.Header.Add("Foo", "Bar 2")
	assert.NoError(err)

	// Fragment
	remaining := bodySize
	fragReqs := []*http.Request{}
	frag, err := NewFragRequest(req, fragmentSize)
	assert.NoError(err)
	for i := 1; i <= frag.N(); i++ {
		r, err := frag.Fragment(i)
		assert.NoError(err)
		assert.NotNil(r)
		fragReqs = append(fragReqs, r)

		contentLen := r.Header.Get("Content-Length")
		if remaining > fragmentSize {
			assert.Equal(strconv.FormatInt(fragmentSize, 10), contentLen)
		} else {
			assert.Equal(strconv.FormatInt(remaining, 10), contentLen)
		}
		remaining -= fragmentSize
	}

	// Defragment
	var countInt atomic.Int32
	defrag := NewDefragRequest()
	var wg sync.WaitGroup
	for _, r := range fragReqs {
		wg.Add(1)
		go func() {
			final, err := defrag.Add(r)
			assert.NoError(err)
			if final {
				countInt.Add(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(int32(1), countInt.Load())
	intReq, err := defrag.Integrated()
	assert.NoError(err)
	assert.NotNil(intReq)

	intBody, err := io.ReadAll(intReq.Body)
	assert.NoError(err)
	assert.Equal(body, intBody)

	contentLen := intReq.Header.Get("Content-Length")
	assert.True(contentLen == strconv.Itoa(len(body)))

	if assert.Len(intReq.Header["Foo"], 2) {
		assert.Equal("Bar 1", intReq.Header["Foo"][0])
		assert.Equal("Bar 2", intReq.Header["Foo"][1])
	}
}

func TestHttpx_FragResponse(t *testing.T) {
	t.Parallel()

	// Using BodyReader
	response(t, 128*1024, 1024, true)
	response(t, 128*1024+16, 1024, true)
	response(t, 1024, 32*1024, true)

	// Using ByteReader
	response(t, 128*1024, 1024, false)
	response(t, 128*1024+16, 1024, false)
	response(t, 1024, 32*1024, false)
}

func response(t *testing.T, bodySize int64, fragmentSize int64, optimized bool) {
	assert := testarossa.For(t)

	body := []byte(rand.AlphaNum64(int(bodySize)))

	var res *http.Response
	if optimized {
		rec := NewResponseRecorder()
		rec.Header().Add("Foo", "Bar 1")
		rec.Header().Add("Foo", "Bar 2")
		n, err := rec.Write(body)
		assert.NoError(err)
		assert.Equal(len(body), n)
		res = rec.Result()
	} else {
		rec := httptest.NewRecorder()
		rec.Header().Add("Foo", "Bar 1")
		rec.Header().Add("Foo", "Bar 2")
		n, err := rec.Write(body)
		assert.NoError(err)
		assert.Equal(len(body), n)
		res = rec.Result()
	}

	// Fragment
	remaining := bodySize
	fragRess := []*http.Response{}
	frag, err := NewFragResponse(res, fragmentSize)
	assert.NoError(err)
	for i := 1; i <= frag.N(); i++ {
		r, err := frag.Fragment(i)
		assert.NoError(err)
		assert.NotNil(r)
		fragRess = append(fragRess, r)

		contentLen := r.Header.Get("Content-Length")
		if remaining > fragmentSize {
			assert.Equal(strconv.FormatInt(fragmentSize, 10), contentLen)
		} else {
			assert.Equal(strconv.FormatInt(remaining, 10), contentLen)
		}
		remaining -= fragmentSize
	}

	// Defragment
	var countInt atomic.Int32
	defrag := NewDefragResponse()
	var wg sync.WaitGroup
	for _, r := range fragRess {
		wg.Add(1)
		go func() {
			final, err := defrag.Add(r)
			assert.NoError(err)
			if final {
				countInt.Add(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(int32(1), countInt.Load())
	intRes, err := defrag.Integrated()
	assert.NoError(err)
	assert.NotNil(intRes)

	intBody, err := io.ReadAll(intRes.Body)
	assert.NoError(err)
	assert.Equal(body, intBody)

	contentLen := intRes.Header.Get("Content-Length")
	assert.True(contentLen == strconv.Itoa(len(body)))

	if assert.Len(intRes.Header["Foo"], 2) {
		assert.Equal("Bar 1", intRes.Header["Foo"][0])
		assert.Equal("Bar 2", intRes.Header["Foo"][1])
	}
}

func TestHttpx_DefragRequestNoContentLen(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	bodySize := 128*1024 + 16
	body := []byte(rand.AlphaNum64(int(bodySize)))
	req, err := http.NewRequest("GET", "https://www.example.com", bytes.NewReader(body))
	assert.NoError(err)

	// Fragment the request
	frag, err := NewFragRequest(req, 1024)
	assert.NoError(err)
	for i := 1; i <= frag.N(); i++ {
		r, err := frag.Fragment(i)
		assert.NoError(err)
		assert.NotNil(r)
		assert.True(r.ContentLength > 0)
		assert.NotEqual("", r.Header.Get("Content-Length"))
	}

	// Defrag should still work without knowing the content length
	defrag := NewDefragRequest()
	for i := 1; i <= frag.N(); i++ {
		r, _ := frag.Fragment(i)
		r.Header.Del("Content-Length")
		r.ContentLength = -1
		_, err := defrag.Add(r)
		assert.NoError(err)
	}
	intReq, err := defrag.Integrated()
	assert.NoError(err)
	assert.NotNil(intReq)
	assert.Equal(-1, int(intReq.ContentLength))
	assert.Equal("", intReq.Header.Get("Content-Length"))
	intBody, err := io.ReadAll(intReq.Body)
	assert.NoError(err)
	assert.Equal(body, intBody)
}

func TestHttpx_DefragResponseNoContentLen(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	bodySize := 128*1024 + 16
	body := []byte(rand.AlphaNum64(int(bodySize)))

	rec := httptest.NewRecorder()
	n, err := rec.Write(body)
	assert.NoError(err)
	assert.Equal(len(body), n)
	res := rec.Result()

	// Fragment the request
	frag, err := NewFragResponse(res, 1024)
	assert.NoError(err)
	for i := 1; i <= frag.N(); i++ {
		r, err := frag.Fragment(i)
		assert.NoError(err)
		assert.NotNil(r)
		assert.True(r.ContentLength > 0)
		assert.NotEqual("", r.Header.Get("Content-Length"))
	}

	// Defrag should still work without knowing the content length
	defrag := NewDefragResponse()
	for i := 1; i <= frag.N(); i++ {
		r, _ := frag.Fragment(i)
		r.Header.Del("Content-Length")
		r.ContentLength = -1
		_, err := defrag.Add(r)
		assert.NoError(err)
	}
	intRes, err := defrag.Integrated()
	assert.NoError(err)
	assert.NotNil(intRes)
	assert.Equal(-1, int(intRes.ContentLength))
	assert.Equal("", intRes.Header.Get("Content-Length"))
	intBody, err := io.ReadAll(intRes.Body)
	assert.NoError(err)
	assert.Equal(body, intBody)
}

func TestHttpx_FragRequestZero(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r, err := http.NewRequest("POST", "/", strings.NewReader("hello"))
	assert.NoError(err)
	_, err = NewFragRequest(r, 0)
	assert.Contains(err, "non-positive")
}

func TestHttpx_FragResponseZero(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	rec := NewResponseRecorder()
	rec.Write([]byte("hello"))
	r := rec.Result()
	_, err := NewFragResponse(r, 0)
	assert.Contains(err, "non-positive")
}
