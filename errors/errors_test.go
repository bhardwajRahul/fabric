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
	assert := testarossa.For(t)

	file, function, line1, _ := errors.RuntimeTrace(0)
	_, _, line2, _ := errors.RuntimeTrace(0)
	assert.Contains(file, "errors_test.go")
	assert.Equal("errors_test.TestErrors_RuntimeTrace", function)
	assert.Equal(line1+1, line2)
}

func TestErrors_Newf(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.Newf("Error %s", "Error")
	assert.Error(err)
	assert.Equal("Error Error", err.Error())
	assert.Len(err.(*errors.TracedError).Stack, 1)
	assert.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newf")
}

func TestErrors_Newc(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.Newc(400, "User Error")
	assert.Error(err)
	assert.Equal("User Error", err.Error())
	assert.Equal(400, err.(*errors.TracedError).StatusCode)
	assert.Equal(400, errors.StatusCode(err))
	assert.Len(err.(*errors.TracedError).Stack, 1)
	assert.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newc")
}

func TestErrors_Newcf(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.Newcf(400, "User %s", "Error")
	assert.Error(err)
	assert.Equal("User Error", err.Error())
	assert.Equal(400, err.(*errors.TracedError).StatusCode)
	assert.Equal(400, errors.StatusCode(err))
	assert.Len(err.(*errors.TracedError).Stack, 1)
	assert.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Newcf")
}

func TestErrors_NewPlusTrace(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.New("error occurred", 409, "key", "value")
	err = errors.Trace(err)
	tracedErr := errors.Convert(err)
	assert.Equal("error occurred", tracedErr.Error())
	assert.Equal(409, tracedErr.StatusCode)
	assert.Equal("value", tracedErr.Properties["key"])
}

func TestErrors_New(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.New("static string")
	tracedErr := errors.Convert(err)
	assert.Error(err)
	assert.Equal("static string", err.Error())
	assert.Equal(500, tracedErr.StatusCode)
	assert.Len(tracedErr.Properties, 0)
	assert.Len(tracedErr.Stack, 1)
	assert.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("format string %s %d", "XYZ", 123)
	tracedErr = errors.Convert(err)
	assert.Error(err)
	assert.Equal("format string XYZ 123", err.Error())
	assert.Equal(500, tracedErr.StatusCode)
	assert.Len(tracedErr.Properties, 0)
	assert.Len(tracedErr.Stack, 1)
	assert.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("format string %s %d", "XYZ", 123,
		"strKey", "ABC",
		"intKey", 888,
	)
	tracedErr = errors.Convert(err)
	assert.Error(err)
	assert.Equal("format string XYZ 123", err.Error())
	assert.Equal(500, tracedErr.StatusCode)
	assert.Len(tracedErr.Properties, 2)
	assert.Equal("ABC", tracedErr.Properties["strKey"])
	assert.Equal(888, tracedErr.Properties["intKey"])
	assert.Len(tracedErr.Stack, 1)
	assert.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	badDateStr := "2025-06-07T25:06:07Z"
	_, originalErr := time.Parse(time.RFC3339, badDateStr)
	err = errors.New("failed to parse '%s'", badDateStr, originalErr, 400)
	tracedErr = errors.Convert(err)
	assert.Error(err)
	assert.Equal("failed to parse '"+badDateStr+"': "+originalErr.Error(), err.Error())
	assert.Equal(400, tracedErr.StatusCode)
	assert.Len(tracedErr.Properties, 0)
	assert.Len(tracedErr.Stack, 1)
	assert.Contains(tracedErr.Stack[0].Function, "TestErrors_New")

	err = errors.New("", originalErr, 400)
	assert.Error(err)
	assert.Equal(originalErr.Error(), err.Error())

	err = errors.New("", 400, originalErr)
	assert.Error(err)
	assert.Equal("bad request: "+originalErr.Error(), err.Error())

	err = errors.New("", originalErr)
	assert.Error(err)
	assert.Equal(originalErr.Error(), err.Error())

	err = errors.New("failed to parse date: %w", originalErr)
	assert.Error(err)
	assert.Equal("failed to parse date: "+originalErr.Error(), err.Error())

	err = errors.New("")
	assert.Error(err)
	assert.NotEqual("", err.Error())

	err = errors.New("message", 5, 6, 7)
	assert.Error(err)
	tracedErr = errors.Convert(err)
	assert.Equal(7, tracedErr.StatusCode)

	// Unnamed property
	err = errors.New("message", false, "dur", time.Second)
	assert.Error(err)
	tracedErr = errors.Convert(err)
	assert.Len(tracedErr.Properties, 2)
	assert.Equal(false, tracedErr.Properties["!BADKEY"])

	// Not enough args for pattern
	err = errors.New("pattern %s %d", "XYZ")
	assert.Error(err)
	assert.Contains(err, "pattern XYZ")

	// Double percent sign
	err = errors.New("pattern %s 100%%d", "XYZ", 400)
	assert.Error(err)
	assert.Equal("pattern XYZ 100%d", err.Error())
	tracedErr = errors.Convert(err)
	assert.Equal(400, tracedErr.StatusCode)
}

