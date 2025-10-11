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

package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/microbus-io/fabric/errors"
)

var identifierRegexp = regexp.MustCompile(`^\w+(\.\w+)*$`)

// EvaluateBoolExp evaluates a boolean expression such as "foo=='bar' && (y==3 || y<0)".
// String literals must be quoted using single or double quotes.
// Logical operators: &&, ||, !.
// Comparison operators: ==, !=, <, <=, >, >=, =~ (regexp), !~ (negate regexp).
func EvaluateBoolExp(boolExp string, symbols map[string]any) (bool, error) {
	b, err := evaluateBoolExp(boolExp, flattenSymbolsMap(symbols))
	return b, errors.Trace(err)
}

// flattenSymbolsMap flattens the symbols map into a shallow dot-notated map,
// while normalizing all arrays to []any and all maps to map[string]any.
func flattenSymbolsMap(symbols map[string]any) map[string]any {
	// Normalize all numbers to float64, all arrays to []any and all maps to map[string]any
	j, err := json.Marshal(symbols)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(j, &symbols)
	if err != nil {
		return nil
	}

	// Flatten the symbols into a shallow dot-notated map
	flattenedSymbols := map[string]any{}
	var flatten func(obj map[string]any, prefix string)
	flatten = func(obj map[string]any, prefix string) {
		for k, v := range obj {
			flattenedSymbols[prefix+k] = v
			switch vv := v.(type) {
			case map[string]any:
				flatten(vv, prefix+k+".")
			case []any:
				for _, a := range vv {
					flattenedSymbols[fmt.Sprintf("%s%s.%v", prefix, k, a)] = true
				}
			default:
				flattenedSymbols[prefix+k] = vv
			}
		}
	}
	flatten(symbols, "")
	return flattenedSymbols
}

