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

package llmapi

// Attachment is a non-text piece of content attached to a Message (image, audio, video,
// document). Exactly one of Data or URI is set per attachment:
//   - Data carries the raw bytes inline. The provider transports them base64-encoded;
//     callers pass raw bytes and never need to encode them themselves.
//   - URI references a pre-uploaded artifact at the provider's file store (e.g. Gemini's
//     File API) or a publicly accessible HTTPS URL (provider-dependent).
//
// MediaType is the IANA mime type (e.g. "image/png", "image/jpeg", "application/pdf",
// "audio/mp3"). Required for both shapes.
//
// Providers that don't support multi-modal input silently ignore Attachments. Currently
// supported by geminillm; other providers may follow.
type Attachment struct {
	MediaType string `json:"mediaType,omitzero" jsonschema_description:"MediaType is the IANA mime type (e.g. image/png, image/jpeg, application/pdf)"`
	Data      []byte `json:"data,omitzero" jsonschema_description:"Data carries inline raw bytes; mutually exclusive with URI"`
	URI       string `json:"uri,omitzero" jsonschema_description:"URI references a pre-uploaded artifact (Gemini File API URI, public HTTPS URL); mutually exclusive with Data"`
}