func TestErrors_Trace(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	stdErr := stderrors.New("standard error")
	assert.Error(stdErr)

	err := errors.Trace(stdErr, http.StatusForbidden, "0123456789abcdef0123456789abcdef", "foo", "bar")
	assert.Error(err)
	assert.Len(err.(*errors.TracedError).Stack, 1)
	assert.Contains(err.(*errors.TracedError).Stack[0].Function, "TestErrors_Trace")
	assert.Equal(http.StatusForbidden, err.(*errors.TracedError).StatusCode)
	assert.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	assert.Equal("bar", err.(*errors.TracedError).Properties["foo"])

	err = errors.Trace(err, http.StatusNotImplemented, "moo", "baz")
	assert.Len(err.(*errors.TracedError).Stack, 2)
	assert.NotEqual("", err.(*errors.TracedError).String())
	assert.Equal(http.StatusNotImplemented, err.(*errors.TracedError).StatusCode)
	assert.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	assert.Equal("bar", err.(*errors.TracedError).Properties["foo"])
	assert.Equal("baz", err.(*errors.TracedError).Properties["moo"])

	err = errors.Trace(err)
	assert.Len(err.(*errors.TracedError).Stack, 3)
	assert.NotEqual("", err.(*errors.TracedError).String())
	assert.Equal(http.StatusNotImplemented, err.(*errors.TracedError).StatusCode)
	assert.Equal("0123456789abcdef0123456789abcdef", err.(*errors.TracedError).Trace)
	assert.Equal("bar", err.(*errors.TracedError).Properties["foo"])
	assert.Equal("baz", err.(*errors.TracedError).Properties["moo"])

	stdErr = errors.Trace(nil)
	assert.NoError(stdErr)
	assert.Nil(stdErr)
}

func TestErrors_Convert(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	stdErr := stderrors.New("other standard error")
	assert.Error(stdErr)

	err := errors.Convert(stdErr)
	assert.Error(err)
	assert.Len(err.Stack, 0)

	stdErr = errors.Trace(err)
	err = errors.Convert(stdErr)
	assert.Error(err)
	assert.Len(err.Stack, 1)

	err = errors.Convert(nil)
	assert.Nil(err)
}

func TestErrors_JSON(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.New("error!")

	b, jsonErr := err.(*errors.TracedError).MarshalJSON()
	assert.NoError(jsonErr)

	var unmarshal errors.TracedError
	jsonErr = unmarshal.UnmarshalJSON(b)
	assert.NoError(jsonErr)

	assert.Equal(err.Error(), unmarshal.Error())
	assert.Equal(err.(*errors.TracedError).String(), unmarshal.String())
}

func TestErrors_Format(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.New("my error")

	s := fmt.Sprintf("%s", err)
	assert.Equal("my error", s)

	v := fmt.Sprintf("%v", err)
	assert.Equal("my error", v)

	vPlus := fmt.Sprintf("%+v", err)
	assert.Equal(err.(*errors.TracedError).String(), vPlus)
	assert.Contains(vPlus, "errors_test.TestErrors_Format")
	assert.Contains(vPlus, "errors/errors_test.go:")

	vSharp := fmt.Sprintf("%#v", err)
	assert.Equal(err.(*errors.TracedError).String(), vSharp)
	assert.Contains(vSharp, "errors_test.TestErrors_Format")
	assert.Contains(vSharp, "errors/errors_test.go:")
}

func TestErrors_Is(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.Trace(os.ErrNotExist)
	assert.True(errors.Is(err, os.ErrNotExist))
}

func TestErrors_Join(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	e1 := stderrors.New("E1")
	e2 := errors.New("E2", 400)
	e3 := errors.New("E3")
	e3 = errors.Trace(e3)
	e4a := stderrors.New("E4a")
	e4b := stderrors.New("E4b")
	e4 := errors.Join(e4a, e4b)
	j := errors.Join(e1, e2, nil, e3, e4)
	assert.True(errors.Is(j, e1))
	assert.True(errors.Is(j, e2))
	assert.True(errors.Is(j, e3))
	assert.True(errors.Is(j, e4))
	assert.True(errors.Is(j, e4a))
	assert.True(errors.Is(j, e4b))
	jj, ok := j.(*errors.TracedError)
	if assert.True(ok) {
		assert.Len(jj.Stack, 1)
		assert.Equal(500, jj.StatusCode)
	}

	assert.Nil(errors.Join(nil, nil))
	assert.Equal(e3, errors.Join(e3, nil))
}

