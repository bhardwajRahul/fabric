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

package utils

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestUtils_Testing(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Should detect when running inside the main test
	fn, ok := Testing()
	assert.Expect(
		fn, "github.com/microbus-io/fabric/utils.TestUtils_Testing",
		ok, true,
	)

	done := make(chan bool)
	go func() {
		// Can't detect when running inside a goroutine
		fn, ok := Testing()
		assert.Expect(
			fn, "",
			ok, false,
		)
		done <- true
	}()
	<-done

	t.Run("sub_test", func(t *testing.T) {
		// Should detect when running inside a sub-test
		fn, ok := Testing()
		assert.Expect(
			fn, "github.com/microbus-io/fabric/utils.TestUtils_Testing",
			ok, true,
		)
	})
}
