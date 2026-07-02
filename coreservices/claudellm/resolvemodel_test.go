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

import (
	"maps"
	"testing"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

func TestResolveModel(t *testing.T) {
	t.Parallel()
	// A populated table short-circuits ensureAliases (no fetch/key needed). It holds the aliases; concrete claude-
	// names resolve by prefix passthrough.
	svc := &Service{}
	svc.modelAliases = map[string]string{
		"fast": "claude-haiku-4-5", "default": "claude-sonnet-5", "smart": "claude-opus-4-8",
		"claude-haiku-latest": "claude-haiku-4-5", "claude-sonnet-latest": "claude-sonnet-5",
		"claude-opus-latest": "claude-opus-4-8", "claude-fable-latest": "claude-fable-5",
	}
	cases := map[string]string{
		"fast":                   "claude-haiku-4-5",       // tier alias
		"default":                "claude-sonnet-5",        // tier alias
		"smart":                  "claude-opus-4-8",        // tier alias
		"claude-opus-latest":     "claude-opus-4-8",        // namespaced synthesized alias resolves via table
		"claude-sonnet-4-latest": "claude-sonnet-4-latest", // real vendor -latest we don't synthesize passes through
		"opus":                   "",                       // generic family word is not a global alias
		"claude-opus-4-8":        "claude-opus-4-8",        // concrete passes through without the table
		"claude-future-9":        "claude-future-9",        // unlisted concrete still passes through by prefix
		"gpt-5":                  "",                       // foreign vendor
		"":                       "",                       // empty
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

func TestBuildClaudeAliases(t *testing.T) {
	t.Parallel()

	// An empty list yields an empty table (no shipped defaults to fall back on).
	if got := buildClaudeAliases(nil); len(got) != 0 {
		t.Errorf("buildClaudeAliases(nil) = %v, want empty", got)
	}

	// A subset of the live catalog (2026-07); created ordering selects the newest model of each family, including
	// a dated haiku snapshot that overrides the shipped dateless default.
	models := []listedModel{
		{id: "claude-fable-5", created: 100},
		{id: "claude-opus-4-8", created: 99},
		{id: "claude-sonnet-5", created: 98},
		{id: "claude-opus-4-7", created: 90},
		{id: "claude-sonnet-4-6", created: 89},
		{id: "claude-haiku-4-5-20251001", created: 88},
		{id: "claude-opus-4-1-20250805", created: 50},
		{id: "text-embedding-foo", created: 200}, // non-claude, ignored
	}
	want := map[string]string{
		llmapi.ModelFast:       "claude-haiku-4-5-20251001",
		llmapi.ModelDefault:    "claude-sonnet-5",
		llmapi.ModelSmart:      "claude-opus-4-8",
		"claude-haiku-latest":  "claude-haiku-4-5-20251001",
		"claude-sonnet-latest": "claude-sonnet-5",
		"claude-opus-latest":   "claude-opus-4-8",
		"claude-fable-latest":  "claude-fable-5",
	}
	if got := buildClaudeAliases(models); !maps.Equal(got, want) {
		t.Errorf("buildClaudeAliases = %v, want %v", got, want)
	}
}
