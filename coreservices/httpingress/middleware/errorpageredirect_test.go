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

package middleware

import (
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestErrorPageRedirect_Redirect(t *testing.T) {
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/login", nil)
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	r.Header.Set("Sec-Fetch-Dest", "document")
	mw := ErrorPageRedirect(http.StatusUnauthorized, "/login")
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		w.WriteHeader(http.StatusUnauthorized)
		return nil
	})(w, r)
	testarossa.NoError(t, err)
	testarossa.Equal(t, w.StatusCode(), http.StatusTemporaryRedirect)
	testarossa.Equal(t, w.Header().Get("Location"), "/login")
}

func TestErrorPageRedirect_NoBrowserHeaders(t *testing.T) {
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/login", nil)
	mw := ErrorPageRedirect(http.StatusUnauthorized, "/login")
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		w.WriteHeader(http.StatusUnauthorized)
		return nil
	})(w, r)
	testarossa.NoError(t, err)
	testarossa.Equal(t, w.StatusCode(), http.StatusUnauthorized)
}

func TestErrorPageRedirect_WrongErrorCode(t *testing.T) {
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/login", nil)
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	r.Header.Set("Sec-Fetch-Dest", "document")
	mw := ErrorPageRedirect(http.StatusUnauthorized, "/login")
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	})(w, r)
	testarossa.NoError(t, err)
	testarossa.Equal(t, w.StatusCode(), http.StatusBadRequest)
}
