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

package errors_test

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/testarossa"
)

func TestErrors_RuntimeTrace(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	file, function, line1, _ := errors.RuntimeTrace(0)
	_, _, line2, _ := errors.RuntimeTrace(0)
	tt.Contains(file, "errors_test.go")
	tt.Equal("errors_test.TestErrors_RuntimeTrace", function)
	tt.Equal(line1+1, line2)
}

func TestErrors_Newf(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.Newf("Error %s", "Error")
	tt.Error(err)
	tt.Equal("Error Error", err.Error())
	tt.Len(err.(*errors.TracedError).Stack, 1)
	tt.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newf")
}

func TestErrors_Newc(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.Newc(400, "User Error")
	tt.Error(err)
	tt.Equal("User Error", err.Error())
	tt.Equal(400, err.(*errors.TracedError).StatusCode)
	tt.Equal(400, errors.StatusCode(err))
	tt.Len(err.(*errors.TracedError).Stack, 1)
	tt.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newc")
}

func TestErrors_Newcf(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.Newcf(400, "User %s", "Error")
	tt.Error(err)
	tt.Equal("User Error", err.Error())
	tt.Equal(400, err.(*errors.TracedError).StatusCode)
	tt.Equal(400, errors.StatusCode(err))
	tt.Len(err.(*errors.TracedError).Stack, 1)
	tt.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newcf")
}

func TestErrors_NewPlusTrace(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.New("error occurred", 409, "key", "value")
	err = errors.Trace(err)
	tracedErr := errors.Convert(err)
	tt.Equal("error occurred", tracedErr.Error())
	tt.Equal(409, tracedErr.StatusCode)
	tt.Equal("value", tracedErr.Properties["key"])
}

func TestErrors_New(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.New("static string")
	tracedErr := errors.Convert(err)
	tt.Error(err)
	tt.Equal("static string", err.Error())
	tt.Equal(500, tracedErr.StatusCode)
	tt.Len(tracedErr.Properties, 0)
	tt.Len(tracedErr.Stack, 1)
	tt.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("format string %s %d", "XYZ", 123)
	tracedErr = errors.Convert(err)
	tt.Error(err)
	tt.Equal("format string XYZ 123", err.Error())
	tt.Equal(500, tracedErr.StatusCode)
	tt.Len(tracedErr.Properties, 0)
	tt.Len(tracedErr.Stack, 1)
	tt.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("format string %s %d", "XYZ", 123,
		"strKey", "ABC",
		"intKey", 888,
	)
	tracedErr = errors.Convert(err)
	tt.Error(err)
	tt.Equal("format string XYZ 123", err.Error())
	tt.Equal(500, tracedErr.StatusCode)
	tt.Len(tracedErr.Properties, 2)
	tt.Equal("ABC", tracedErr.Properties["strKey"])
	tt.Equal(888, tracedErr.Properties["intKey"])
	tt.Len(tracedErr.Stack, 1)
	tt.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	badDateStr := "2025-06-07T25:06:07Z"
	_, originalErr := time.Parse(time.RFC3339, badDateStr)
	err = errors.New("failed to parse '%s'", badDateStr, originalErr, 400)
	tracedErr = errors.Convert(err)
	tt.Error(err)
	tt.Equal("failed to parse '"+badDateStr+"': "+originalErr.Error(), err.Error())
	tt.Equal(400, tracedErr.StatusCode)
	tt.Len(tracedErr.Properties, 0)
	tt.Len(tracedErr.Stack, 1)
	tt.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("", originalErr, 400)
	tt.Error(err)
	tt.Equal(originalErr.Error(), err.Error())

	err = errors.New("", 400, originalErr)
	tt.Error(err)
	tt.Equal("bad request: "+originalErr.Error(), err.Error())

	err = errors.New("", originalErr)
	tt.Error(err)
	tt.Equal(originalErr.Error(), err.Error())

	err = errors.New("failed to parse date: %w", originalErr)
	tt.Error(err)
	tt.Equal("failed to parse date: "+originalErr.Error(), err.Error())

	err = errors.New("")
	tt.Error(err)
	tt.NotEqual("", err.Error())

	err = errors.New("message", 5, 6, 7)
	tt.Error(err)
	tracedErr = errors.Convert(err)
	tt.Equal(7, tracedErr.StatusCode)

	// Unnamed property
	err = errors.New("message", false, "dur", time.Second)
	tt.Error(err)
	tracedErr = errors.Convert(err)
	tt.Len(tracedErr.Properties, 2)
	tt.Equal(false, tracedErr.Properties["!BADKEY"])

	// Not enough args for pattern
	err = errors.New("pattern %s %d", "XYZ")
	tt.Error(err)
	tt.Contains(err, "pattern XYZ")

	// Double percent sign
	err = errors.New("pattern %s 100%%d", "XYZ", 400)
	tt.Error(err)
	tt.Equal("pattern XYZ 100%d", err.Error())
	tracedErr = errors.Convert(err)
	tt.Equal(400, tracedErr.StatusCode)
}

