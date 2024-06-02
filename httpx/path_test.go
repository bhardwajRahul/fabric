/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

package httpx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpx_JoinHostAndPath(t *testing.T) {
	assert.Equal(t, "https://example.com:443", JoinHostAndPath("example.com", ""))
	assert.Equal(t, "https://example.com:443/", JoinHostAndPath("example.com", "/"))
	assert.Equal(t, "https://example.com:443/path", JoinHostAndPath("example.com", "/path"))
	assert.Equal(t, "https://example.com:443/path", JoinHostAndPath("example.com", "path"))
	assert.Equal(t, "https://example.com:123", JoinHostAndPath("example.com", ":123"))
	assert.Equal(t, "https://example.com:123/path", JoinHostAndPath("example.com", ":123/path"))
	assert.Equal(t, "https://example.org:123/path", JoinHostAndPath("example.com", "https://example.org:123/path"))
	assert.Equal(t, "https://example.org:123/path", JoinHostAndPath("example.com", "//example.org:123/path"))
}

func TestHttpx_ParseURLValid(t *testing.T) {
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
		assert.NoError(t, err, "%s", k)
		assert.Equal(t, v, u.String())
	}
}

func TestHttpx_ParseURLInvalid(t *testing.T) {
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
		assert.Error(t, err, "%s", x)
		assert.Nil(t, u)
	}
}
