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

package geminillm

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()
	svc := &Service{} // nil modelAliases falls back to geminiDefaultAliases
	cases := map[string]string{
		"fast":            "gemini-3.1-flash-lite",  // tier alias
		"default":         "gemini-flash-latest",    // tier alias (floating pointer)
		"smart":           "gemini-3.1-pro-preview", // tier alias
		"flash":           "gemini-flash-latest",    // family alias (floating pointer)
		"pro":             "gemini-3.1-pro-preview", // family alias
		"flash-lite":      "gemini-3.1-flash-lite",  // family alias
		"gemini-2.5-pro":  "gemini-2.5-pro",         // known concrete via prefix
		"gemini-future-9": "gemini-future-9",        // unlisted concrete still passes through by prefix
		"gpt-5":           "",                       // foreign vendor
		"":                "",                       // empty
	}
	for in, want := range cases {
		if got := svc.resolveModel(in); got != want {
			t.Errorf("resolveModel(%q) = %q, want %q", in, got, want)
		}
	}
}
