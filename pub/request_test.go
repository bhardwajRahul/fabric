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

package pub

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/testarossa"
)

func TestPub_MethodAndURL(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// GET
	req, err := NewRequest([]Option{
		GET("https://www.example.com"),
	}...)
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	assert.Equal("GET", httpReq.Method)
	assert.Equal("www.example.com", httpReq.URL.Hostname())

	// POST
	req, err = NewRequest([]Option{
		POST("https://www.example.com/path"),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("POST", httpReq.Method)
	assert.Equal("www.example.com", httpReq.URL.Hostname())
	assert.Equal("/path", httpReq.URL.Path)

	// Any method
	req, err = NewRequest([]Option{
		Method("Delete"), // Mixed case
		URL("https://www.example.com"),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("DELETE", httpReq.Method)
	assert.Equal("www.example.com", httpReq.URL.Hostname())
}

func TestPub_Header(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	req, err := NewRequest([]Option{
		GET("https://www.example.com"),
		Header("Content-Type", "text/html"),
		Header("X-SOMETHING", "Else"), // Uppercase
	}...)
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	assert.Equal("text/html", httpReq.Header.Get("Content-Type"))
	assert.Equal("Else", httpReq.Header.Get("X-Something"))
}

func TestPub_Body(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// String
	req, err := NewRequest([]Option{
		GET("https://www.example.com"),
		Body("Hello World"),
	}...)
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	body, err := io.ReadAll(httpReq.Body)
	assert.NoError(err)
	assert.Equal("Hello World", string(body))

	// []byte
	req, err = NewRequest([]Option{
		GET("https://www.example.com"),
		Body([]byte("Hello World")),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	body, err = io.ReadAll(httpReq.Body)
	assert.NoError(err)
	assert.Equal("Hello World", string(body))

	// io.Reader
	req, err = NewRequest([]Option{
		GET("https://www.example.com"),
		Body(bytes.NewReader([]byte("Hello World"))),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	body, err = io.ReadAll(httpReq.Body)
	assert.NoError(err)
	assert.Equal("Hello World", string(body))

	// JSON
	j := struct {
		S string `json:"s"`
		I int    `json:"i"`
	}{"ABC", 123}
	req, err = NewRequest([]Option{
		GET("https://www.example.com"),
		Body(j),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	body, err = io.ReadAll(httpReq.Body)
	assert.NoError(err)
	assert.Equal(`{"s":"ABC","i":123}`, string(body))

	// nil
	req, err = NewRequest([]Option{
		POST("https://www.example.com"),
		Body(nil),
	}...)
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Nil(httpReq.Body)
}

func toHTTP(req *Request) (*http.Request, error) {
	httpReq, err := http.NewRequest(req.Method, req.URL, req.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for name, value := range req.Header {
		httpReq.Header[name] = value
	}
	return httpReq, nil
}

func TestPub_Canonical(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r, err := NewRequest(GET("https://www.example.com:567/path?a=5&b=6")) // https
	assert.NoError(err)
	assert.Equal("https://www.example.com:567/path", r.Canonical())

	r, err = NewRequest(POST("http://www.example.com/path")) // http
	assert.NoError(err)
	assert.Equal("http://www.example.com:80/path", r.Canonical())

	r, err = NewRequest(PATCH("//www.example.com/path")) // no scheme
	assert.NoError(err)
	assert.Equal("https://www.example.com:443/path", r.Canonical())
}

func TestPub_Apply(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	r, err := NewRequest()
	assert.NoError(err)

	r.Apply(URL("https://www.example.com/delete"), Method("DELETE"))
	assert.Equal("DELETE", r.Method)
	assert.Equal("https://www.example.com:443/delete", r.Canonical())

	r.Apply(GET("https://www.example.com/get"))
	assert.Equal("GET", r.Method)
	assert.Equal("https://www.example.com:443/get", r.Canonical())

	r.Apply(POST("https://www.example.com/post"))
	assert.Equal("POST", r.Method)
	assert.Equal("https://www.example.com:443/post", r.Canonical())

	r.Apply(Multicast())
	assert.Equal(true, r.Multicast)

	r.Apply(Unicast())
	assert.Equal(false, r.Multicast)

	r.Apply(Body("lorem ipsum"))
	body, err := io.ReadAll(r.Body)
	assert.NoError(err)
	assert.Equal("lorem ipsum", string(body))

	r.Apply(Header("Foo", "Bar"))
	assert.Equal("Bar", r.Header.Get("Foo"))

	r.Apply(Actor(struct {
		Sub   string   `json:"sub"`
		Roles []string `json:"roles"`
	}{
		Sub:   "foo@example.com",
		Roles: []string{"a", "b", "c"},
	}))
	assert.Equal(`{"sub":"foo@example.com","roles":["a","b","c"]}`, r.Header.Get(frame.HeaderActor))
}

func TestPub_QueryArgs(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	req, err := NewRequest(GET("https://www.example.com:443/path?a=1"))
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://www.example.com:443/path?a=1", httpReq.URL.String())

	err = req.Apply(QueryArg("b", "2"))
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://www.example.com:443/path?a=1&b=2", httpReq.URL.String())

	err = req.Apply(QueryArg("a", "3"))
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://www.example.com:443/path?a=1&b=2&a=3", httpReq.URL.String())

	err = req.Apply(URL("https://zzz.example.com:123/newpath"))
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://zzz.example.com:123/newpath?b=2&a=3", httpReq.URL.String())

	err = req.Apply(QueryString("m=5&n=6"))
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://zzz.example.com:123/newpath?b=2&a=3&m=5&n=6", httpReq.URL.String())

	err = req.Apply(Query(url.Values{
		"x": []string{"33"},
		"y": []string{"66"},
	}))
	assert.NoError(err)
	httpReq, err = toHTTP(req)
	assert.NoError(err)
	assert.Equal("https://zzz.example.com:123/newpath?b=2&a=3&m=5&n=6&x=33&y=66", httpReq.URL.String())
}

func TestPub_RelativeURL(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	req, err := NewRequest([]Option{
		GET("https://www.example.com/path/to/file"),
		RelativeURL("../another-file"),
	}...)
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	assert.Equal("GET", httpReq.Method)
	assert.Equal("/path/another-file", httpReq.URL.Path)
}

func TestPub_FillPathArguments(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	req, err := NewRequest([]Option{
		GET("https://www.example.com/user/{id}/details?id=5"),
	}...)
	assert.NoError(err)
	httpReq, err := toHTTP(req)
	assert.NoError(err)
	assert.Equal("GET", httpReq.Method)
	assert.Equal("/user/5/details", httpReq.URL.Path)
}
