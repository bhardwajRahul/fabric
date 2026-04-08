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

	"github.com/microbus-io/errors"
)

// Reducer defines how concurrent state modifications from parallel tasks are merged during fan-in.
type Reducer string

const (
	ReducerReplace Reducer = "replace" // Last write wins (default)
	ReducerAppend  Reducer = "append"  // Concatenate arrays
	ReducerAdd     Reducer = "add"     // Sum numeric values
	ReducerUnion   Reducer = "union"   // Merge arrays, deduplicate
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
	default:
		return nil, errors.New("unknown reducer: %s", string(r))
	}
}

// MergeState applies changes on top of state, using the provided reducers for
// fields that have one. Fields without an explicit reducer use replace semantics.
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
		reducer := reducers[k] // defaults to "" which is replace
		existing, exists := merged[k]
		if !exists || reducer == "" || reducer == ReducerReplace {
			merged[k] = v
		} else {
			merged[k], err = reducer.Reduce(existing, v)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}
	return merged, nil
}

// FilterState returns a subset of state based on declared field names.
// nil or empty = pass nothing.
// ["*"] = pass everything.
// Named fields = pass only those fields.
func FilterState(state map[string]any, declared []string) map[string]any {
	if len(declared) == 0 {
		return make(map[string]any)
	}
	if len(declared) == 1 && declared[0] == "*" {
		return state
	}
	filtered := make(map[string]any, len(declared))
	for _, field := range declared {
		if v, ok := state[field]; ok {
			filtered[field] = v
		}
	}
	return filtered
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

// reduceAppend concatenates two JSON arrays.
func reduceAppend(existing, incoming any) (any, error) {
	existingRaw, err := marshalAny(existing)
	if err != nil {
		return nil, errors.Trace(err)
	}
	incomingRaw, err := marshalAny(incoming)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var a, b []json.RawMessage
	if err := json.Unmarshal(existingRaw, &a); err != nil {
		return nil, errors.Trace(err)
	}
	if err := json.Unmarshal(incomingRaw, &b); err != nil {
		return nil, errors.Trace(err)
	}
	result, err := json.Marshal(append(a, b...))
	return json.RawMessage(result), errors.Trace(err)
}

// reduceAdd sums two JSON numbers.
func reduceAdd(existing, incoming any) (any, error) {
	existingRaw, err := marshalAny(existing)
	if err != nil {
		return nil, errors.Trace(err)
	}
	incomingRaw, err := marshalAny(incoming)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var a, b float64
	if err := json.Unmarshal(existingRaw, &a); err != nil {
		return nil, errors.Trace(err)
	}
	if err := json.Unmarshal(incomingRaw, &b); err != nil {
		return nil, errors.Trace(err)
	}
	result, err := json.Marshal(a + b)
	return json.RawMessage(result), errors.Trace(err)
}

// reduceUnion merges two JSON arrays, deduplicating elements by their JSON representation.
func reduceUnion(existing, incoming any) (any, error) {
	existingRaw, err := marshalAny(existing)
	if err != nil {
		return nil, errors.Trace(err)
	}
	incomingRaw, err := marshalAny(incoming)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var a, b []json.RawMessage
	if err := json.Unmarshal(existingRaw, &a); err != nil {
		return nil, errors.Trace(err)
	}
	if err := json.Unmarshal(incomingRaw, &b); err != nil {
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
