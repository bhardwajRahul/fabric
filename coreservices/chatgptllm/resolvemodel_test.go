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

import (
	"maps"
	"testing"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

func TestResolveModel(t *testing.T) {
	t.Parallel()
	// A populated table short-circuits ensureAliases (no fetch/key needed). It holds the aliases; concrete names
	// resolve by prefix passthrough.
	svc := &Service{}
	svc.modelAliases = map[string]string{
		"fast": "gpt-5.4-mini", "default": "gpt-5.5", "smart": "gpt-5.5-pro",
		"gpt-latest": "gpt-5.5", "gpt-pro-latest": "gpt-5.5-pro",
		"gpt-mini-latest": "gpt-5.4-mini", "gpt-nano-latest": "gpt-5.4-nano",
	}
	cases := map[string]string{
		"fast":            "gpt-5.4-mini", // tier alias
		"default":         "gpt-5.5",      // tier alias
		"smart":           "gpt-5.5-pro",  // tier alias
		"mini":            "",             // generic family word is not a global alias
		"gpt-mini-latest": "gpt-5.4-mini", // namespaced synthesized alias resolves via table
		"gpt-latest":      "gpt-5.5",      // synthesized floating alias resolves via table, not prefix passthrough
		"gpt-5.5":         "gpt-5.5",      // concrete (prefix + version digit) passes through without the table
		"gpt-9.9":         "gpt-9.9",      // unlisted concrete still passes through by prefix
		"o3-pro":          "o3-pro",       // o-series variant passes through
		"o3":              "o3",           // bare o-series is concrete
		"o1":              "o1",           // bare o-series is concrete
		"claude-opus":     "",             // foreign vendor
		"":                "",             // empty
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

func TestBuildChatgptAliases(t *testing.T) {
	t.Parallel()

	// An empty list yields an empty table (no shipped defaults to fall back on).
	if got := buildChatgptAliases(nil); len(got) != 0 {
		t.Errorf("buildChatgptAliases(nil) = %v, want empty", got)
	}

	// A subset of the live catalog. The latest per variant is the highest version, tie-broken by created; older
	// families, preview/dated names, non-tier suffixes, and non-text models are ignored.
	models := []listedModel{
		{id: "gpt-5.5-pro", created: 100},
		{id: "gpt-5.5", created: 99},
		{id: "gpt-5.4", created: 500},         // newer timestamp but lower version; gpt-5.5 still wins default
		{id: "gpt-5.6-preview", created: 600}, // highest version but a preview; excluded, so gpt-5.5 stays
		{id: "gpt-5.4-mini", created: 90},
		{id: "gpt-5.4-nano", created: 89},
		{id: "gpt-5-codex", created: 200},            // non-tier suffix, ignored
		{id: "gpt-5.5-chat", created: 201},           // chat variant, ignored
		{id: "gpt-5.5-pro-preview", created: 700},    // multi-segment, excluded; gpt-5.5-pro stays smart
		{id: "gpt-4o", created: 300},                 // <5, ignored
		{id: "o3", created: 300},                     // o-series has no tier alias
		{id: "text-embedding-3-large", created: 400}, // non-text, ignored
	}
	want := map[string]string{
		llmapi.ModelFast:    "gpt-5.4-mini",
		llmapi.ModelDefault: "gpt-5.5",
		llmapi.ModelSmart:   "gpt-5.5-pro",
		"gpt-latest":        "gpt-5.5",
		"gpt-pro-latest":    "gpt-5.5-pro",
		"gpt-mini-latest":   "gpt-5.4-mini",
		"gpt-nano-latest":   "gpt-5.4-nano",
	}
	if got := buildChatgptAliases(models); !maps.Equal(got, want) {
		t.Errorf("buildChatgptAliases = %v, want %v", got, want)
	}
}

func TestIsReasoningModel(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"o1":                  true,  // o-series
		"o3":                  true,  // o-series
		"o3-pro":              true,  // o-series variant
		"o4-mini":             true,  // o-series variant
		"gpt-5":               true,  // gpt-5 cutoff
		"gpt-5.5":             true,  // gpt-5+
		"gpt-5.4-mini":        true,  // gpt-5+ variant
		"gpt-6":               true,  // future gpt above cutoff
		"gpt-6.2-pro":         true,  // future gpt variant
		"gpt-next":            true,  // unversioned gpt defaults to reasoning
		"gpt-5.5-chat":        false, // gpt-5+ chat variant is non-reasoning
		"gpt-5.5-chat-latest": false, // gpt-5+ chat variant is non-reasoning
		"gpt-4o":              false, // pre-5 chat family
		"gpt-4.1":             false, // pre-5 chat family
		"gpt-4o-mini":         false, // pre-5 chat family
		"gpt-3.5-turbo":       false, // pre-5 chat family
		"claude-opus-4-8":     false, // foreign vendor
		"o":                   false, // bare o, no digit
		"":                    false, // empty
	}
	for in, want := range cases {
		if got := isReasoningModel(in); got != want {
			t.Errorf("isReasoningModel(%q) = %v, want %v", in, got, want)
		}
	}
}
