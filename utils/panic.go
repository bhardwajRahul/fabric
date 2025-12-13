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
	"reflect"
	"runtime"
	"strings"

	"github.com/microbus-io/errors"
)

// CatchPanic calls the given function and returns any panic as a standard error.
//
// Deprecated: Use [errors.CatchPanic] instead.
func CatchPanic(f func() error) (err error) {
	return errors.CatchPanic(f)
}

// IsNil returns true if x is nil or an interface holding nil.
func IsNil(x any) bool {
	defer func() { recover() }()
	return x == nil || reflect.ValueOf(x).IsNil()
}

// Testing indicates if the code is running inside a unit test, and if so, the test function name as well.
func Testing() (testFuncName string, underTest bool) {
	for lvl := 1; true; lvl++ {
		pc, _, _, ok := runtime.Caller(lvl)
		if !ok {
			break
		}
		runtimeFunc := runtime.FuncForPC(pc)
		funcName := runtimeFunc.Name()
		if strings.HasPrefix(funcName, "testing.") {
			// testing.tRunner is the test runner
			// testing.(*B).runN is the benchmark runner
			return testFuncName, true
		} else if strings.Contains(funcName, ".Test") || strings.Contains(funcName, ".Benchmark") {
			// Pick the top-most testing function
			if strings.Contains(funcName, ".func") {
				funcName = strings.TrimRight(funcName, "0123456789")
				funcName = strings.TrimSuffix(funcName, ".func")
			}
			testFuncName = funcName
		}
	}
	return "", false
}
