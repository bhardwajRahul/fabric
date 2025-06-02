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
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/testarossa"
)

func TestUtils_ToKebabCase(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

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

		"Foo BAR":    "foo-bar",
		"Foo  b A R": "foo-b-a-r",
		"Foo_BAR":    "foo-bar",
		"Foo___bAR":  "foo-b-ar",
		"Foo_ BAR":   "foo-bar",
		"Foo _ BAR":  "foo-bar",
		" FooBAR":    "-foo-bar",
		"_FooBAR":    "-foo-bar",
		"_ Foo-_BAR": "-foo-bar",

		"Foo123":        "foo-123",
		"123-foo":       "123-foo",
		" 123-foo":      "-123-foo",
		"_123-foo_":     "-123-foo-",
		"Foo123Bar":     "foo-123-bar",
		"Foo123bar":     "foo-123-bar",
		"Foo 123 bar":   "foo-123-bar",
		"foo 1 2 3 bar": "foo-1-2-3-bar",
	}
	for id, expected := range testCases {
		actual := ToKebabCase(id)
		tt.Equal(expected, actual, "expected %s, got %s, in %s", expected, actual, id)
	}
}

func TestUtils_LooksLikeJWT(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	newSignedToken := func(claims jwt.MapClaims) string {
		x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := x.SignedString([]byte("0123456789abcdef0123456789abcdef"))
		return s
	}

	tt.True(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	tt.True(LooksLikeJWT(newSignedToken(jwt.MapClaims{})))
	tt.True(LooksLikeJWT(newSignedToken(jwt.MapClaims{"claim": "something"})))
	tt.True(LooksLikeJWT(newSignedToken(nil)))

	// Bad characters
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e$$.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e==.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")) // No padding
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e+/.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")) // Base64 URL

	// Incorrect dots
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_:XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))

	// Too short
	tt.False(LooksLikeJWT("eyXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.e30.XXX"))
	tt.False(LooksLikeJWT("eyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.X.X"))
	tt.False(LooksLikeJWT("eyX.X.X"))

	// No ey
	tt.False(LooksLikeJWT("xxXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.eyABCDEFGHIJKLMNOPQRSTUVWZYZabcdefghijklmnopqrstuvwzyz01234567890-_.XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
}

func TestUtil_StringClaimFromJWT(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	newSignedToken := func(claims jwt.MapClaims) string {
		x := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, _ := x.SignedString([]byte("0123456789abcdef0123456789abcdef"))
		return s
	}

	token := newSignedToken(jwt.MapClaims{"sub": "123456", "claim": "something", "roles": 12345})
	val, ok := StringClaimFromJWT(token, "claim")
	tt.True(ok)
	tt.Equal("something", val)
	val, ok = StringClaimFromJWT(token, "sub")
	tt.True(ok)
	tt.Equal("123456", val)
	val, ok = StringClaimFromJWT(token, "roles")
	tt.False(ok)
	tt.Equal("", val)
	val, ok = StringClaimFromJWT(token, "nosuchclaim")
	tt.False(ok)
	tt.Equal("", val)
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

func TestUtil_AnyToString(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	tc := map[string]any{
		"string":        "string",
		"5":             5,
		"123.45":        123.45,
		"TextMarshaler": &textMarshaler{},
		"Stringer":      &stringer{},
		"Error!":        errors.New("Error!"),
		"true":          true,
		"false":         false,
	}
	for expected, o := range tc {
		actual := AnyToString(o)
		tt.Equal(expected, actual)
	}
}

type textMarshaler struct{}

func (tm *textMarshaler) MarshalText() ([]byte, error) {
	return []byte("TextMarshaler"), nil
}

type stringer struct{}

func (s *stringer) String() string {
	return "Stringer"
}
