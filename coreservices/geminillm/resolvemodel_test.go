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

import (
	"maps"
	"slices"
	"testing"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

func TestResolveModel(t *testing.T) {
	t.Parallel()
	// A populated table short-circuits ensureAliases (no fetch/key needed). It holds the aliases; concrete gemini-
	// names (and -latest pointers) resolve by prefix passthrough.
	svc := &Service{}
	svc.modelAliases = map[string]string{
		"fast": "gemini-flash-lite-latest", "default": "gemini-flash-latest", "smart": "gemini-pro-latest",
	}
	cases := map[string]string{
		"fast":              "gemini-flash-lite-latest", // tier alias (floating pointer)
		"default":           "gemini-flash-latest",      // tier alias (floating pointer)
		"smart":             "gemini-pro-latest",        // tier alias (floating pointer)
		"pro":               "",                         // generic family word is not a global alias
		"gemini-pro-latest": "gemini-pro-latest",        // the family floating pointer works via prefix passthrough
		"gemini-2.5-pro":    "gemini-2.5-pro",           // concrete passes through without the table
		"gemini-future-9":   "gemini-future-9",          // unlisted concrete still passes through by prefix
		"gpt-5":             "",                         // foreign vendor
		"":                  "",                         // empty
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

func TestBuildGeminiAliases(t *testing.T) {
	t.Parallel()

	// An empty list yields an empty table (no shipped defaults to fall back on).
	if got := buildGeminiAliases(nil); len(got) != 0 {
		t.Errorf("buildGeminiAliases(nil) = %v, want empty", got)
	}

	// Each tier maps to its floating -latest pointer, kept only when that pointer is a live text model. Here the
	// flash-lite pointer is absent, so the fast tier is dropped.
	models := []listedModel{
		{id: "gemini-flash-latest", methods: []string{"generateContent"}},
		{id: "gemini-pro-latest", methods: []string{"generateContent"}},
		{id: "gemini-2.5-flash-image-preview", methods: []string{"generateContent"}}, // denied by name
	}
	want := map[string]string{
		llmapi.ModelDefault: "gemini-flash-latest",
		llmapi.ModelSmart:   "gemini-pro-latest",
	}
	if got := buildGeminiAliases(models); !maps.Equal(got, want) {
		t.Errorf("buildGeminiAliases = %v, want %v", got, want)
	}
}

func TestGeminiTextModelIDs(t *testing.T) {
	t.Parallel()
	// The list is noisy: only models advertising generateContent and not named like a non-text family survive.
	models := []listedModel{
		{id: "gemini-flash-latest", methods: []string{"generateContent", "countTokens"}},
		{id: "gemini-pro-latest", methods: []string{"generateContent"}},
		{id: "gemini-2.5-flash-image-preview", methods: []string{"generateContent"}}, // denied by name (image)
		{id: "gemini-2.0-flash-tts", methods: []string{"generateContent"}},           // denied by name (tts)
		{id: "text-embedding-004", methods: []string{"embedContent"}},                // no generateContent
		{id: "imagen-3.0-generate", methods: []string{"predict"}},                    // no generateContent + denied
	}
	want := []string{"gemini-flash-latest", "gemini-pro-latest"}
	if got := geminiTextModelIDs(models); !slices.Equal(got, want) {
		t.Errorf("geminiTextModelIDs = %v, want %v", got, want)
	}
}
