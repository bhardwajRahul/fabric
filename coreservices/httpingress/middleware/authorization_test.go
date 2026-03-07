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
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

var (
	bearerTokenSigKey = strings.Repeat("0123456789abcdef", 4)
	accessTokenSigKey = strings.Repeat("0011223344556677", 4)
)

func TestAuthorization_Validation(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	mw := Authorization(exchange)

	// Valid token
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+mintBearerToken(jwt.MapClaims{"sub": "foo@example.com", "ok": true}))

	received := false
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received, _ = frame.Of(r).IfActor(`sub=="foo@example.com"`)
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.True(received)
	}

	// Invalid token
	w = httpx.NewResponseRecorder()
	r, _ = http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+mintBearerToken(jwt.MapClaims{"sub": "foo@example.com", "ok": false}))

	received = true
	err = mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.False(received)
	}
}

func TestAuthorization_IncorrectSignature(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "foo@example.com", "ok": true})
	incorrectlySignedToken, _ := token.SignedString([]byte("incorrect-signature"))

	mw := Authorization(exchange)

	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.Header.Set("Authorization", "Bearer "+incorrectlySignedToken)

	received := true
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.False(received)
	}
}

func TestAuthorization_Order(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	actor := struct {
		Sub string `json:"sub"`
		By  string `json:"by"`
	}{}

	mw := Authorization(exchange)

	// Authorization cookie
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)
	r.AddCookie(&http.Cookie{
		Name:     "Authorization",
		Value:    mintBearerToken(jwt.MapClaims{"sub": "foo@example.com", "by": "cookie", "ok": true}),
		MaxAge:   60,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})

	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		frame.Of(r).ParseActor(&actor)
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.Equal("cookie", actor.By)
	}

	// Authorization: Bearer
	w = httpx.NewResponseRecorder()
	r.Header.Set("Authorization", "Bearer "+mintBearerToken(jwt.MapClaims{"sub": "foo@example.com", "by": "header", "ok": true}))
	assert.Contains(r.Header.Get("Cookie"), "Authorization") // Cookie is still there but overridden by header
	err = mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		frame.Of(r).ParseActor(&actor)
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.Equal("header", actor.By)
	}
}

func TestAuthorization_MalformedJWT(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	validatorCalled := false
	mw := Authorization(func(ctx context.Context, token string) (actorJWT string, err error) {
		validatorCalled = true
		return "", nil
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
	if assert.NoError(err) {
		assert.True(validatorCalled)
		assert.False(received)
	}
}

func TestAuthorization_NoJWT(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	validatorCalled := false
	mw := Authorization(func(ctx context.Context, token string) (actorJWT string, err error) {
		validatorCalled = true
		return "", nil
	})

	// AuthToken header
	w := httpx.NewResponseRecorder()
	r, _ := http.NewRequest("GET", "/page", nil)

	received := true
	err := mw(func(w http.ResponseWriter, r *http.Request) (err error) {
		received = r.Header.Get(frame.HeaderActor) != ""
		return nil
	})(w, r)
	if assert.NoError(err) {
		assert.False(received)
		assert.False(validatorCalled)
	}
}

func mintBearerToken(claims jwt.MapClaims) string {
	x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := x.SignedString([]byte(bearerTokenSigKey))
	return s
}

func mintAccessToken(claims jwt.MapClaims) string {
	x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := x.SignedString([]byte(accessTokenSigKey))
	return s
}

func exchange(ctx context.Context, bearerToken string) (accessToken string, err error) {
	parsedToken, _ := jwt.Parse(bearerToken, func(t *jwt.Token) (any, error) {
		return []byte(bearerTokenSigKey), nil
	})
	if parsedToken == nil || !parsedToken.Valid {
		return "", nil
	}
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil
	}
	if okVal, exists := claims["ok"]; !exists || !okVal.(bool) {
		return "", nil
	}
	return mintAccessToken(claims), nil
}
