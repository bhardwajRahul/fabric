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

package schema

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

// Goldens mirror connector/subjects_test.go. Drift here breaks the wire format.
func TestFlattenHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	cases := []struct{ in, want string }{
		{"www.sub.example.com", "www_sub_example_com"},
		{"www.example.com", "www_example_com"},
		{"example.com", "example_com"},
		{"com", "com"},
		{"hostname", "hostname"},
		{"", ""},
		{"example.", "example_"},
		{".example", "_example"},
		{"..", "__"},
		{"a_b.c", "a_b_c"},
		{"a.b.c", "a_b_c"},
		// URL-special chars in route hostnames: encoded as lowercase %xx; '.' still flattens to '_'.
		{"my$.xml", "my%24_xml"},
		{"a!b.c", "a%21b_c"},
		{"a~b.c", "a%7eb_c"},
	}
	for _, tc := range cases {
		assert.Equal(tc.want, FlattenHostname(tc.in), "FlattenHostname(%q)", tc.in)
	}
}

func TestEncodePathSegment(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	cases := []struct{ in, want string }{
		// Case is preserved (paths are case-sensitive).
		{"UPPERCASE", "UPPERCASE"},
		// '.' encoded uniformly with everything else.
		{"file.html", "file%2ehtml"},
		// Underscore encoded so a literal '_' segment doesn't collide with the empty-segment marker.
		{"a_b_c", "a%5fb%5fc"},
		// Hyphens and digits pass through.
		{"two-W0rds", "two-W0rds"},
		// URL specials and '*' get encoded so they don't collide with NATS wildcards.
		{"special!", "special%21"},
		{"asteri*", "asteri%2a"},
		// Empty segment - caller's responsibility, but the encoder handles it gracefully.
		{"", ""},
		// Combined: uppercase + period + special.
		{"UPPER.xml", "UPPER%2exml"},
	}
	for _, tc := range cases {
		assert.Equal(tc.want, EncodePathSegment(tc.in), "EncodePathSegment(%q)", tc.in)
	}
}

func TestReverseHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	cases := []struct{ in, want string }{
		{"www.sub.example.com", "com.example.sub.www"},
		{"www.example.com", "com.example.www"},
		{"example.com", "com.example"},
		{"com", "com"},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(tc.want, ReverseHostname(tc.in), "ReverseHostname(%q)", tc.in)
	}
}

func TestExposedRoutes(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	m := &Manifest{
		Webs:      map[string]Route{"W": {Method: "GET", Route: ":443/w"}},
		Functions: map[string]Route{"F": {Method: "POST", Route: ":444/f"}},
		Tasks:     map[string]Route{"T": {Route: ":428/t"}},
		Workflows: map[string]Route{"X": {Route: ":428/x"}},
	}
	assert.Equal(4, len(m.ExposedRoutes()))
}
