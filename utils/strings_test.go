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

package utils

import (
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/testarossa"
)

func TestUtils_ToKebabCase(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"fooBar":     "foo-bar",
		"FooBar":     "foo-bar",
		"fooBAR":     "foo-bar",
		"FooBAR":     "foo-bar",
		"urlEncoder": "url-encoder",
		"URLEncoder": "url-encoder",
		"foobarX":    "foobar-x",
		"a":          "a",
		"A":          "a",
		"HTTP":       "http",
		"":           "",
	}
	for id, expected := range testCases {
		testarossa.Equal(t, expected, ToKebabCase(id), "%s", id)
	}
}

func TestUtils_LooksLikeJWT(t *testing.T) {
	t.Parallel()

	newSignedToken := func(claims jwt.MapClaims) string {
		x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := x.SignedString([]byte("0123456789abcdef0123456789abcdef"))
		return s
	}

	testarossa.True(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	testarossa.True(t, LooksLikeJWT(newSignedToken(jwt.MapClaims{})))
	testarossa.True(t, LooksLikeJWT(newSignedToken(jwt.MapClaims{"claim": "something"})))
	testarossa.True(t, LooksLikeJWT(newSignedToken(nil)))

	// Bad characters
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e$$.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e==.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")) // No padding
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e+/.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")) // Base64 URL

	// Incorrect dots
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_:XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))

	// Too short
	testarossa.False(t, LooksLikeJWT("eyXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e30.XXX"))
	testarossa.False(t, LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.X.X"))
	testarossa.False(t, LooksLikeJWT("eyX.X.X"))

	// No ey
	testarossa.False(t, LooksLikeJWT("xxXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
}

func TestUtil_StringClaimFromJWT(t *testing.T) {
	newSignedToken := func(claims jwt.MapClaims) string {
		x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := x.SignedString([]byte("0123456789abcdef0123456789abcdef"))
		return s
	}

	token := newSignedToken(jwt.MapClaims{"sub": "123456", "claim": "something", "roles": 12345})
	val, ok := StringClaimFromJWT(token, "claim")
	testarossa.True(t, ok)
	testarossa.Equal(t, "something", val)
	val, ok = StringClaimFromJWT(token, "sub")
	testarossa.True(t, ok)
	testarossa.Equal(t, "123456", val)
	val, ok = StringClaimFromJWT(token, "roles")
	testarossa.False(t, ok)
	testarossa.Equal(t, "", val)
	val, ok = StringClaimFromJWT(token, "nosuchclaim")
	testarossa.False(t, ok)
	testarossa.Equal(t, "", val)
}

func BenchmarkUtil_StringClaimFromJWT(b *testing.B) {
	newSignedToken := func(claims jwt.MapClaims) string {
		x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := x.SignedString([]byte("0123456789abcdef0123456789abcdef"))
		return s
	}
	token := newSignedToken(jwt.MapClaims{
		"sub":   "harry@hogwarts.edu",
		"claim": "something",
		"roles": "wizard student",
		"groups": []string{
			"Gryffindor",
		},
		"born": 1980,
	})

	b.ResetTimer()
	for range b.N {
		StringClaimFromJWT(token, "claim")
	}
}
