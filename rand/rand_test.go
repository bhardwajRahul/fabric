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

package rand

import (
	"regexp"
	"testing"

	"github.com/microbus-io/testarossa"
)

func BenchmarkRand_AlphaNum64(b *testing.B) {
	for b.Loop() {
		AlphaNum64(16)
	}
	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/rand
	// cpu: Apple M1 Pro
	// BenchmarkRand_AlphaNum64-10    	41181684	        27.45 ns/op	      16 B/op	       1 allocs/op
}

func TestRand_AlphaNum64(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	for i := 1; i < 1024; i++ {
		an64 := AlphaNum64(i)
		assert.Len(an64, i)
		match := re.MatchString(an64)
		assert.True(match)
	}
}

func TestRand_AlphaNum32(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	re := regexp.MustCompile(`^[A-Z0-9]+$`)
	for i := 1; i < 1024; i++ {
		an32 := AlphaNum32(i)
		assert.Len(an32, i)
		match := re.MatchString(an32)
		assert.True(match)
	}
}