func TestErrors_Trace(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("standard error")
	tt.Error(stdErr)

	err := errors.Trace(stdErr, http.StatusForbidden, "0123456789abcdef0123456789abcdef", "foo", "bar")
	tt.Error(err)
	tt.Len(err.(*errors.TracedError).Stack, 1)
	tt.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Trace")
	tt.Equal(http.StatusForbidden, err.(*errors.TracedError).StatusCode)
	tt.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	tt.Equal("bar", err.(*errors.TracedError).Properties["foo"])

	err = errors.Trace(err, http.StatusNotImplemented, "moo", "baz")
	tt.Len(err.(*errors.TracedError).Stack, 2)
	tt.NotEqual("", err.(*errors.TracedError).String())
	tt.Equal(http.StatusNotImplemented, err.(*errors.TracedError).StatusCode)
	tt.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	tt.Equal("bar", err.(*errors.TracedError).Properties["foo"])
	tt.Equal("baz", err.(*errors.TracedError).Properties["moo"])

	err = errors.Trace(err)
	tt.Len(err.(*errors.TracedError).Stack, 3)
	tt.NotEqual("", err.(*errors.TracedError).String())
	tt.Equal(http.StatusNotImplemented, err.(*errors.TracedError).StatusCode)
	tt.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	tt.Equal("bar", err.(*errors.TracedError).Properties["foo"])
	tt.Equal("baz", err.(*errors.TracedError).Properties["moo"])

	stdErr = errors.Trace(nil)
	tt.NoError(stdErr)
	tt.Nil(stdErr)
}

func TestErrors_Convert(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("other standard error")
	tt.Error(stdErr)

	err := errors.Convert(stdErr)
	tt.Error(err)
	tt.Len(err.Stack, 0)

	stdErr = errors.Trace(err)
	err = errors.Convert(stdErr)
	tt.Error(err)
	tt.Len(err.Stack, 1)

	err = errors.Convert(nil)
	tt.Nil(err)
}

func TestErrors_JSON(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.New("error!")

	b, jsonErr := err.(*errors.TracedError).MarshalJSON()
	tt.NoError(jsonErr)

	var unmarshal errors.TracedError
	jsonErr = unmarshal.UnmarshalJSON(b)
	tt.NoError(jsonErr)

	tt.Equal(err.Error(), unmarshal.Error())
	tt.Equal(err.(*errors.TracedError).String(), unmarshal.String())
}

func TestErrors_Format(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.New("my error")

	s := fmt.Sprintf("%s", err)
	tt.Equal("my error", s)

	v := fmt.Sprintf("%v", err)
	tt.Equal("my error", v)

	vPlus := fmt.Sprintf("%+v", err)
	tt.Equal(err.(*errors.TracedError).String(), vPlus)
	tt.Contains(vPlus, "errors_test.TestErrors_Format")
	tt.Contains(vPlus, "errors/errors_test.go:")

	vSharp := fmt.Sprintf("%#v", err)
	tt.Equal(err.(*errors.TracedError).String(), vSharp)
	tt.Contains(vSharp, "errors_test.TestErrors_Format")
	tt.Contains(vSharp, "errors/errors_test.go:")
}

func TestErrors_Is(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.Trace(os.ErrNotExist)
	tt.True(errors.Is(err, os.ErrNotExist))
}

