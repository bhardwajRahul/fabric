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

	"github.com/microbus-io/testarossa"
)

func TestReducer_Replace(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	result, err := ReducerReplace.Reduce(json.RawMessage(`"old"`), json.RawMessage(`"new"`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `"new"`)
}

func TestReducer_Append(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	result, err := ReducerAppend.Reduce(json.RawMessage(`[1,2]`), json.RawMessage(`[3,4]`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `[1,2,3,4]`)

	// Append with different types
	result, err = ReducerAppend.Reduce(json.RawMessage(`["a","b"]`), json.RawMessage(`["c"]`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `["a","b","c"]`)

	// Error on non-array
	_, err = ReducerAppend.Reduce(json.RawMessage(`"not an array"`), json.RawMessage(`[1]`))
	assert.Error(err)
}

func TestReducer_Add(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	result, err := ReducerAdd.Reduce(json.RawMessage(`10`), json.RawMessage(`5`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `15`)

	// Floating point
	result, err = ReducerAdd.Reduce(json.RawMessage(`1.5`), json.RawMessage(`2.5`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `4`)

	// Error on non-number
	_, err = ReducerAdd.Reduce(json.RawMessage(`"not a number"`), json.RawMessage(`1`))
	assert.Error(err)
}

func TestReducer_Union(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	result, err := ReducerUnion.Reduce(json.RawMessage(`[1,2,3]`), json.RawMessage(`[2,3,4]`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `[1,2,3,4]`)

	// Strings
	result, err = ReducerUnion.Reduce(json.RawMessage(`["a","b"]`), json.RawMessage(`["b","c"]`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `["a","b","c"]`)

	// No overlap
	result, err = ReducerUnion.Reduce(json.RawMessage(`[1]`), json.RawMessage(`[2]`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `[1,2]`)

	// Error on non-array
	_, err = ReducerUnion.Reduce(json.RawMessage(`"not an array"`), json.RawMessage(`[1]`))
	assert.Error(err)
}

func TestReducer_EmptyDefault(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Empty string reducer should behave like replace
	result, err := Reducer("").Reduce(json.RawMessage(`"old"`), json.RawMessage(`"new"`))
	assert.NoError(err)
	assert.Expect(string(result.(json.RawMessage)), `"new"`)
}

func TestReducer_Unknown(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	_, err := Reducer("bogus").Reduce(json.RawMessage(`1`), json.RawMessage(`2`))
	assert.Error(err)
}

func TestMergeState_Replace(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	state := map[string]any{"a": 1, "b": 2}
	changes := map[string]any{"b": 3, "c": 4}
	merged, err := MergeState(state, changes, nil)
	if assert.NoError(err) {
		m := merged
		data, _ := marshalAny(m["a"])
		assert.Expect(string(data), "1")
		data, _ = marshalAny(m["b"])
		assert.Expect(string(data), "3")
		data, _ = marshalAny(m["c"])
		assert.Expect(string(data), "4")
	}
}

func TestMergeState_WithReducers(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	state := map[string]any{
		"count": json.RawMessage(`10`),
		"items": json.RawMessage(`[1,2]`),
		"name":  json.RawMessage(`"old"`),
	}
	changes := map[string]any{
		"count": json.RawMessage(`5`),
		"items": json.RawMessage(`[3]`),
		"name":  json.RawMessage(`"new"`),
	}
	reducers := map[string]Reducer{
		"count": ReducerAdd,
		"items": ReducerAppend,
	}
	merged, err := MergeState(state, changes, reducers)
	if assert.NoError(err) {
		m := merged
		data, _ := marshalAny(m["count"])
		assert.Expect(string(data), "15")
		data, _ = marshalAny(m["items"])
		assert.Expect(string(data), "[1,2,3]")
		data, _ = marshalAny(m["name"])
		assert.Expect(string(data), `"new"`)
	}
}

func TestMergeState_NilInputs(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Nil state
	merged, err := MergeState(nil, map[string]any{"a": 1}, nil)
	if assert.NoError(err) {
		data, _ := marshalAny(merged["a"])
		assert.Expect(string(data), "1")
	}

	// Nil changes
	merged, err = MergeState(map[string]any{"a": 1}, nil, nil)
	if assert.NoError(err) {
		data, _ := marshalAny(merged["a"])
		assert.Expect(string(data), "1")
	}

	// Both nil
	merged, err = MergeState(nil, nil, nil)
	if assert.NoError(err) {
		assert.Expect(len(merged), 0)
	}
}
