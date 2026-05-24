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

func TestReducer_Merge(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// New keys win on collision; all keys retained.
	result, err := ReducerMerge.Reduce(
		json.RawMessage(`{"a":1,"b":2}`),
		json.RawMessage(`{"b":3,"c":4}`),
	)
	assert.NoError(err)
	var got map[string]any
	_ = json.Unmarshal(result.(json.RawMessage), &got)
	assert.Expect(got["a"], float64(1))
	assert.Expect(got["b"], float64(3))
	assert.Expect(got["c"], float64(4))

	// Error on non-object
	_, err = ReducerMerge.Reduce(json.RawMessage(`[1,2]`), json.RawMessage(`{"a":1}`))
	assert.Error(err)
	assert.Contains(err.Error(), "merge reducer requires object")
	assert.Contains(err.Error(), "array")
}

func TestReducer_TypeMismatchErrors(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	_, err := ReducerAdd.Reduce(json.RawMessage(`"hi"`), json.RawMessage(`1`))
	assert.Error(err)
	assert.Contains(err.Error(), "add reducer requires number")
	assert.Contains(err.Error(), "string")

	_, err = ReducerAppend.Reduce(json.RawMessage(`{"a":1}`), json.RawMessage(`[1]`))
	assert.Error(err)
	assert.Contains(err.Error(), "append reducer requires array")
	assert.Contains(err.Error(), "object")

	_, err = ReducerUnion.Reduce(json.RawMessage(`42`), json.RawMessage(`[1]`))
	assert.Error(err)
	assert.Contains(err.Error(), "union reducer requires array")
	assert.Contains(err.Error(), "number")
}

func TestReducer_NullContributionIgnored(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Add: null is identity 0 on either side.
	got, err := ReducerAdd.Reduce(json.RawMessage(`5`), json.RawMessage(`null`))
	assert.NoError(err)
	assert.Equal("5", string(got.(json.RawMessage)))
	got, err = ReducerAdd.Reduce(json.RawMessage(`null`), json.RawMessage(`7`))
	assert.NoError(err)
	assert.Equal("7", string(got.(json.RawMessage)))

	// Append: null contributes nothing.
	got, err = ReducerAppend.Reduce(json.RawMessage(`[1,2]`), json.RawMessage(`null`))
	assert.NoError(err)
	assert.Equal("[1,2]", string(got.(json.RawMessage)))
	got, err = ReducerAppend.Reduce(json.RawMessage(`null`), json.RawMessage(`[3]`))
	assert.NoError(err)
	assert.Equal("[3]", string(got.(json.RawMessage)))

	// Union: null contributes nothing.
	got, err = ReducerUnion.Reduce(json.RawMessage(`["a","b"]`), json.RawMessage(`null`))
	assert.NoError(err)
	assert.Equal(`["a","b"]`, string(got.(json.RawMessage)))

	// Merge: null contributes nothing.
	got, err = ReducerMerge.Reduce(json.RawMessage(`{"k":1}`), json.RawMessage(`null`))
	assert.NoError(err)
	assert.Equal(`{"k":1}`, string(got.(json.RawMessage)))
}

func TestReducerForFieldName(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	cases := []struct {
		name string
		want Reducer
	}{
		// Matches: prefix followed by uppercase letter.
		{"sumScore", ReducerAdd},
		{"sumX", ReducerAdd},
		{"listMessages", ReducerAppend},
		{"listItems", ReducerAppend},
		{"setUsers", ReducerUnion},
		{"setRoles", ReducerUnion},
		// English words: prefix followed by lowercase letter.
		{"summary", ""},
		{"summer", ""},
		{"listening", ""},
		{"listed", ""},
		{"setup", ""},
		{"setupTime", ""},
		{"settings", ""},
		// Bare prefix (no boundary).
		{"sum", ""},
		{"list", ""},
		{"set", ""},
		// No prefix.
		{"messages", ""},
		{"score", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := ReducerForFieldName(tc.name)
		if got != tc.want {
			t.Errorf("ReducerForFieldName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	_ = assert
}

func TestMergeState_PrefixDispatch(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	state := map[string]any{
		"sumScore":     json.RawMessage(`10`),
		"listMessages": json.RawMessage(`["a","b"]`),
		"setRoles":     json.RawMessage(`["admin"]`),
		"messages":     json.RawMessage(`"old"`),
	}
	changes := map[string]any{
		"sumScore":     json.RawMessage(`5`),
		"listMessages": json.RawMessage(`["c"]`),
		"setRoles":     json.RawMessage(`["admin","user"]`),
		"messages":     json.RawMessage(`"new"`),
	}
	merged, err := MergeState(state, changes, nil)
	if assert.NoError(err) {
		data, _ := marshalAny(merged["sumScore"])
		assert.Expect(string(data), "15")
		data, _ = marshalAny(merged["listMessages"])
		assert.Expect(string(data), `["a","b","c"]`)
		data, _ = marshalAny(merged["setRoles"])
		assert.Expect(string(data), `["admin","user"]`)
		// Non-prefix field falls through to replace.
		data, _ = marshalAny(merged["messages"])
		assert.Expect(string(data), `"new"`)
	}
}

func TestMergeState_PolymorphicSet(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// set* with array value -> union behavior.
	mergedArr, err := MergeState(
		map[string]any{"setItems": json.RawMessage(`[1,2]`)},
		map[string]any{"setItems": json.RawMessage(`[2,3]`)},
		nil,
	)
	if assert.NoError(err) {
		data, _ := marshalAny(mergedArr["setItems"])
		assert.Expect(string(data), `[1,2,3]`)
	}

	// set* with object value -> merge behavior.
	mergedObj, err := MergeState(
		map[string]any{"setUsers": json.RawMessage(`{"alice":1,"bob":2}`)},
		map[string]any{"setUsers": json.RawMessage(`{"bob":3,"carol":4}`)},
		nil,
	)
	if assert.NoError(err) {
		var got map[string]any
		_ = json.Unmarshal(mergedObj["setUsers"].(json.RawMessage), &got)
		assert.Expect(got["alice"], float64(1))
		assert.Expect(got["bob"], float64(3))
		assert.Expect(got["carol"], float64(4))
	}
}

func TestMergeState_ExplicitOverridesPrefix(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Explicit ReducerReplace on a set* field disables polymorphism.
	merged, err := MergeState(
		map[string]any{"setItems": json.RawMessage(`[1,2]`)},
		map[string]any{"setItems": json.RawMessage(`[3]`)},
		map[string]Reducer{"setItems": ReducerReplace},
	)
	if assert.NoError(err) {
		data, _ := marshalAny(merged["setItems"])
		assert.Expect(string(data), `[3]`)
	}
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
