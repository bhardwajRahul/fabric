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

package chatgptllm

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()
	svc := &Service{} // nil modelAliases falls back to chatgptDefaultAliases
	cases := map[string]string{
		"fast":         "gpt-5.4-mini", // tier alias
		"default":      "gpt-5.5",      // tier alias
		"smart":        "gpt-5.5-pro",  // tier alias
		"mini":         "gpt-5.4-mini", // family alias
		"nano":         "gpt-5.4-nano", // family alias
		"gpt-5.5":      "gpt-5.5",      // known concrete via prefix
		"o3-pro":       "o3-pro",       // reasoning-family prefix
		"gpt-future-9": "gpt-future-9", // unlisted concrete still passes through by prefix
		"claude-opus":  "",             // foreign vendor
		"":             "",             // empty
	}
	for in, want := range cases {
		if got := svc.resolveModel(in); got != want {
			t.Errorf("resolveModel(%q) = %q, want %q", in, got, want)
		}
	}
}
