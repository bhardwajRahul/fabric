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
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestUtils_ValidateHostname(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	valid := []string{
		"hello",
		"hello.WORLD",
		"123.456",
		"1",
		"hello_world",
		"hello-world",
	}
	invalid := []string{
		"hello world",
		"hello..world",
		"hello.",
		".hello",
		"~hello",
		"$",
		"",
	}

	for _, x := range valid {
		tt.NoError(ValidateHostname(x), "%s", x)
	}
	for _, x := range invalid {
		tt.Error(ValidateHostname(x), "%s", x)
	}
}

func TestUtils_ValidateConfigName(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	valid := []string{
		"hello",
		"WORLD",
		"hello123",
		"hello-world",
		"hello_world",
	}
	invalid := []string{
		"hello world",
		"hello.world",
		"1hello",
		"_hello",
		"",
	}

	for _, x := range valid {
		tt.NoError(ValidateConfigName(x), "%s", x)
	}
	for _, x := range invalid {
		tt.Error(ValidateConfigName(x), "%s", x)
	}
}

func TestUtils_ValidateTickerName(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	valid := []string{
		"hello",
		"WORLD",
		"hello123",
		"hello-world",
		"hello_world",
	}
	invalid := []string{
		"hello world",
		"hello.world",
		"1hello",
		"_hello",
		"",
	}

	for _, x := range valid {
		tt.NoError(ValidateTickerName(x), "%s", x)
	}
	for _, x := range invalid {
		tt.Error(ValidateTickerName(x), "%s", x)
	}
}
