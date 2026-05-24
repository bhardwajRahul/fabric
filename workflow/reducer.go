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
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/microbus-io/errors"
)

// Reducer defines how concurrent state modifications from parallel tasks are merged during fan-in.
type Reducer string

const (
	ReducerReplace Reducer = "replace" // Last write wins (default)
	ReducerAppend  Reducer = "append"  // Concatenate arrays
	ReducerAdd     Reducer = "add"     // Sum numeric values
	ReducerUnion   Reducer = "union"   // Merge arrays, deduplicate
	ReducerMerge   Reducer = "merge"   // Merge objects, new key wins
)

// Reduce applies the reducer to merge an incoming value into an existing value.
// Both existing and incoming are JSON-compatible values (json.RawMessage or native Go types).
// The result is the merged value.
func (r Reducer) Reduce(existing, incoming any) (any, error) {
	switch r {
	case ReducerReplace, "":
		return incoming, nil
	case ReducerAppend:
		return reduceAppend(existing, incoming)
	case ReducerAdd:
		return reduceAdd(existing, incoming)
	case ReducerUnion:
		return reduceUnion(existing, incoming)
	case ReducerMerge:
		return reduceMerge(existing, incoming)
	default:
		return nil, errors.New("unknown reducer: %s", string(r))
	}
}

// ReducerForFieldName returns the reducer inferred from a state field name.
// Returns empty (replace) if no convention prefix matches.
//
// Conventions:
//
//	sum*  - numeric add
//	list* - array append
//	set*  - polymorphic: array union, object merge
//
// The character right after the prefix must be uppercase to avoid matching
// English words like "summary", "listening", or "setup".
func ReducerForFieldName(name string) Reducer {
	switch {
	case hasUpperBoundary(name, "sum"):
		return ReducerAdd
	case hasUpperBoundary(name, "list"):
		return ReducerAppend
	case hasUpperBoundary(name, "set"):
		return ReducerUnion
	}
	return ""
}

// hasUpperBoundary reports whether name starts with prefix followed by an uppercase letter.
func hasUpperBoundary(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return unicode.IsUpper(r)
}

// MergeState applies changes on top of state, using the provided reducers for
// fields that have one. For fields without an explicit reducer, the reducer is
// inferred from the field name's prefix (sum*, list*, set*); fields not
// matching a convention prefix use replace semantics.
//
// The set* prefix is polymorphic: it dispatches to union when the value is a
// JSON array and to merge when the value is a JSON object. Explicit reducer
// configuration via SetReducer is strict and does not polymorph.
//
// State, changes and the result are map[string]any or nil.
func MergeState(state any, changes any, reducers map[string]Reducer) (map[string]any, error) {
	stateMap, err := toAnyMap(state)
	if err != nil {
		return nil, errors.Trace(err)
	}
	changesMap, err := toAnyMap(changes)
	if err != nil {
		return nil, errors.Trace(err)
	}
	merged := make(map[string]any, len(stateMap)+len(changesMap))
	maps.Copy(merged, stateMap)
	for k, v := range changesMap {
		existing, exists := merged[k]
		reducer, explicit := reducers[k]
		if !explicit || reducer == "" {
			reducer = ReducerForFieldName(k)
			// set* polymorphism: object value -> merge, array value -> union
			if reducer == ReducerUnion && (isJSONObject(existing) || isJSONObject(v)) {
				reducer = ReducerMerge
			}
		}
		if !exists || reducer == "" || reducer == ReducerReplace {
			merged[k] = v
			continue
		}
		merged[k], err = reducer.Reduce(existing, v)
		if err != nil {
			return nil, errors.New("reducer '%s' failed on field '%s': %w", string(reducer), k, err)
		}
	}
	return merged, nil
}

// toAnyMap converts any to map[string]any.
// Accepts map[string]any (fast path), map[string]json.RawMessage, or nil.
func toAnyMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	switch m := v.(type) {
	case map[string]any:
		return m, nil
	case map[string]json.RawMessage:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k] = val
		}
		return result, nil
	default:
		// Marshal then unmarshal to get a map
		data, err := json.Marshal(v)
		if err != nil {
			return nil, errors.Trace(err)
		}
		var result map[string]any
		err = json.Unmarshal(data, &result)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return result, nil
	}
}