func TestErrors_String(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := errors.New("oops!", 400, "key", "value")
	err = errors.Trace(err)
	s := err.(*errors.TracedError).String()
	assert.Contains(s, "oops!")
	assert.Contains(s, "400")
	assert.Contains(s, "/fabric/errors/errors_test.go:")
	assert.Contains(s, "key")
	assert.Contains(s, "value")
	firstDash := strings.Index(s, "-")
	assert.True(firstDash > 0)
	secondDash := strings.Index(s[firstDash+1:], "-")
	assert.True(secondDash > 0)
}

func TestErrors_Unwrap(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	stdErr := stderrors.New("oops")
	err := errors.Trace(stdErr)
	assert.Equal(stdErr, errors.Unwrap(err))

	err = errors.New("", stdErr)
	assert.Equal(stdErr, errors.Unwrap(err))

	err = errors.New("failed: %w", stdErr)
	assert.Equal(stdErr, errors.Unwrap(errors.Unwrap(err)))
	assert.True(errors.Is(err, stdErr))

	inlineErr := stderrors.New("inline")
	arg1Err := stderrors.New("arg1")
	arg2Err := stderrors.New("arg2")
	err = errors.New("failed: %w", inlineErr, arg1Err, "id", 123, arg2Err)
	assert.Equal("failed: inline: arg1: arg2", err.Error())
	assert.True(errors.Is(err, inlineErr))
	assert.True(errors.Is(err, arg1Err))
	assert.True(errors.Is(err, arg2Err))
}

func TestErrors_TraceFull(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	stdErr := stderrors.New("Oops")
	err := errors.Trace(stdErr)
	errFull := errors.TraceFull(stdErr, 0)

	tracedErr := errors.Convert(err)
	assert.Len(tracedErr.Stack, 1)

	assert.Len(errFull.(*errors.TracedError).Stack, 2)
	assert.Equal("errors_test.TestErrors_TraceFull", errFull.(*errors.TracedError).Stack[0].Function)
	assert.Equal("testing.tRunner", errFull.(*errors.TracedError).Stack[1].Function)
}

func TestErrors_TraceUp(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	err := stderrors.New("hello")
	err0 := errors.TraceUp(err, 0)
	assert.Equal("errors_test.TestErrors_TraceUp", errors.Convert(err0).Stack[0].Function)
	err1 := errors.TraceUp(err, 1)
	assert.Equal("testing.tRunner", errors.Convert(err1).Stack[0].Function)

	assert.Nil(errors.TraceUp(nil, 0))
	assert.Nil(errors.TraceUp(nil, 1))
}

func TestErrors_CatchPanic(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// String
	err := errors.CatchPanic(func() error {
		panic("message")
	})
	assert.Error(err)
	assert.Equal("message", err.Error())

	// Error
	err = errors.CatchPanic(func() error {
		panic(errors.New("panic"))
	})
	assert.Error(err)
	assert.Equal("panic", err.Error())

	// Number
	err = errors.CatchPanic(func() error {
		panic(5)
	})
	assert.Error(err)
	assert.Equal("5", err.Error())

	// Division by zero
	err = errors.CatchPanic(func() error {
		j := 1
		j--
		i := 5 / j
		i++
		return nil
	})
	assert.Error(err)
	assert.Equal("runtime error: integer divide by zero", err.Error())

	// Nil map
	err = errors.CatchPanic(func() error {
		x := map[int]int{}
		if true {
			x = nil
		}
		x[5] = 6
		return nil
	})
	assert.Error(err)
	assert.Equal("assignment to entry in nil map", err.Error())

	// Standard error
	err = errors.CatchPanic(func() error {
		return errors.New("standard")
	})
	assert.Error(err)
	assert.Equal("standard", err.Error())
}

func TestErrors_AnonymousProperties(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Errors
	base := stderrors.New("base")
	err := errors.New("failed", base)
	assert.Equal("failed: base", err.Error())
	assert.True(errors.Is(err, base))

	// Status code
	err = errors.New("failed", 409)
	assert.Equal(409, errors.Convert(err).StatusCode)

	// Trace ID
	err = errors.New("failed", "0123456789abcdef0123456789abcdef")
	assert.Equal("0123456789abcdef0123456789abcdef", errors.Convert(err).Trace)
}
