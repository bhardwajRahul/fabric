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
	"net/http"
	"net/url"
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

func TestHttpx_DeepObjectArrays(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Simple string array
	var simple struct {
		Tags []string `json:"tags"`
	}
	vals := url.Values{
		"tags[0]": {"alpha"},
		"tags[1]": {"beta"},
		"tags[2]": {"gamma"},
	}
	err := DecodeDeepObject(vals, &simple)
	if assert.NoError(err) {
		assert.Equal(3, len(simple.Tags))
		assert.Equal("alpha", simple.Tags[0])
		assert.Equal("beta", simple.Tags[1])
		assert.Equal("gamma", simple.Tags[2])
	}

	// Array of objects
	type Item struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	var items struct {
		Items []Item `json:"items"`
	}
	vals = url.Values{
		"items[0][name]":  {"foo"},
		"items[0][value]": {"10"},
		"items[1][name]":  {"bar"},
		"items[1][value]": {"20"},
	}
	err = DecodeDeepObject(vals, &items)
	if assert.NoError(err) {
		assert.Equal(2, len(items.Items))
		assert.Equal("foo", items.Items[0].Name)
		assert.Equal(10, items.Items[0].Value)
		assert.Equal("bar", items.Items[1].Name)
		assert.Equal(20, items.Items[1].Value)
	}

	// Non-sequential keys are not treated as arrays
	var nonSeq struct {
		M map[string]string `json:"m"`
	}
	vals = url.Values{
		"m[0]": {"a"},
		"m[2]": {"b"},
	}
	err = DecodeDeepObject(vals, &nonSeq)
	if assert.NoError(err) {
		assert.Equal("a", nonSeq.M["0"])
		assert.Equal("b", nonSeq.M["2"])
	}

	// Dot notation arrays
	var dotArr struct {
		X []int `json:"x"`
	}
	vals = url.Values{
		"x.0": {"100"},
		"x.1": {"200"},
		"x.2": {"300"},
	}
	err = DecodeDeepObject(vals, &dotArr)
	if assert.NoError(err) {
		assert.Equal(3, len(dotArr.X))
		assert.Equal(100, dotArr.X[0])
		assert.Equal(200, dotArr.X[1])
		assert.Equal(300, dotArr.X[2])
	}
}

func TestHttpx_DeepObjectTypeDetection(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var data struct {
		S    string  `json:"s"`
		I    int     `json:"i"`
		F    float64 `json:"f"`
		B    bool    `json:"b"`
		Null *string `json:"null"`
		E    string  `json:"e"`
		LI   int     `json:"li"`
		LF   float32 `json:"lf"`
	}

	vals := url.Values{
		"s":    {"hello"},
		"i":    {"42"},
		"f":    {"3.14"},
		"b":    {"true"},
		"null": {"null"},
		"e":    {""},
		"li":   {"10000000"},
		"lf":   {"8.9e+23"},
	}
	err := DecodeDeepObject(vals, &data)
	if assert.NoError(err) {
		assert.Equal("hello", data.S)
		assert.Equal(42, data.I)
		assert.Equal(3.14, data.F)
		assert.Equal(true, data.B)
		assert.Nil(data.Null)
		assert.Equal("", data.E)
		assert.Equal(10000000, data.LI)
		assert.Equal(float32(8.9e+23), data.LF)
	}

	// Number into string field
	var strNum struct {
		S string `json:"s"`
	}
	err = DecodeDeepObject(url.Values{"s": {"5"}}, &strNum)
	if assert.NoError(err) {
		assert.Equal("5", strNum.S)
	}

	// Boolean false
	var boolFalse struct {
		B bool `json:"b"`
	}
	err = DecodeDeepObject(url.Values{"b": {"false"}}, &boolFalse)
	if assert.NoError(err) {
		assert.Equal(false, boolFalse.B)
	}
}

func TestHttpx_DeepObjectNesting(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Mixed bracket and dot notation
	var data struct {
		X struct {
			A int `json:"a"`
			B int `json:"b"`
			Y struct {
				C int `json:"c"`
				D int `json:"d"`
			} `json:"y"`
		} `json:"x"`
		S string `json:"s"`
	}
	r, err := http.NewRequest("GET", `/path?x.a=5&x[b]=3&x.y.c=1&x[y][d]=2&s=str`, nil)
	assert.NoError(err)
	err = DecodeDeepObject(r.URL.Query(), &data)
	if assert.NoError(err) {
		assert.Equal(5, data.X.A)
		assert.Equal(3, data.X.B)
		assert.Equal(1, data.X.Y.C)
		assert.Equal(2, data.X.Y.D)
		assert.Equal("str", data.S)
	}
}