func TestErrors_Join(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	e1 := stderrors.New("E1")
	e2 := errors.New("E2", 400)
	e3 := errors.New("E3")
	e3 = errors.Trace(e3)
	e4a := stderrors.New("E4a")
	e4b := stderrors.New("E4b")
	e4 := errors.Join(e4a, e4b)
	j := errors.Join(e1, e2, nil, e3, e4)
	tt.True(errors.Is(j, e1))
	tt.True(errors.Is(j, e2))
	tt.True(errors.Is(j, e3))
	tt.True(errors.Is(j, e4))
	tt.True(errors.Is(j, e4a))
	tt.True(errors.Is(j, e4b))
	jj, ok := j.(*errors.TracedError)
	if tt.True(ok) {
		tt.Len(jj.Stack, 1)
		tt.Equal(500, jj.StatusCode)
	}

	tt.Nil(errors.Join(nil, nil))
	tt.Equal(e3, errors.Join(e3, nil))
}

func TestErrors_String(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := errors.New("oops!", 400, "key", "value")
	err = errors.Trace(err)
	s := err.(*errors.TracedError).String()
	tt.Contains(s, "oops!")
	tt.Contains(s, "400")
	tt.Contains(s, "/fabric/errors/errors_test.go:")
	tt.Contains(s, "key")
	tt.Contains(s, "value")
	firstDash := strings.Index(s, "-")
	tt.True(firstDash > 0)
	secondDash := strings.Index(s[firstDash+1:], "-")
	tt.True(secondDash > 0)
}

func TestErrors_Unwrap(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("oops")
	err := errors.Trace(stdErr)
	tt.Equal(stdErr, errors.Unwrap(err))

	err = errors.New("", stdErr)
	tt.Equal(stdErr, errors.Unwrap(err))

	err = errors.New("failed: %w", stdErr)
	tt.Equal(stdErr, errors.Unwrap(errors.Unwrap(err)))
	tt.True(errors.Is(err, stdErr))

	inlineErr := stderrors.New("inline")
	arg1Err := stderrors.New("arg1")
	arg2Err := stderrors.New("arg2")
	err = errors.New("failed: %w", inlineErr, arg1Err, "id", 123, arg2Err)
	tt.Equal("failed: inline: arg1: arg2", err.Error())
	tt.True(errors.Is(err, inlineErr))
	tt.True(errors.Is(err, arg1Err))
	tt.True(errors.Is(err, arg2Err))
}

func TestErrors_TraceFull(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("Oops")
	err := errors.Trace(stdErr)
	errFull := errors.TraceFull(stdErr, 0)

	tracedErr := errors.Convert(err)
	tt.Len(tracedErr.Stack, 1)

	tt.Len(errFull.(*errors.TracedError).Stack, 2)
	tt.Equal("errors_test.TestErrors_TraceFull", errFull.(*errors.TracedError).Stack[0].Function)
	tt.Equal("testing.tRunner", errFull.(*errors.TracedError).Stack[1].Function)
}

func TestErrors_TraceUp(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := stderrors.New("hello")
	err0 := errors.TraceUp(err, 0)
	tt.Equal("errors_test.TestErrors_TraceUp", errors.Convert(err0).Stack[0].Function)
	err1 := errors.TraceUp(err, 1)
	tt.Equal("testing.tRunner", errors.Convert(err1).Stack[0].Function)

	tt.Nil(errors.TraceUp(nil, 0))
	tt.Nil(errors.TraceUp(nil, 1))
}

func TestErrors_CatchPanic(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// String
	err := errors.CatchPanic(func() error {
		panic("message")
	})
	tt.Error(err)
	tt.Equal("message", err.Error())

	// Error
	err = errors.CatchPanic(func() error {
		panic(errors.New("panic"))
	})
	tt.Error(err)
	tt.Equal("panic", err.Error())

	// Number
	err = errors.CatchPanic(func() error {
		panic(5)
	})
	tt.Error(err)
	tt.Equal("5", err.Error())

	// Division by zero
	err = errors.CatchPanic(func() error {
		j := 1
		j--
		i := 5 / j
		i++
		return nil
	})
	tt.Error(err)
	tt.Equal("runtime error: integer divide by zero", err.Error())

	// Nil map
	err = errors.CatchPanic(func() error {
		x := map[int]int{}
		if true {
			x = nil
		}
		x[5] = 6
		return nil
	})
	tt.Error(err)
	tt.Equal("assignment to entry in nil map", err.Error())

	// Standard error
	err = errors.CatchPanic(func() error {
		return errors.New("standard")
	})
	tt.Error(err)
	tt.Equal("standard", err.Error())
}