// evaluateBoolExp evaluates the boolean expressing, assuming that the input symbols have been flattened and normalized.
func evaluateBoolExp(boolExp string, flattenedSymbols map[string]any) (bool, error) {
	// Resolve parenthesized sub-expressions
	parenLoop := true
	for parenLoop {
		parenStart := -1
		parenDepth := 0
		var quote byte
		for i := range boolExp {
			if boolExp[i] == '\'' || boolExp[i] == '"' || boolExp[i] == '`' {
				if boolExp[i] == quote {
					// Close quote
					quote = 0
				} else {
					// Open quote
					quote = boolExp[i]
				}
				continue
			}
			if quote != 0 {
				continue
			}
			if boolExp[i] == '(' {
				parenDepth++
				if parenStart < 0 {
					parenStart = i
				}
			} else if boolExp[i] == ')' {
				parenDepth--
				if parenDepth == 0 {
					subEval, err := evaluateBoolExp(boolExp[parenStart+1:i], flattenedSymbols)
					if err != nil {
						return false, errors.Trace(err)
					}
					boolExp = boolExp[:parenStart] + " " + strconv.FormatBool(subEval) + " " + boolExp[i+1:]
					break
				}
			}
		}
		if parenDepth != 0 {
			return false, errors.New("invalid parenthesis pattern")
		}
		if parenStart < 0 {
			break
		}
	}

	// True/false constants
	boolExp = strings.TrimSpace(boolExp)
	if strings.EqualFold(boolExp, "true") {
		return true, nil
	}
	if strings.EqualFold(boolExp, "false") {
		return false, nil
	}

	// Split by ||
	parts := strings.Split(boolExp, "||")
	if len(parts) > 1 {
		for _, part := range parts {
			subEval, err := evaluateBoolExp(part, flattenedSymbols)
			if err != nil {
				return false, errors.Trace(err)
			}
			if subEval {
				return true, nil
			}
		}
		return false, nil
	}

	// Split by &&
	parts = strings.Split(boolExp, "&&")
	if len(parts) > 1 {
		for _, part := range parts {
			subEval, err := evaluateBoolExp(part, flattenedSymbols)
			if err != nil {
				return false, errors.Trace(err)
			}
			if !subEval {
				return false, nil
			}
		}
		return true, nil
	}

	// Binary operators
	var before, after, operator string
	for _, op := range []string{"==", "!=", "<=", ">=", "=~", "!~", "<", ">"} {
		var found bool
		if before, after, found = strings.Cut(boolExp, op); found {
			operator = op
			break
		}
	}
	if operator != "" {
		x := evalValue(before, flattenedSymbols)
		y := evalValue(after, flattenedSymbols)
		switch operator {
		case "==":
			return sameType(x, y) && eq(x, y), nil
		case "!=":
			return !sameType(x, y) || !eq(x, y), nil
		case "<=":
			return sameType(x, y) && (eq(x, y) || lt(x, y)), nil
		case ">=":
			return sameType(x, y) && (eq(x, y) || lt(y, x)), nil
		case "=~": // regexp
			xs, ok := x.(string)
			if !ok {
				return false, nil
			}
			ys, ok := y.(string)
			if !ok {
				return false, nil
			}
			matched, err := regexp.MatchString(ys, xs)
			if err != nil {
				return false, errors.New("invalid regexp '%s'", y)
			}
			return matched, nil
		case "!~": // negative regexp
			xs, ok := x.(string)
			if !ok {
				return false, nil
			}
			ys, ok := y.(string)
			if !ok {
				return false, nil
			}
			matched, err := regexp.MatchString(ys, xs)
			if err != nil {
				return false, errors.New("invalid regexp '%s'", y)
			}
			return !matched, nil
		case "<":
			return sameType(x, y) && lt(x, y), nil
		case ">":
			return sameType(x, y) && lt(y, x), nil
		}
	}

	// Operator !
	not := false
	for strings.HasPrefix(boolExp, "!") {
		not = !not
		boolExp = strings.TrimSpace(boolExp[1:])
	}
	// Existence
	v := evalValue(boolExp, flattenedSymbols)
	if IsNil(v) {
		// Verify it's an identifier x.y.z
		matched := identifierRegexp.MatchString(boolExp)
		if !matched {
			return false, errors.New("invalid identifier '%s'", boolExp)
		}
	}
	b, ok := v.(bool)
	if !ok {
		b = !empty(v)
	}
	if not {
		return !b, nil
	}
	return b, nil
}

// sameType returns true if x and y are of the same type.
func sameType(x any, y any) bool {
	return reflect.TypeOf(x) == reflect.TypeOf(y)
}

// empty returns true if x is nil or the zero value for its type.
func empty(x any) bool {
	if IsNil(x) {
		return true
	}
	switch v := x.(type) {
	case string:
		return v == ""
	case float64:
		return v == 0
	case bool:
		return !v
	default:
		return false
	}
}

// eq returns true if x and y are of the same type and x==y.
func eq(x any, y any) bool {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		return false
	}
	if IsNil(x) && IsNil(y) {
		return true
	}
	switch v := x.(type) {
	case string:
		return v == y.(string)
	case float64:
		return v == y.(float64)
	case bool:
		return v == y.(bool)
	default:
		return false
	}
}

// lt returns true if x and y are of the same type and x<y.
func lt(x any, y any) bool {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		return false
	}
	switch v := x.(type) {
	case string:
		return v < y.(string)
	case float64:
		return v < y.(float64)
	default:
		return false
	}
}

// evalValue returns the value of a terminal expression.
func evalValue(v string, symbols map[string]any) any {
	v = strings.TrimSpace(v)

	// Symbol
	if symbols != nil {
		if s, ok := symbols[v]; ok {
			return s
		}
	}
	// String
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	if strings.HasPrefix(v, `'`) && strings.HasSuffix(v, `'`) && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	if strings.HasPrefix(v, "`") && strings.HasSuffix(v, "`") && len(v) >= 2 {
		return v[1 : len(v)-1]
	}
	// Number
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	// Boolean
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return nil
}
