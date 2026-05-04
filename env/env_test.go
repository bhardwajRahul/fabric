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

package env

import (
	"os"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/testarossa"
)

func TestEnv_OSOverridesFile(t *testing.T) {
	// No parallel — mutates process-wide env.
	assert := testarossa.For(t)

	os.Chdir("testdata/subdir")
	defer os.Chdir("..")
	defer os.Chdir("..")

	// Real OS env wins over yaml (dotenv convention). The file value is loaded
	// only into keys that are not already set in the OS env.
	os.Setenv("X5981245X", "InOS")
	defer os.Unsetenv("X5981245X")
	Load() // re-run to reflect the new cwd
	assert.Equal("InOS", os.Getenv("X5981245X"))
	assert.Equal("InOS", Get("X5981245X"))

	// File value lands when the OS env is unset.
	os.Unsetenv("X5981245X")
	Load()
	assert.Equal("InFile", Get("X5981245X"))
	assert.Equal("InFile", os.Getenv("X5981245X"))

	// Case sensitive keys.
	assert.Equal("infile", Get("x5981245x"))
	assert.NotEqual(Get("X5981245X"), Get("x5981245x"))

	// Push / Pop manipulates the real OS env with save/restore.
	Push("X5981245X", "Pushed")
	assert.Equal("Pushed", Get("X5981245X"))
	assert.Equal("Pushed", os.Getenv("X5981245X")) // visible to third-party libs
	Pop("X5981245X")
	assert.Equal("InFile", Get("X5981245X"))
	err := errors.CatchPanic(func() error {
		Pop("X5981245X")
		return nil
	})
	assert.Error(err)

	// Push restores correctly when the prior value was unset.
	os.Unsetenv("YA1B2C3X")
	Push("YA1B2C3X", "Pushed")
	assert.Equal("Pushed", Get("YA1B2C3X"))
	Pop("YA1B2C3X")
	_, ok := os.LookupEnv("YA1B2C3X")
	assert.False(ok, "Pop must Unsetenv when the prior was unset")

	// Subdirectory yaml takes priority over ancestor yaml.
	assert.Equal("Child", Get("X35638125X"))
	os.Chdir("..")
	os.Unsetenv("X35638125X") // ensure parent dir's yaml lands fresh
	Load()
	assert.Equal("Parent", Get("X35638125X"))
	os.Chdir("subdir")
	os.Unsetenv("X35638125X")
	Load()

	// Lookup forwards to os.LookupEnv.
	_, ok = Lookup("X5981245X")
	assert.True(ok)
	_, ok = Lookup("x5981245x")
	assert.True(ok)
	_, ok = Lookup("X35638125X")
	assert.True(ok)
	_, ok = Lookup("Y5981245Y")
	assert.False(ok)
}
