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

package claudellm

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()
	svc := &Service{} // nil modelAliases falls back to claudeDefaultAliases
	cases := map[string]string{
		"fast":            "claude-haiku-4-5", // tier alias
		"default":         "claude-sonnet-5",  // tier alias
		"smart":           "claude-opus-4-8",  // tier alias
		"opus":            "claude-opus-4-8",  // family alias
		"fable":           "claude-fable-5",   // family alias
		"claude-opus-4-8": "claude-opus-4-8",  // known concrete via prefix
		"claude-future-9": "claude-future-9",  // unlisted concrete still passes through by prefix
		"gpt-5":           "",                 // foreign vendor
		"":                "",                 // empty
	}
	for in, want := range cases {
		if got := svc.resolveModel(in); got != want {
			t.Errorf("resolveModel(%q) = %q, want %q", in, got, want)
		}
	}
}
