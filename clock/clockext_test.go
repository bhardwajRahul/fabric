/*
Copyright 2023 Microbus LLC and various contributors

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

package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClock_NewMockAt(t *testing.T) {
	t.Parallel()

	tm := time.Date(2022, 10, 27, 20, 26, 15, 0, time.Local)
	mock := NewMockAt(tm)
	assert.True(t, tm.Equal(mock.Now()))

	mock = NewMockAtDate(2022, 10, 27, 20, 26, 15, 0, time.Local)
	assert.True(t, tm.Equal(mock.Now()))

	mock = NewMockAtNow()
	assert.WithinDuration(t, time.Now(), mock.Now(), 100*time.Millisecond)
}