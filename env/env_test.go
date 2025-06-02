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

package env

import (
	"os"
	"testing"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/testarossa"
)

func TestEnv_FileOverridesOS(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	os.Chdir("testdata")
	defer os.Chdir("..")

	// File overrides OS
	os.Setenv("X5981245X", "InOS")
	defer os.Unsetenv("X5981245X")
	tt.Equal("InOS", os.Getenv("X5981245X"))
	tt.Equal("InFile", Get("X5981245X"))

	// Case sensitive keys
	tt.Equal("infile", Get("x5981245x"))
	tt.NotEqual(Get("X5981245X"), os.Getenv("x5981245x"))

	// Push/pop
	Push("X5981245X", "Pushed")
	tt.Equal("Pushed", Get("X5981245X"))
	Pop("X5981245X")
	tt.Equal("InFile", Get("X5981245X"))
	err := errors.CatchPanic(func() error {
		Pop("X5981245X")
		return nil
	})
	tt.Error(err)

	// Lookup
	_, ok := Lookup("X5981245X")
	tt.True(ok)
	_, ok = Lookup("x5981245x")
	tt.True(ok)
	_, ok = Lookup("Y5981245Y")
	tt.False(ok)
}
