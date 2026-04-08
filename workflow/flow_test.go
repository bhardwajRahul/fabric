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

package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

func TestFlow_GetSetString(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("name", "Alice")
	assert.NoError(err)
	assert.Equal("Alice", f.GetString("name"))
}

func TestFlow_GetSetInt(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("count", 42)
	assert.NoError(err)
	assert.Equal(42, f.GetInt("count"))
}

func TestFlow_GetSetFloat(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("score", 3.14)
	assert.NoError(err)
	assert.Equal(3.14, f.GetFloat("score"))
}

func TestFlow_GetSetBool(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("valid", true)
	assert.NoError(err)
	assert.True(f.GetBool("valid"))
}

func TestFlow_GetSetStrings(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("tags", []string{"a", "b"})
	assert.NoError(err)
	got := f.GetStrings("tags")
	assert.Equal(2, len(got))
	assert.Equal("a", got[0])
	assert.Equal("b", got[1])
}

func TestFlow_GetSetDuration(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	err := f.Set("timeout", 5*time.Second)
	assert.NoError(err)
	assert.Equal(5*time.Second, f.GetDuration("timeout"))
}

func TestFlow_GetComplex(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	type Order struct {
		ID    string `json:"id"`
		Total int    `json:"total"`
	}
	f := NewFlow()
	err := f.Set("order", &Order{ID: "abc", Total: 100})
	assert.NoError(err)
	var got Order
	err = f.Get("order", &got)
	assert.NoError(err)
	assert.Equal("abc", got.ID)
	assert.Equal(100, got.Total)
}

func TestFlow_GetMissing(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	assert.Equal("", f.GetString("missing"))
	assert.Equal(0, f.GetInt("missing"))
}

func TestFlow_SetTracksChanges(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Set("a", "1")
	f.Set("b", "2")
	assert.Equal(2, len(f.changes))
	assert.Equal(`"1"`, string(f.changes["a"].(json.RawMessage)))
}

func TestFlow_ParseStateAndSetChanges(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Set("name", "Alice")
	f.Set("score", 10)
	f.Set("extra", "untouched")
	f.changes = make(map[string]any) // reset changes from Set calls
	snap := f.Snapshot()
	var view struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}
	err := f.ParseState(&view)
	assert.NoError(err)
	assert.Equal("Alice", view.Name)
	assert.Equal(10, view.Score)

	// Modify only score
	view.Score = 25
	err = f.SetChanges(view, snap)
	assert.NoError(err)

	// Only score should be in changes
	assert.Equal(1, len(f.changes))
	assert.Equal("25", string(f.changes["score"].(json.RawMessage)))
	// Extra field should be untouched
	assert.Equal(`"untouched"`, string(f.state["extra"].(json.RawMessage)))
}

func TestFlow_SetChangesNoChanges(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Set("name", "Alice")
	f.changes = make(map[string]any) // reset changes from Set calls
	snap := f.Snapshot()
	var view struct {
		Name string `json:"name"`
	}
	err := f.ParseState(&view)
	assert.NoError(err)
	// No modifications
	err = f.SetChanges(view, snap)
	assert.NoError(err)
	assert.Equal(0, len(f.changes))
}

func TestFlow_Has(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	assert.False(f.Has("missing"))
	f.Set("name", "Alice")
	assert.True(f.Has("name"))
}

func TestFlow_Goto(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Goto("next-task")
	assert.Equal("next-task", f.gotoNext)
}

func TestFlow_Interrupt(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Interrupt(map[string]any{"request": "ssn"})
	assert.True(f.interrupt)
	assert.Equal("ssn", f.interruptPayload["request"])
}

func TestFlow_RetryNow(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	ok := f.RetryNow()
	assert.True(ok)
	assert.True(f.retry)
}

func TestFlow_Retry(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Attempt 0: should retry
	f := NewFlow()
	f.attempt = 0
	ok := f.Retry(3, time.Second, 2.0, 10*time.Second)
	assert.True(ok)
	assert.True(f.retry)
	maxAttempts, initialDelay, multiplier, maxDelay, requested := f.RetryRequested()
	assert.True(requested)
	assert.Equal(3, maxAttempts)
	assert.Equal(time.Second, initialDelay)
	assert.Equal(2.0, multiplier)
	assert.Equal(10*time.Second, maxDelay)

	// Attempt 2: should retry (still under max of 3)
	f2 := NewFlow()
	f2.attempt = 2
	ok = f2.Retry(3, time.Second, 2.0, 10*time.Second)
	assert.True(ok)
	assert.True(f2.retry)

	// Attempt 3: exhausted
	f3 := NewFlow()
	f3.attempt = 3
	ok = f3.Retry(3, time.Second, 2.0, 10*time.Second)
	assert.False(ok)
	assert.False(f3.retry)
}

func TestFlow_Sleep(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	f := NewFlow()
	f.Sleep(10 * time.Second)
	assert.Equal(10*time.Second, f.sleepDuration)
}

func TestFlow_MarshalUnmarshal(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	original := NewFlow()
	original.Set("name", "Alice")
	original.Goto("next")
	original.Retry(5, time.Second, 2.0, 30*time.Second)
	original.Sleep(5 * time.Second)
	original.Interrupt(map[string]any{"request": "ssn"})
	original.attempt = 2

	data, err := json.Marshal(original)
	assert.NoError(err)

	restored := NewFlow()
	err = json.Unmarshal(data, restored)
	assert.NoError(err)

	assert.Equal("Alice", restored.GetString("name"))
	assert.Equal("next", restored.gotoNext)
	assert.True(restored.retry)
	assert.Equal(5*time.Second, restored.sleepDuration)
	assert.True(restored.interrupt)
	assert.Equal("ssn", restored.interruptPayload["request"])
	assert.Equal(2, restored.attempt)
	assert.Equal(5, restored.backoffMaxAttempts)
	assert.Equal(time.Second, restored.backoffInitialDelay)
	assert.Equal(2.0, restored.backoffDelayMultiplier)
	assert.Equal(30*time.Second, restored.backoffMaxDelay)
}
