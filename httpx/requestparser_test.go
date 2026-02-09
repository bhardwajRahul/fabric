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

package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestHttpx_RequestParserOverrideJSON(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var data struct {
		X int
		Y int
	}
	var buf bytes.Buffer
	buf.WriteString(`{"x":1,"y":1}`)

	r, err := http.NewRequest("POST", "/path", &buf)
	r.Header.Set("Content-Type", "application/json")
	assert.NoError(err)
	err = ReadInputPayload(r, "/path", &data)
	assert.NoError(err)
	assert.Equal(1, data.X)
	assert.Equal(1, data.Y)

	r, err = http.NewRequest("POST", "/path?x=2", &buf)
	assert.NoError(err)
	err = ReadInputPayload(r, "/path", &data)
	assert.NoError(err)
	assert.Equal(2, data.X)
	assert.Equal(1, data.Y)
}

func TestHttpx_WriteInputPayload(t *testing.T) {
	t.Parallel()

	t.Run("post_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			X int `json:"x,omitzero"`
			Y int `json:"y,omitzero"`
		}
		query, body, err := WriteInputPayload("POST", In{X: 3, Y: 4})
		assert.NoError(err)
		assert.Nil(query)
		in, ok := body.(In)
		assert.Equal(true, ok)
		assert.Equal(3, in.X)
		assert.Equal(4, in.Y)
	})

	t.Run("get_query", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			X int `json:"x,omitzero"`
			Y int `json:"y,omitzero"`
		}
		query, body, err := WriteInputPayload("GET", In{X: 3, Y: 4})
		assert.NoError(err)
		assert.Nil(body)
		assert.Equal("3", query.Get("x"))
		assert.Equal("4", query.Get("y"))
	})

	t.Run("http_request_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			HTTPRequestBody []int `json:"-"`
		}
		query, body, err := WriteInputPayload("POST", In{HTTPRequestBody: []int{1, 2, 3}})
		assert.NoError(err)
		assert.Len(query, 0)
		slice, ok := body.([]int)
		assert.Equal(true, ok)
		assert.Equal([]int{1, 2, 3}, slice)
	})

	t.Run("http_request_body_with_other_args", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			HTTPRequestBody []int  `json:"-"`
			Name            string `json:"name,omitzero"`
		}
		query, body, err := WriteInputPayload("POST", In{HTTPRequestBody: []int{1, 2}, Name: "alice"})
		assert.NoError(err)
		slice, ok := body.([]int)
		assert.Equal(true, ok)
		assert.Equal([]int{1, 2}, slice)
		assert.Equal("alice", query.Get("name"))
		assert.Equal("", query.Get("HTTPRequestBody"))
	})
}

func TestHttpx_ReadInputPayload(t *testing.T) {
	t.Parallel()

	t.Run("json_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			X int `json:"x,omitzero"`
			Y int `json:"y,omitzero"`
		}
		var in In
		r, err := http.NewRequest("POST", `/path`, strings.NewReader(`{"x":10,"y":20}`))
		assert.NoError(err)
		r.Header.Set("Content-Type", "application/json")
		err = ReadInputPayload(r, "/path", &in)
		assert.NoError(err)
		assert.Equal(10, in.X)
		assert.Equal(20, in.Y)
	})

	t.Run("query_args", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			X int `json:"x,omitzero"`
			Y int `json:"y,omitzero"`
		}
		var in In
		r, err := http.NewRequest("GET", `/path?x=5&y=6`, nil)
		assert.NoError(err)
		err = ReadInputPayload(r, "/path", &in)
		assert.NoError(err)
		assert.Equal(5, in.X)
		assert.Equal(6, in.Y)
	})

	t.Run("http_request_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type In struct {
			HTTPRequestBody []int `json:"-"`
		}
		var in In
		r, err := http.NewRequest("POST", `/path`, strings.NewReader(`[1,2,3]`))
		assert.NoError(err)
		r.Header.Set("Content-Type", "application/json")
		err = ReadInputPayload(r, "/path", &in)
		assert.NoError(err)
		assert.Equal([]int{1, 2, 3}, in.HTTPRequestBody)
	})
}

