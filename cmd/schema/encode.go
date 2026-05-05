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

package schema

import "strings"

// hexDigits is the lowercase hex alphabet used for percent-encoded bytes.
// Mirrors connector.subjectHexDigits.
const hexDigits = "0123456789abcdef"

// FlattenHostname encodes a hostname for use as a single NATS subject segment.
// '.' becomes '_'; characters outside [A-Za-z0-9_-] are percent-encoded as %XX
// (uppercase hex). Mirrors connector.escapeHostname; output must be byte-
// identical so emitted ACL subjects match what the runtime publishes.
func FlattenHostname(hostname string) string {
	if !needsHostnameEscape(hostname) {
		return hostname
	}
	var b strings.Builder
	b.Grow(len(hostname) + 8)
	for i := 0; i < len(hostname); i++ {
		c := hostname[i]
		switch {
		case c == '.':
			b.WriteByte('_')
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '-', c == '_':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0xF])
		}
	}
	return b.String()
}

// EncodePathSegment percent-encodes a single URL path segment for inclusion in
// a NATS subject. Every byte outside [A-Za-z0-9-] becomes %XX (uppercase hex).
// Mirrors connector.escapePathPart. Wildcard sentinels are the caller's
// responsibility; this helper assumes a literal segment.
func EncodePathSegment(segment string) string {
	if !needsPathEscape(segment) {
		return segment
	}
	var b strings.Builder
	b.Grow(len(segment) + 8)
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		switch {
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '-':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0xF])
		}
	}
	return b.String()
}

// ReverseHostname reverses dot-separated segments: www.example.com -> com.example.www.
func ReverseHostname(hostname string) string {
	if !strings.ContainsRune(hostname, '.') {
		return hostname
	}
	parts := strings.Split(hostname, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}

func needsHostnameEscape(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			continue
		}
		return true
	}
	return false
}

func needsPathEscape(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
			continue
		}
		return true
	}
	return false
}
