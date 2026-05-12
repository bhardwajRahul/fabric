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

package middleware

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestCors_AllowedOrigin(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	mw := Cors(func(r *http.Request, origin string) string {
		if origin == "https://allowed.example" {
			return origin
		}
		return ""
	})

	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "http://ingress.example/x", nil)
	r.Header.Set("Origin", "https://allowed.example")
	err := mw(func(w http.ResponseWriter, r *http.Request) error { return nil })(w, r)
	assert.NoError(err)
	assert.Equal("https://allowed.example", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal("true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCors_RejectedOrigin(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	mw := Cors(func(r *http.Request, origin string) string { return "" })

	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "http://ingress.example/x", nil)
	r.Header.Set("Origin", "https://attacker.example")
	err := mw(func(w http.ResponseWriter, r *http.Request) error {
		t.Fatal("downstream handler must not be called for a rejected origin")
		return nil
	})(w, r)
	assert.Error(err)
}

func TestCors_NoOriginPassesThrough(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	mw := Cors(func(r *http.Request, origin string) string {
		t.Fatal("allowedOrigin must not be consulted when Origin is absent")
		return ""
	})

	called := false
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "http://ingress.example/x", nil)
	err := mw(func(w http.ResponseWriter, r *http.Request) error {
		called = true
		return nil
	})(w, r)
	assert.NoError(err)
	assert.True(called)
	assert.Equal("", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCors_SameOriginPinningFromRequest(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Simulates the default config: pin ACAO to scheme://host derived from
	// the request itself. X-Forwarded-* must be ignored so an edge attacker
	// cannot inflate ACAO to a host they control.
	allow := func(r *http.Request, origin string) string {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + r.Host
	}
	mw := Cors(allow)

	// Plain HTTP request: scheme is derived from r.TLS == nil.
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "http://ingress.example:4040/x", nil)
	r.Host = "ingress.example:4040"
	r.Header.Set("Origin", "https://attacker.example")
	r.Header.Set("X-Forwarded-Host", "attacker.tld")
	r.Header.Set("X-Forwarded-Proto", "https")
	err := mw(func(w http.ResponseWriter, r *http.Request) error { return nil })(w, r)
	assert.NoError(err)
	assert.Equal("http://ingress.example:4040", w.Header().Get("Access-Control-Allow-Origin"))

	// TLS request: scheme flips to https.
	w = httpx.NewResponseRecorder()
	r, _ = http.NewRequest("GET", "https://ingress.example/x", nil)
	r.Host = "ingress.example"
	r.TLS = &tls.ConnectionState{}
	r.Header.Set("Origin", "https://attacker.example")
	err = mw(func(w http.ResponseWriter, r *http.Request) error { return nil })(w, r)
	assert.NoError(err)
	assert.Equal("https://ingress.example", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCors_PreflightShortCircuits(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	mw := Cors(func(r *http.Request, origin string) string { return origin })

	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("OPTIONS", "http://ingress.example/x", nil)
	r.Header.Set("Origin", "https://allowed.example")
	err := mw(func(w http.ResponseWriter, r *http.Request) error {
		t.Fatal("preflight must not call the downstream handler")
		return nil
	})(w, r)
	assert.NoError(err)
	assert.Equal(http.StatusNoContent, w.Result().StatusCode)
}
