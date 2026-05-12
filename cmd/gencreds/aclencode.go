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

package main

import (
	"strings"

	"github.com/microbus-io/fabric/cmd/internal/schema"
)

// encodePath converts a URL path into its NATS subject form. Must produce
// byte-identical output to connector.escapePathPart so emitted ACL subjects
// match what subjectOf produces at runtime. Wildcard sentinels (`*`, `>`)
// remain literal segments here; per-segment percent-encoding goes through
// schema.EncodePathSegment.
func encodePath(path string) string {
	if path == "" || path == "/" {
		return "_"
	}
	path = strings.TrimPrefix(path, "/")
	segs := strings.Split(path, "/")
	encoded := make([]string, len(segs))
	for i, s := range segs {
		encoded[i] = encodeSegment(s)
	}
	return strings.Join(encoded, ".")
}

// encodeSegment encodes a single path segment, preserving the wildcard
// sentinels Microbus uses for path arguments.
func encodeSegment(s string) string {
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "...}") {
		return ">"
	}
	if s == "*" || (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		return "*"
	}
	return schema.EncodePathSegment(s)
}
