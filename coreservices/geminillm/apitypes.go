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

import "encoding/json"

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	Tools             []geminiToolDec  `json:"tools,omitzero"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitzero"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitzero"`
}

type geminiGenConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitzero"`
	Temperature     float64 `json:"temperature,omitzero"`
}

type geminiContent struct {
	Role  string       `json:"role,omitzero"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string          `json:"text,omitzero"`
	Thought          bool            `json:"thought,omitzero"`
	ThoughtSignature string          `json:"thoughtSignature,omitzero"`
	FunctionCall     *geminiFuncCall `json:"functionCall,omitzero"`
	FunctionResponse *geminiFuncResp `json:"functionResponse,omitzero"`
	// InlineData carries non-text content (images, audio, video, documents) directly in the
	// request/response. Data must be the raw bytes - encoding/json's base64 treatment of []byte
	// matches Gemini's wire format exactly, so no manual encoding is needed.
	InlineData *geminiInlineData `json:"inlineData,omitzero"`
	// FileData references a pre-uploaded artifact (Gemini File API URI like
	// "https://generativelanguage.googleapis.com/v1beta/files/abc-123") or a public HTTPS URL.
	FileData *geminiFileData `json:"fileData,omitzero"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

type geminiFileData struct {
	MimeType string `json:"mimeType,omitzero"`
	FileURI  string `json:"fileUri"`
}

type geminiFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFuncResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiToolDec struct {
	FunctionDeclarations []geminiFunc `json:"functionDeclarations"`
}

type geminiFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitzero"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	ModelVersion  string              `json:"modelVersion"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
}

type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	// ThoughtsTokenCount is the number of tokens spent on internal reasoning by Gemini 2.5
	// thinking models. Billed but reported separately from CandidatesTokenCount. We fold this
	// into llmapi.Usage.OutputTokens so OutputTokens reflects total billed completion across
	// providers, and surface the breakdown via llmapi.Usage.ThinkingTokens.
	ThoughtsTokenCount int `json:"thoughtsTokenCount"`
}
