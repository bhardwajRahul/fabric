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

package httpx

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/utils"
)

var jsonNumberRegexp = regexp.MustCompile(`^(\-?)(0|([1-9][0-9]*))(\.[0-9]+)?([eE][\+\-]?[0-9]+)?$`)

// EncodeDeepObject encodes an object into string representation with bracketed nested fields names.
// For example, color[R]=100&color[G]=200&color[B]=150 .
func EncodeDeepObject(obj any) (url.Values, error) {
	buf, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var m map[string]any
	err = json.Unmarshal(buf, &m)
	if err != nil {
		return nil, errors.Trace(err)
	}
	result := make(url.Values)
	encodeOne("", m, result)
	return result, nil
}

func encodeOne(prefix string, obj any, values url.Values) {
	var val string
	switch fieldObj := obj.(type) {
	case map[string]any:
		for k, v := range fieldObj {
			if prefix == "" {
				encodeOne(k, v, values)
			} else {
				encodeOne(prefix+"["+k+"]", v, values)
			}
		}
		return
	case string:
		val = fieldObj
	case bool:
		val = strconv.FormatBool(fieldObj)
	case int64:
		val = strconv.FormatInt(fieldObj, 10)
	case int:
		val = strconv.FormatInt(int64(fieldObj), 10)
	case float64:
		if fieldObj == float64(int64(fieldObj)) {
			val = strconv.FormatInt(int64(fieldObj), 10)
		} else {
			val = strconv.FormatFloat(fieldObj, 'g', -1, 64)
		}
	case float32:
		if float64(fieldObj) == float64(int64(fieldObj)) {
			val = strconv.FormatInt(int64(fieldObj), 10)
		} else {
			val = strconv.FormatFloat(float64(fieldObj), 'g', -1, 64)
		}
	default:
		if obj == nil {
			val = "null"
		} else {
			val = utils.AnyToString(fieldObj)
		}
	}
	values.Set(prefix, val)
}

// DecodeDeepObject decodes an object from a string representation with bracketed or dot-notation
// nested field names. For example, color[R]=100&color[G]=200&color[B]=150 or color.R=100&color.G=200.
// Maps whose keys are sequential integers starting from 0 are decoded as arrays. For example,
// x[0]=a&x[1]=b&x[2]=c is decoded as {"x":["a","b","c"]}.
// It builds a single JSON object from all values and unmarshals it in one pass.
func DecodeDeepObject(values url.Values, obj any) error {
	tree := make(map[string]any)
	for k, vv := range values {
		// Normalize: convert bracket notation a[b][c] to dot notation a.b.c
		k = strings.ReplaceAll(k, "]", "")
		k = strings.ReplaceAll(k, "[", ".")
		segments := strings.Split(k, ".")

		// Use the last value for each key (matching url.Values.Get semantics)
		v := vv[len(vv)-1]

		// Walk down the tree, creating nested maps as needed
		node := tree
		for i, seg := range segments {
			if i == len(segments)-1 {
				// Leaf: detect the value type
				node[seg] = detectValue(v)
			} else {
				// Intermediate: ensure a nested map exists
				next, ok := node[seg].(map[string]any)
				if !ok {
					next = make(map[string]any)
					node[seg] = next
				}
				node = next
			}
		}
	}
	converted := mapsToSlices(tree)
	buf, err := json.Marshal(converted)
	if err != nil {
		return errors.Trace(err)
	}
	err = json.Unmarshal(buf, obj)
	if err == nil {
		return nil
	}
	// If unmarshaling fails due to a type mismatch (e.g. JSON number into a Go string field),
	// retry with all leaf values as strings.
	if _, ok := err.(*json.UnmarshalTypeError); ok {
		allStrings := mapsToSlices(leafsToStrings(tree))
		buf, err = json.Marshal(allStrings)
		if err != nil {
			return errors.Trace(err)
		}
		err = json.Unmarshal(buf, obj)
	}
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// mapsToSlices recursively converts maps whose keys are sequential integers starting from 0
// into slices. For example, map["0"]="a", map["1"]="b" becomes ["a", "b"].
func mapsToSlices(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	// Recurse into children first
	for k, child := range m {
		m[k] = mapsToSlices(child)
	}
	// Check if all keys are sequential integers 0..len-1
	if len(m) == 0 {
		return m
	}
	for i := range len(m) {
		if _, ok := m[strconv.Itoa(i)]; !ok {
			return m
		}
	}
	// Convert to a slice
	s := make([]any, len(m))
	for i := range s {
		s[i] = m[strconv.Itoa(i)]
	}
	return s
}

// leafsToStrings recursively converts all non-map, non-nil leaf values to strings.
func leafsToStrings(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		if v == nil {
			return nil
		}
		return utils.AnyToString(v)
	}
	out := make(map[string]any, len(m))
	for k, child := range m {
		out[k] = leafsToStrings(child)
	}
	return out
}

// detectValue infers the Go type of a query parameter value string.
func detectValue(v string) any {
	switch {
	case v == "":
		return ""
	case v == "null":
		return nil
	case v == "true":
		return true
	case v == "false":
		return false
	case jsonNumberRegexp.MatchString(v):
		// Parse as json.Number to preserve precision through marshal/unmarshal
		var n json.Number
		n = json.Number(v)
		return n
	default:
		return v
	}
}
