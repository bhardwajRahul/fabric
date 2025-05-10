/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package utils

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unsafe"
)

var reUpperCaseIdentifier = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)

// IsUpperCaseIdentifier accepts only UpperCaseIdentifiers.
func IsUpperCaseIdentifier(id string) bool {
	return reUpperCaseIdentifier.MatchString(id)
}

var reLowerCaseIdentifier = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// IsLowerCaseIdentifier accepts only lowerCaseIdentifiers.
func IsLowerCaseIdentifier(id string) bool {
	return reLowerCaseIdentifier.MatchString(id)
}

// ToKebabCase converts a CamelCase identifier to kebab-case.
func ToKebabCase(id string) string {
	idRunes := []rune(id)
	n := len(idRunes)
	if n == 0 {
		return id
	}
	idRunes = append(idRunes, rune('x')) // Terminal
	var sb strings.Builder
	sb.WriteRune(unicode.ToLower(idRunes[0]))

	for i := 1; i < n; i++ {
		rPrev := idRunes[i-1]
		r := idRunes[i]
		rNext := idRunes[i+1]
		if unicode.IsUpper(r) {
			switch {
			case unicode.IsLower(rPrev) && unicode.IsLower(rNext):
				// ooXoo
				sb.WriteByte('-')
			case unicode.IsUpper(rPrev) && unicode.IsUpper(rNext):
				// oOXOo
				break
			case unicode.IsUpper(rPrev) && unicode.IsLower(rNext):
				if i < n-1 {
					// oOXoo
					sb.WriteByte('-')
				} else {
					// oooOX
					break
				}
			case unicode.IsLower(rPrev) && unicode.IsUpper(rNext):
				// ooXOo
				sb.WriteByte('-')
			}
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}

// ToSnakeCase converts a CamelCase identifier to snake_case.
func ToSnakeCase(id string) string {
	return strings.ReplaceAll(ToKebabCase(id), "-", "_")
}

// LooksLikeJWT checks if the token is likely to be a signed representation of a JWT.
func LooksLikeJWT(token string) bool {
	// Shortest JWT using HS256 algorithm and no payload:
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.8VKCTiBegJPuPIZlp0wbV0Sbdn5BS6TE5DCx6oYNc5o
	if len(token) < 36+1+3+1+43 {
		return false
	}
	// A JWT starts with {" which encodes to ey in base64
	if !strings.HasPrefix(token, "ey") {
		return false
	}
	// Identify the sections
	sectionLen := [3]int{0, 0, 0}
	dots := 0
	for _, rn := range token {
		if rn == '.' {
			dots++
			if dots > 2 {
				return false
			}
		} else if !(rn >= 'A' && rn <= 'Z' || rn >= 'a' && rn <= 'z' || rn >= '0' && rn <= '9' || rn == '-' || rn == '_') {
			return false
		}
		sectionLen[dots]++
	}
	if dots != 2 || sectionLen[0] < 36 || sectionLen[1] < 3 || sectionLen[2] < 43 {
		return false
	}
	return true
}

var base64DecoderPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4096)
		return &buf
	},
}

// StringClaimFromJWT extracts a claim from a JWT with minimal memory allocations without fully parsing it.
// The claim must be a string, i.e. appear as "name":"value" in the claim part.
func StringClaimFromJWT(token string, name string) (value string, ok bool) {
	if !LooksLikeJWT(token) {
		return "", false
	}
	dot1 := strings.Index(token, ".")
	dot2 := strings.LastIndex(token, ".")
	if dot1 < 0 || dot2 <= dot1 {
		return "", false
	}
	encoded := UnsafeStringToBytes(token[dot1+1 : dot2])
	buf := base64DecoderPool.Get().(*[]byte)
	defer base64DecoderPool.Put(buf)
	decoded := *buf
	n, err := base64.RawURLEncoding.Decode(decoded, encoded)
	if err != nil {
		return "", false
	}
	if n == len(decoded) {
		decoded, err = base64.RawURLEncoding.DecodeString(token[dot1+1 : dot2])
		if err != nil {
			return "", false
		}
	}
	nameAsBytes := UnsafeStringToBytes(name)
	p := bytes.Index(decoded, nameAsBytes)
	if p < 0 || p > len(decoded)-5 {
		return "", false
	}
	p += len(nameAsBytes)
	if !bytes.HasPrefix(decoded[p:], []byte(`":"`)) {
		return "", false
	}
	p += 3
	q := bytes.Index(decoded[p:], []byte(`"`))
	if q < 0 {
		return "", false
	}
	return string(decoded[p : p+q]), true
}

// UnsafeStringToBytes converts a string to a slice of bytes with no memory allocation.
// The slice points to the original data of the string and should not be modified.
func UnsafeStringToBytes(s string) []byte {
	pStr := unsafe.StringData(s)
	return unsafe.Slice(pStr, len(s))
}

// UnsafeBytesToString converts a slice of bytes to a string with no memory allocation.
// The original byte slice data should not be modified.
func UnsafeBytesToString(b []byte) string {
	pBytes := unsafe.SliceData(b)
	return unsafe.String(pBytes, len(b))
}
