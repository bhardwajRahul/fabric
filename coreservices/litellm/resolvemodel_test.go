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

package litellm

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()
	// A populated model_name set short-circuits ensureAliases (no fetch/key needed). LiteLLM's model_name is the
	// alias, so a held name resolves to itself and anything else returns "".
	svc := &Service{}
	svc.modelNames = map[string]bool{"smart": true, "gpt-4o": true, "my-model": true}
	cases := map[string]string{
		"smart":    "smart",    // an operator-named tier the proxy exposes
		"gpt-4o":   "gpt-4o",   // a held model_name
		"my-model": "my-model", // an arbitrary held model_name
		"opus":     "",         // not in the proxy's model_list
		"gpt-5.5":  "",         // not in the proxy's model_list
		"":         "",         // empty
	}
	for in, want := range cases {
		got, err := svc.resolveModel(t.Context(), in)
		if err != nil {
			t.Fatalf("resolveModel(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("resolveModel(%q) = %q, want %q", in, got, want)
		}
	}
}
