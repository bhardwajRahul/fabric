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
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

var signatureKey = strings.Repeat("0123456789abcdef", 4)

func newSignedToken(claims jwt.MapClaims) string {
	x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := x.SignedString([]byte(signatureKey))
	return s
}

func validator(ctx context.Context, token string) (actor any, valid bool, err error) {
	parsedToken, _ := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return []byte(signatureKey), nil
	})
	return parsedToken.Claims, parsedToken.Valid && parsedToken.Claims.(jwt.MapClaims)["ok"].(bool), nil
}

func TestAuthorization_Validation(t *testing.T) {
	t.Parallel()

	mw := Authorization(validator)

	// Valid token
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+newSignedToken(jwt.MapClaims{"sub": "foo@example.com", "ok": true}))

	received := false
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received, _ = frame.Of(r).IfActor(`sub=="foo@example.com"`)
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.True(t, received)
	}

	// Invalid token
	w = httpx.NewResponseRecorder()
	r, _ = http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+newSignedToken(jwt.MapClaims{"sub": "foo@example.com", "ok": false}))

	received = true
	err = mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.False(t, received)
	}
}

func TestAuthorization_IncorrectSignature(t *testing.T) {
	t.Parallel()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "foo@example.com", "ok": true})
	incorrectlySignedToken, _ := token.SignedString([]byte("incorrect-signature"))

	mw := Authorization(validator)

	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+incorrectlySignedToken)

	received := true
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.False(t, received)
	}
}

func TestAuthorization_Order(t *testing.T) {
	t.Parallel()

	actor := struct {
		Sub string `json:"sub"`
		By  string `json:"by"`
	}{}

	mw := Authorization(validator)

	// Authorization cookie
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.AddCookie(&http.Cookie{
		Name:     "Authorization",
		Value:    newSignedToken(jwt.MapClaims{"sub": "foo@example.com", "by": "cookie", "ok": true}),
		MaxAge:   60,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})

	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		frame.Of(r).ParseActor(&actor)
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, "cookie", actor.By)
	}

	// Authorization: Bearer
	w = httpx.NewResponseRecorder()
	r.Header.Set("Authorization", "Bearer "+newSignedToken(jwt.MapClaims{"sub": "foo@example.com", "by": "header", "ok": true}))
	testarossa.Contains(t, r.Header.Get("Cookie"), "Authorization") // Cookie is still there but overridden by header
	err = mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		frame.Of(r).ParseActor(&actor)
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, "header", actor.By)
	}
}

func TestAuthorization_MalformedJWT(t *testing.T) {
	t.Parallel()

	validatorCalled := false
	mw := Authorization(func(ctx context.Context, token string) (actor any, valid bool, err error) {
		validatorCalled = true
		return nil, false, nil
	})

	// AuthToken header
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer This-is-not-a-JWT")

	received := true
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.True(t, validatorCalled)
		testarossa.False(t, received)
	}
}

func TestAuthorization_NoJWT(t *testing.T) {
	t.Parallel()

	validatorCalled := false
	mw := Authorization(func(ctx context.Context, token string) (actor any, valid bool, err error) {
		validatorCalled = true
		return nil, false, nil
	})

	// AuthToken header
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)

	received := true
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if testarossa.NoError(t, err) {
		testarossa.False(t, received)
		testarossa.False(t, validatorCalled)
	}
}
