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

package frame

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestFrame_Of(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// http.Request
	httpRequest, err := http.NewRequest("GET", "https://www.example.com", nil)
	tt.NoError(err)
	httpRequest.Header.Set(HeaderMsgId, "123")
	tt.Equal("123", Of(httpRequest).MessageID())

	// httptest.ResponseRecorder and http.Response
	httpRecorder := httptest.NewRecorder()
	httpRecorder.Header().Set(HeaderMsgId, "123")
	tt.Equal("123", Of(httpRecorder).MessageID())
	httpResponse := httpRecorder.Result()
	tt.Equal("123", Of(httpResponse).MessageID())

	// http.Header
	hdr := make(http.Header)
	hdr.Set(HeaderMsgId, "123")
	tt.Equal("123", Of(hdr).MessageID())

	// context.Context
	ctx := context.WithValue(context.Background(), contextKey, hdr)
	tt.Equal("123", Of(ctx).MessageID())

	// Empty context.Context should not panic
	tt.Equal("", Of(context.Background()).MessageID())
}

func TestFrame_GetSet(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	f := Of(make(http.Header))

	tt.Equal("", f.OpCode())
	f.SetOpCode(OpCodeError)
	tt.Equal(OpCodeError, f.OpCode())
	f.SetOpCode("")
	tt.Equal("", f.OpCode())

	tt.Zero(f.CallDepth())
	f.SetCallDepth(123)
	tt.Equal(123, f.CallDepth())
	f.SetCallDepth(0)
	tt.Zero(f.CallDepth())

	tt.Equal("", f.FromHost())
	f.SetFromHost("www.example.com")
	tt.Equal("www.example.com", f.FromHost())
	f.SetFromHost("")
	tt.Equal("", f.FromHost())

	tt.Equal("", f.FromID())
	f.SetFromID("1234567890")
	tt.Equal("1234567890", f.FromID())
	f.SetFromID("")
	tt.Equal("", f.FromID())

	tt.Zero(f.FromVersion())
	f.SetFromVersion(12345)
	tt.Equal(12345, f.FromVersion())
	f.SetFromVersion(0)
	tt.Zero(f.FromVersion())

	tt.Equal("", f.MessageID())
	f.SetMessageID("1234567890")
	tt.Equal("1234567890", f.MessageID())
	f.SetMessageID("")
	tt.Equal("", f.MessageID())

	budget := f.TimeBudget()
	tt.Equal(time.Duration(0), budget)
	f.SetTimeBudget(123 * time.Second)
	budget = f.TimeBudget()
	tt.Equal(123*time.Second, budget)
	f.SetTimeBudget(0)
	budget = f.TimeBudget()
	tt.Equal(time.Duration(0), budget)

	tt.Equal("", f.Queue())
	f.SetQueue("1234567890")
	tt.Equal("1234567890", f.Queue())
	f.SetQueue("")
	tt.Equal("", f.Queue())

	fi, fm := f.Fragment()
	tt.Equal(1, fi)
	tt.Equal(1, fm)
	f.SetFragment(2, 5)
	fi, fm = f.Fragment()
	tt.Equal(fi, 2)
	tt.Equal(fm, 5)
	f.SetFragment(0, 0)
	fi, fm = f.Fragment()
	tt.Equal(fi, 1)
	tt.Equal(fm, 1)
}

func TestFrame_XForwarded(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	httpRequest, err := http.NewRequest("GET", "https://www.example.com", nil)
	tt.NoError(err)
	frame := Of(httpRequest)
	tt.Equal("", frame.XForwardedBaseURL())

	httpRequest.Header.Set("X-Forwarded-Proto", "https")
	httpRequest.Header.Set("X-Forwarded-Host", "www.proxy.com")
	httpRequest.Header.Set("X-Forwarded-Prefix", "/example")
	tt.Equal("https://www.proxy.com/example", frame.XForwardedBaseURL())

	httpRequest.Header.Set("X-Forwarded-Prefix", "/example/")
	tt.Equal("https://www.proxy.com/example", frame.XForwardedBaseURL())
}

