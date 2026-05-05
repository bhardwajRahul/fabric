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

package main

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestSubsumes(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	cases := []struct {
		a, b string
		want bool
	}{
		// Equal patterns are not subsumption: dedup must not erase
		// duplicates to nothing.
		{"a.b.c", "a.b.c", false},
		{">", ">", false},
		{"*", "*", false},

		// `>` subsumes any non-empty trailing tail at that position.
		{"x.>", "x.y", true},
		{"x.>", "x.y.z", true},
		{"x.>", "x.y.z.w", true},
		// But `>` requires at least one more segment - it does not
		// subsume the bare prefix or a strictly-shorter pattern.
		{"x.>", "x", false},
		{"a.b.>", "a.b", false},

		// Top-level `>` subsumes any non-empty subject.
		{">", "a", true},
		{">", "a.b.c", true},

		// `*` subsumes exactly one segment at the same position.
		{"a.*", "a.b", true},
		{"a.*.c", "a.b.c", true},
		{"*.b.c", "a.b.c", true},
		// `*` cannot stand in for "no segment" or "many segments".
		{"a.*", "a", false},
		{"a.*", "a.b.c", false},
		// Mixed `*` segments still need exact length unless `>` is
		// involved.
		{"a.*.>", "a.b.c", true},
		{"a.*.>", "a.b.c.d", true},
		{"a.*.>", "a.b", false},

		// Differing literal segments do not subsume in either direction.
		{"a.b", "a.c", false},
		{"a.b.c", "a.b.d", false},

		// Length mismatches with no `>` to absorb the tail.
		{"a.b", "a.b.c", false},
		{"a.b.c", "a.b", false},

		// Cross-position wildcard / literal: `*` cannot subsume a
		// different literal at a different index.
		{"a.*", "*.b", false},
		{"*.b", "a.*", false},
	}
	for _, c := range cases {
		assert.Equal(c.want, subsumes(c.a, c.b), "subsumes(%q, %q)", c.a, c.b)
	}
}

func TestDedupVerb(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Sibling subsumption: the broader pattern survives, the narrower one drops.
	in := []string{"x.a", "x.b", "x.>"}
	out := dedupVerb(append([]string(nil), in...))
	assert.Equal(1, len(out), "broader-survives length")
	if len(out) > 0 {
		assert.Equal("x.>", out[0])
	}

	// Equal patterns are not subsumption: both copies pass through unchanged.
	// This is by design - subsumes(a, a) == false, so dedup never erases
	// duplicates to nothing. Exact-duplicate input is rare in practice
	// because rule construction buckets by canonical form upstream.
	in = []string{"x.y", "x.y"}
	out = dedupVerb(append([]string(nil), in...))
	assert.Equal(2, len(out), "dedupVerb dup")

	// No subsumption: nothing drops.
	in = []string{"a.b", "a.c", "x.y"}
	out = dedupVerb(append([]string(nil), in...))
	assert.Equal(3, len(out), "dedupVerb disjoint")
}
