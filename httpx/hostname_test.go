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
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestHttpx_ValidateHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	valid := []string{
		"hello",
		"hello.world",
		"123.456",
		"1",
		"hello-world",
		"a.b.c.d",
		"my-service.example.com",
	}
	invalid := []string{
		"",
		" hello",
		"hello ",
		"hello world",
		"hello..world",
		"hello.",
		".hello",
		"~hello",
		"$",
		"hello_world",            // underscore is reserved for NATS subject flat-form
		"Hello",                  // uppercase rejected; caller must lowercase
		"hello.WORLD",            // uppercase rejected
		"id-foo",                 // reserved prefix
		"id-foo.bar",             // reserved prefix
		"loc-us",                 // reserved prefix
		"loc-us-west.b",          // reserved prefix
		"all",                    // reserved broadcast hostname
		"foo.all",                // reserved broadcast suffix
		"foo.bar.all",            // reserved broadcast suffix
		strings.Repeat("x", 253), // length cap
	}

	for _, x := range valid {
		assert.NoError(ValidateHostname(x), "%s", x)
	}
	for _, x := range invalid {
		assert.Error(ValidateHostname(x), "%s", x)
	}
}
