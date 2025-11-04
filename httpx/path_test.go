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
	"net/url"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestHttpx_JoinHostAndPath(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	assert.Equal("https://example.com:443", JoinHostAndPath("example.com", ""))
	assert.Equal("https://example.com:443/", JoinHostAndPath("example.com", "/"))
	assert.Equal("https://example.com:443/path", JoinHostAndPath("example.com", "/path"))
	assert.Equal("https://example.com:443/path", JoinHostAndPath("example.com", "path"))
	assert.Equal("https://example.com:123", JoinHostAndPath("example.com", ":123"))
	assert.Equal("https://example.com:123/path", JoinHostAndPath("example.com", ":123/path"))
	assert.Equal("https://example.org:123/path", JoinHostAndPath("example.com", "https://example.org:123/path"))
	assert.Equal("https://example.org:123/path", JoinHostAndPath("example.com", "//example.org:123/path"))
}

func TestHttpx_ParseURLValid(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	valid := map[string]string{
		"https://example.com:123/path":         "https://example.com:123/path",
		"https://example.com/path":             "https://example.com:443/path",
		"https://example.com/":                 "https://example.com:443/",
		"https://example.com":                  "https://example.com:443",
		"https://example":                      "https://example:443",
		"//example.com/path":                   "https://example.com:443/path",
		"http://example.com/path":              "http://example.com:80/path",
		"https://example.com/path/sub?q=1&m=2": "https://example.com:443/path/sub?q=1&m=2",
		"https://example.com:123/0":            "https://example.com:123/0",
	}

	for k, v := range valid {
		u, err := ParseURL(k)
		assert.NoError(err, "%s", k)
		assert.Equal(v, u.String())
	}
}

func TestHttpx_ParseURLInvalid(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	invalid := []string{
		"https://example.com:BAD/path",
		"https://$.com:123/path",
		"https://example..com/path",
		"https://example.com:123:456/path",
		"example.com/path",
		"/example.com/path",
		"/path",
		"",
	}
	for _, x := range invalid {
		u, err := ParseURL(x)
		assert.Error(err, "%s", x)
		assert.Nil(u)
	}
}

func TestHttpx_FillPathArguments(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	testCases := []string{
		"https://example.com/article/{user}/comment/{comment}?user=123&comment=456", "https://example.com/article/123/comment/456",
		"https://example.com/article/{user}/comment/{comment}?user=123&comment=456&x=789", "https://example.com/article/123/comment/456?x=789",
		"https://example.com/article/{}/comment/{}?path1=123&path2=456&x=789", "https://example.com/article/123/comment/456?x=789",
		"https://example.com/fixed/{named}/{}/{suffix+}?named=1&path2=2&suffix=3/4&q=5", "https://example.com/fixed/1/2/3/4?q=5",
		"https://example.com/fixed/{named}/{}/{suffix+}", "https://example.com/fixed///",
		"https://example.com/fixed/{named}/{suffix+}?named=" + url.QueryEscape("[a&b/c]") + "&suffix=" + url.QueryEscape("[d&e/f]"), "https://example.com/fixed/" + url.PathEscape("[a&b/c]") + "/" + url.PathEscape("[d&e") + "/" + url.PathEscape("f]"),
	}
	for i := 0; i < len(testCases); i += 2 {
		resolved, err := FillPathArguments(testCases[i])
		if assert.NoError(err) {
			assert.Equal(testCases[i+1], resolved)
		}
	}
}

func TestHttpx_ResolveURL(t *testing.T) {
	testCases := []struct {
		base     string
		rel      string
		expected string
	}{
		// Full path resolution
		{
			base:     "https://example.com/foo",
			rel:      "/bar",
			expected: "https://example.com/bar",
		},
		// Full URL override
		{
			base:     "https://example.com/foo",
			rel:      "https://company.io/bar",
			expected: "https://company.io/bar",
		},
		// Empty relative URL returns base
		{
			base:     "https://example.com/foo",
			rel:      "",
			expected: "https://example.com/foo",
		},
		// Relative path resolution
		{
			base:     "https://example.com/foo/bar",
			rel:      "baz",
			expected: "https://example.com/foo/baz",
		},
		{
			base:     "https://example.com/foo/bar/",
			rel:      "baz",
			expected: "https://example.com/foo/bar/baz",
		},
		// Path traversal with ../
		{
			base:     "https://example.com/foo/bar",
			rel:      "../baz",
			expected: "https://example.com/baz",
		},
		{
			base:     "https://example.com/foo/bar/qux",
			rel:      "../../baz",
			expected: "https://example.com/baz",
		},
		// Fragment handling
		{
			base:     "https://example.com/foo",
			rel:      "#section",
			expected: "https://example.com/foo#section",
		},
		{
			base:     "https://example.com/foo",
			rel:      "/bar#section",
			expected: "https://example.com/bar#section",
		},
		// Query string handling
		{
			base:     "https://example.com/foo?old=1",
			rel:      "?new=2",
			expected: "https://example.com/foo?new=2",
		},
		{
			base:     "https://example.com/foo",
			rel:      "bar?q=1&r=2",
			expected: "https://example.com/bar?q=1&r=2",
		},
		// Curly braces preservation (special handling in ResolveURL)
		{
			base:     "https://example.com/foo",
			rel:      "/api/{id}/details",
			expected: "https://example.com/api/{id}/details",
		},
		{
			base:     "https://example.com/api",
			rel:      "users/{userId}/posts/{postId}",
			expected: "https://example.com/users/{userId}/posts/{postId}",
		},
		// Protocol-relative URLs
		{
			base:     "http://example.com/foo",
			rel:      "//company.io/bar",
			expected: "http://company.io/bar",
		},
		// Absolute path with query and fragment
		{
			base:     "https://example.com/old/path",
			rel:      "/new/path?q=1#top",
			expected: "https://example.com/new/path?q=1#top",
		},
		// Combined query and fragment
		{
			base:     "https://example.com/foo",
			rel:      "bar?q=1#section",
			expected: "https://example.com/bar?q=1#section",
		},
	}

	assert := testarossa.For(t)
	for _, tc := range testCases {
		resolved, err := ResolveURL(tc.base, tc.rel)
		assert.Expect(resolved, tc.expected, err, nil)
	}
}