func TestHttpx_WriteOutputPayload(t *testing.T) {
	t.Parallel()

	t.Run("standard", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			Sum int `json:"sum,omitzero"`
		}
		w := httptest.NewRecorder()
		err := WriteOutputPayload(w, Out{Sum: 42})
		assert.NoError(err)
		assert.Equal("application/json", w.Header().Get("Content-Type"))
		assert.Equal(http.StatusOK, w.Code)

		var out Out
		err = json.Unmarshal(w.Body.Bytes(), &out)
		assert.NoError(err)
		assert.Equal(42, out.Sum)
	})

	t.Run("http_status_code", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			Sum            int `json:"sum,omitzero"`
			HTTPStatusCode int `json:"-"`
		}
		w := httptest.NewRecorder()
		err := WriteOutputPayload(w, Out{Sum: 0, HTTPStatusCode: http.StatusBadRequest})
		assert.NoError(err)
		assert.Equal(http.StatusBadRequest, w.Code)

		// HTTPStatusCode should not appear in the JSON body
		var raw map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &raw)
		assert.NoError(err)
		_, hasStatusCode := raw["HTTPStatusCode"]
		assert.Equal(false, hasStatusCode)
	})

	t.Run("http_response_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			HTTPResponseBody []int `json:"-"`
		}
		w := httptest.NewRecorder()
		err := WriteOutputPayload(w, Out{HTTPResponseBody: []int{10, 20, 30}})
		assert.NoError(err)

		var result []int
		err = json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(err)
		assert.Equal([]int{10, 20, 30}, result)
	})

	t.Run("http_status_code_and_response_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			HTTPResponseBody []int `json:"-"`
			HTTPStatusCode   int   `json:"-"`
		}
		w := httptest.NewRecorder()
		err := WriteOutputPayload(w, Out{HTTPResponseBody: []int{1}, HTTPStatusCode: http.StatusCreated})
		assert.NoError(err)
		assert.Equal(http.StatusCreated, w.Code)

		var result []int
		err = json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(err)
		assert.Equal([]int{1}, result)
	})
}

func TestHttpx_ReadOutputPayload(t *testing.T) {
	t.Parallel()

	t.Run("standard", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			Sum int `json:"sum,omitzero"`
		}
		var out Out
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"sum":42}`)),
		}
		err := ReadOutputPayload(res, &out)
		assert.NoError(err)
		assert.Equal(42, out.Sum)
	})

	t.Run("http_status_code", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			Sum            int `json:"sum,omitzero"`
			HTTPStatusCode int `json:"-"`
		}
		var out Out
		res := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"sum":0}`)),
		}
		err := ReadOutputPayload(res, &out)
		assert.NoError(err)
		assert.Equal(0, out.Sum)
		assert.Equal(http.StatusBadRequest, out.HTTPStatusCode)
	})

	t.Run("http_response_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			HTTPResponseBody []int `json:"-"`
		}
		var out Out
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[10,20,30]`)),
		}
		err := ReadOutputPayload(res, &out)
		assert.NoError(err)
		assert.Equal([]int{10, 20, 30}, out.HTTPResponseBody)
	})

	t.Run("http_status_code_and_response_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			HTTPResponseBody []int `json:"-"`
			HTTPStatusCode   int   `json:"-"`
		}
		var out Out
		res := &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(`[1,2]`)),
		}
		err := ReadOutputPayload(res, &out)
		assert.NoError(err)
		assert.Equal(http.StatusCreated, out.HTTPStatusCode)
		assert.Equal([]int{1, 2}, out.HTTPResponseBody)
	})

	t.Run("nil_body", func(t *testing.T) {
		assert := testarossa.For(t)

		type Out struct {
			Sum int `json:"sum,omitzero"`
		}
		var out Out
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       nil,
		}
		err := ReadOutputPayload(res, &out)
		assert.NoError(err)
		assert.Equal(0, out.Sum)
	})
}

func TestHttpx_RequestParserOverrideFormData(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var data struct {
		X int
		Y int
	}
	var buf bytes.Buffer
	buf.WriteString(`x=1&y=1`)

	r, err := http.NewRequest("POST", "/path", &buf)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	assert.NoError(err)
	err = ReadInputPayload(r, "/path", &data)
	assert.NoError(err)
	assert.Equal(1, data.X)
	assert.Equal(1, data.Y)

	r, err = http.NewRequest("POST", "/path?x=2", &buf)
	assert.NoError(err)
	err = ReadInputPayload(r, "/path", &data)
	assert.NoError(err)
	assert.Equal(2, data.X)
	assert.Equal(1, data.Y)
}
