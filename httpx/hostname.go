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

package httpx

import (
	"regexp"
	"strings"

	"github.com/microbus-io/errors"
)

var (
	hostnameValidator = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]+)*$`)
)

// ValidateHostname checks that the string is a canonical Microbus service identity.
// Rules:
//   - Length up to 252 characters.
//   - Only lowercase letters, digits, dot separators, and hyphens. No underscores. No uppercase.
//   - Segments are separated by single dots; no leading, trailing, or consecutive dots.
//   - The first segment may not start with the reserved prefixes "id-" or "loc-".
//   - The hostname is not "all" and does not end in ".all".
//
// The caller is responsible for normalization (trim, lowercase). Non-canonical input
// produces an error rather than being silently coerced.
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return errors.New("invalid hostname '%s'", hostname)
	}
	if len(hostname) >= 253 {
		return errors.New("invalid hostname '%s'", hostname)
	}
	if !hostnameValidator.MatchString(hostname) {
		return errors.New("invalid hostname '%s'", hostname)
	}
	if strings.HasPrefix(hostname, "id-") || strings.HasPrefix(hostname, "loc-") {
		return errors.New("invalid hostname '%s' (reserved prefix)", hostname)
	}
	if hostname == "all" || strings.HasSuffix(hostname, ".all") {
		return errors.New("invalid hostname '%s' (reserved)", hostname)
	}
	return nil
}