func TestFrame_Languages(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	testCases := []string{
		"", "",
		"en", "en",
		"EN", "EN",
		"da, en-gb;q=0.8, en;q=0.7", "da,en-gb,en",
		"da, en-gb;q=0.7, en;q=0.8", "da,en,en-gb",
		"da,en-gb;q=0.7,en;q=0.8", "da,en,en-gb",
		" en ;q=1   , es ; q = 0.5 ", "en,es",
	}
	h := http.Header{}
	for i := 0; i < len(testCases); i += 2 {
		h.Set("Accept-Language", testCases[i])
		langs := Of(h).Languages()
		var expected []string
		if testCases[i+1] != "" {
			expected = strings.Split(testCases[i+1], ",")
		}
		tt.Equal(expected, langs)
	}
}

func TestFrame_ParseActor(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	f := Of(make(http.Header))

	// Map
	m1 := map[string]any{"iss": "my_issuer", "number": 6.0, "string": "example", "array": []any{"x", "y", "z"}}
	var m2 map[string]any
	f.SetActor(m1)
	ok, err := f.ParseActor(&m2)
	if tt.NoError(err) && tt.True(ok) {
		tt.Equal(m1, m2)
	}

	// Object
	type Obj struct {
		Iss    string   `json:"iss"`
		Number float64  `json:"number"`
		String string   `json:"string"`
		Array  []string `json:"array"`
	}
	o1 := Obj{Iss: "my_issuer", Number: 6.0, String: "example", Array: []string{"x", "y", "z"}}
	var o2 Obj
	f.SetActor(o1)
	ok, err = f.ParseActor(&o2)
	if tt.NoError(err) && tt.True(ok) {
		tt.Equal(o1, o2)
	}

	// Overwrite with another object
	o1 = Obj{Iss: "another_issuer", Number: 8.0, String: "foo", Array: []string{"a", "b", "c"}}
	f.SetActor(o1)
	ok, err = f.ParseActor(&o2)
	if tt.NoError(err) && tt.True(ok) {
		tt.Equal(o1, o2)
	}
}

func TestFrame_IfActor(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	f := Of(make(http.Header))

	// False before setting actor
	ok, err := f.IfActor("anything")
	if tt.NoError(err) {
		tt.False(ok)
	}
	ok, err = f.IfActor("first_issuer")
	if tt.NoError(err) {
		tt.False(ok)
	}
	ok, err = f.IfActor("iss=='first_issuer' && super_user")
	if tt.NoError(err) {
		tt.False(ok)
	}
	ok, err = f.IfActor("iss=='first_issuer' && sub")
	if tt.NoError(err) {
		tt.False(ok)
	}

	// Set actor
	f.SetActor(map[string]any{
		"iss":        "first_issuer",
		"sub":        "subject@first.com",
		"super_user": true,
		"roles":      "admin, manager, user",
		"groups":     []any{"sales", "engineering"},
	})

	// Should trim newline
	tt.NotContains(f.Header().Get(HeaderActor), "\n")

	// True
	ok, err = f.IfActor("iss=='first_issuer'")
	if tt.NoError(err) {
		tt.True(ok)
	}
	ok, err = f.IfActor("iss=='first_issuer' && super_user")
	if tt.NoError(err) {
		tt.True(ok)
	}
	ok, err = f.IfActor("iss=='first_issuer' && sub")
	if tt.NoError(err) {
		tt.True(ok)
	}
	ok, err = f.IfActor("roles=~'manager' && sub!~'example.com'")
	if tt.NoError(err) {
		tt.True(ok)
	}
	ok, err = f.IfActor("groups.sales && groups.engineering && !groups.hr")
	if tt.NoError(err) {
		tt.True(ok)
	}

	// False
	ok, err = f.IfActor("iss=='second_issuer'")
	if tt.NoError(err) {
		tt.False(ok)
	}
	ok, err = f.IfActor("groups.hr || roles=~'director'")
	if tt.NoError(err) {
		tt.False(ok)
	}
}