// jsonKind classifies a JSON-compatible value as one of "object", "array",
// "string", "number", "bool", "null", or "" if undetermined.
func jsonKind(v any) string {
	if v == nil {
		return "null"
	}
	raw, err := marshalAny(v)
	if err != nil {
		return ""
	}
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return "object"
		case '[':
			return "array"
		case '"':
			return "string"
		case 't', 'f':
			return "bool"
		case 'n':
			return "null"
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return "number"
		default:
			return ""
		}
	}
	return ""
}

// isJSONObject reports whether v's JSON form is an object (or null, which merges harmlessly).
func isJSONObject(v any) bool {
	return jsonKind(v) == "object"
}

// reduceAppend concatenates two JSON arrays.
func reduceAppend(existing, incoming any) (any, error) {
	a, err := unmarshalArray(existing, "append")
	if err != nil {
		return nil, errors.Trace(err)
	}
	b, err := unmarshalArray(incoming, "append")
	if err != nil {
		return nil, errors.Trace(err)
	}
	result, err := json.Marshal(append(a, b...))
	return json.RawMessage(result), errors.Trace(err)
}

// reduceAdd sums two JSON numbers.
func reduceAdd(existing, incoming any) (any, error) {
	a, err := unmarshalNumber(existing, "add")
	if err != nil {
		return nil, errors.Trace(err)
	}
	b, err := unmarshalNumber(incoming, "add")
	if err != nil {
		return nil, errors.Trace(err)
	}
	result, err := json.Marshal(a + b)
	return json.RawMessage(result), errors.Trace(err)
}

// reduceUnion merges two JSON arrays, deduplicating elements by their JSON representation.
func reduceUnion(existing, incoming any) (any, error) {
	a, err := unmarshalArray(existing, "union")
	if err != nil {
		return nil, errors.Trace(err)
	}
	b, err := unmarshalArray(incoming, "union")
	if err != nil {
		return nil, errors.Trace(err)
	}
	seen := make(map[string]bool, len(a))
	for _, v := range a {
		seen[string(v)] = true
	}
	for _, v := range b {
		if !seen[string(v)] {
			a = append(a, v)
			seen[string(v)] = true
		}
	}
	result, err := json.Marshal(a)
	return json.RawMessage(result), errors.Trace(err)
}

// reduceMerge merges two JSON objects key-by-key. New keys win on collision.
func reduceMerge(existing, incoming any) (any, error) {
	a, err := unmarshalObject(existing, "merge")
	if err != nil {
		return nil, errors.Trace(err)
	}
	b, err := unmarshalObject(incoming, "merge")
	if err != nil {
		return nil, errors.Trace(err)
	}
	if a == nil {
		a = make(map[string]json.RawMessage, len(b))
	}
	for k, v := range b {
		a[k] = v
	}
	result, err := json.Marshal(a)
	return json.RawMessage(result), errors.Trace(err)
}

// A cleared slot (Go nil or JSON null) short-circuits to the reducer's identity
// (nil array, nil object, zero number) so flow.Clear contributions are ignored
// at fan-in rather than failing the type check.

func unmarshalArray(v any, reducerName string) ([]json.RawMessage, error) {
	if isCleared(v) {
		return nil, nil
	}
	raw, err := marshalAny(v)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, errors.New("%s reducer requires array, got %s", reducerName, jsonKind(v))
	}
	return arr, nil
}

func unmarshalObject(v any, reducerName string) (map[string]json.RawMessage, error) {
	if isCleared(v) {
		return nil, nil
	}
	raw, err := marshalAny(v)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, errors.New("%s reducer requires object, got %s", reducerName, jsonKind(v))
	}
	return obj, nil
}

func unmarshalNumber(v any, reducerName string) (float64, error) {
	if isCleared(v) {
		return 0, nil
	}
	raw, err := marshalAny(v)
	if err != nil {
		return 0, errors.Trace(err)
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, errors.New("%s reducer requires number, got %s", reducerName, jsonKind(v))
	}
	return n, nil
}
