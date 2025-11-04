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
	"net/http"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestHttpx_DeepObject(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type Point struct {
		X int
		Y int
	}
	type Doc struct {
		I       int       `json:"i"`
		LI      int       `json:"li"`
		Zero    int       `json:"z,omitzero"`
		B       bool      `json:"b"`
		F       float32   `json:"f"`
		LF      float32   `json:"lf"`
		S       string    `json:"s"`
		Pt      Point     `json:"pt"`
		Empty   *Point    `json:"e,omitzero"`
		Null    *Point    `json:"n"`
		Special string    `json:"sp"`
		T       time.Time `json:"t"`
	}

	// Encode
	d1 := Doc{
		I:       5,
		LI:      10000000,
		B:       true,
		F:       5.67,
		LF:      8.9e23,
		S:       "Hello",
		Special: "Q&A",
		Pt:      Point{X: 3, Y: 4},
		T:       time.Date(2001, 10, 1, 12, 0, 0, 0, time.UTC),
	}
	values, err := EncodeDeepObject(d1)
	if assert.NoError(err) {
		assert.Equal("5", values.Get("i"))
		assert.Equal("10000000", values.Get("li"))
		assert.Equal("true", values.Get("b"))
		assert.Equal("5.67", values.Get("f"))
		assert.Equal("8.9e+23", values.Get("lf"))
		assert.Equal("Hello", values.Get("s"))
		assert.Equal("Q&A", values.Get("sp"))
		assert.Equal("3", values.Get("pt[X]"))
		assert.Equal("4", values.Get("pt[Y]"))
		assert.Equal("2001-10-01T12:00:00Z", values.Get("t"))
	}

	var d2 Doc
	err = DecodeDeepObject(values, &d2)
	if assert.NoError(err) {
		assert.Equal(d1, d2)
	}
}

func TestHttpx_DeepObjectRequestPath(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var data struct {
		X struct {
			A int
			B int
			Y struct {
				C int
				D int
			}
		}
		S string
		B bool
		E string
	}
	r, err := http.NewRequest("GET", `/path?x.a=5&x[b]=3&x.y.c=1&x[y][d]=2&s=str&b=true&e=`, nil)
	assert.NoError(err)
	err = DecodeDeepObject(r.URL.Query(), &data)
	assert.NoError(err)
	assert.Equal(5, data.X.A)
	assert.Equal(3, data.X.B)
	assert.Equal(1, data.X.Y.C)
	assert.Equal(2, data.X.Y.D)
	assert.Equal("str", data.S)
	assert.Equal(true, data.B)
	assert.Equal("", data.E)
}

func TestHttpx_DeepObjectDecodeOne(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	data := struct {
		S string  `json:"s"`
		I int     `json:"i"`
		F float64 `json:"f"`
		B bool    `json:"b"`
	}{}

	// Into string
	err := decodeOne("s", "hello", &data)
	if assert.NoError(err) {
		assert.Equal("hello", data.S)
	}
	err = decodeOne("s", `"hello"`, &data)
	if assert.NoError(err) {
		assert.Equal("hello", data.S)
	}
	err = decodeOne("s", "5", &data)
	if assert.NoError(err) {
		assert.Equal("5", data.S)
	}

	// Into int
	err = decodeOne("i", "5", &data)
	if assert.NoError(err) {
		assert.Equal(5, data.I)
	}
	err = decodeOne("i", "1000000", &data)
	if assert.NoError(err) {
		assert.Equal(1000000, data.I)
	}
	// err = decodeOne("i", "1e+06", &data)
	// if assert.NoError(err) {
	// 	assert.Equal(1000000, data.I)
	// }

	// Into float64
	err = decodeOne("f", "5", &data)
	if assert.NoError(err) {
		assert.Equal(5.0, data.F)
	}
	err = decodeOne("f", "5.6", &data)
	if assert.NoError(err) {
		assert.Equal(5.6, data.F)
	}
	err = decodeOne("f", "1e-3", &data)
	if assert.NoError(err) {
		assert.Equal(.001, data.F)
	}

	// Into bool
	err = decodeOne("b", "true", &data)
	if assert.NoError(err) {
		assert.Equal(true, data.B)
	}
	err = decodeOne("b", `"true"`, &data)
	if assert.NoError(err) {
		assert.Equal(true, data.B)
	}
}
