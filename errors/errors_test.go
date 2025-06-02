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

package errors

import (
	stderrors "errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestErrors_RuntimeTrace(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	file, function, line1, _ := RuntimeTrace(0)
	_, _, line2, _ := RuntimeTrace(0)
	tt.Contains(file, "errors_test.go")
	tt.Equal("errors.TestErrors_RuntimeTrace", function)
	tt.Equal(line1+1, line2)
}

func TestErrors_New(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	tracedErr := New("This is a new error!")
	tt.Error(tracedErr)
	tt.Equal("This is a new error!", tracedErr.Error())
	tt.Len(tracedErr.(*TracedError).stack, 1)
	tt.Contains(tracedErr.(*TracedError).stack[0].Function, "TestErrors_New")
}

func TestErrors_Newf(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := Newf("Error %s", "Error")
	tt.Error(err)
	tt.Equal("Error Error", err.Error())
	tt.Len(err.(*TracedError).stack, 1)
	tt.Contains(err.(*TracedError).stack[0].Function, "TestErrors_Newf")
}

func TestErrors_Newc(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := Newc(400, "User Error")
	tt.Error(err)
	tt.Equal("User Error", err.Error())
	tt.Equal(400, err.(*TracedError).StatusCode)
	tt.Equal(400, StatusCode(err))
	tt.Len(err.(*TracedError).stack, 1)
	tt.Contains(err.(*TracedError).stack[0].Function, "TestErrors_Newc")
}

func TestErrors_Newcf(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := Newcf(400, "User %s", "Error")
	tt.Error(err)
	tt.Equal("User Error", err.Error())
	tt.Equal(400, err.(*TracedError).StatusCode)
	tt.Equal(400, StatusCode(err))
	tt.Len(err.(*TracedError).stack, 1)
	tt.Contains(err.(*TracedError).stack[0].Function, "TestErrors_Newcf")
}

func TestErrors_Trace(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("Standard Error")
	tt.Error(stdErr)

	err := Trace(stdErr)
	tt.Error(err)
	tt.Len(err.(*TracedError).stack, 1)
	tt.Contains(err.(*TracedError).stack[0].Function, "TestErrors_Trace")

	err = Trace(err)
	tt.Len(err.(*TracedError).stack, 2)
	tt.NotEqual("", err.(*TracedError).String())

	err = Trace(err)
	tt.Len(err.(*TracedError).stack, 3)
	tt.NotEqual("", err.(*TracedError).String())

	stdErr = Trace(nil)
	tt.NoError(stdErr)
	tt.Nil(stdErr)
}

func TestErrors_Convert(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("Other standard error")
	tt.Error(stdErr)

	err := Convert(stdErr)
	tt.Error(err)
	tt.Len(err.stack, 0)

	stdErr = Trace(err)
	err = Convert(stdErr)
	tt.Error(err)
	tt.Len(err.stack, 1)

	err = Convert(nil)
	tt.Nil(err)
}

func TestErrors_JSON(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := New("Error!")

	b, jsonErr := err.(*TracedError).MarshalJSON()
	tt.NoError(jsonErr)

	var unmarshal TracedError
	jsonErr = unmarshal.UnmarshalJSON(b)
	tt.NoError(jsonErr)

	tt.Equal(err.Error(), unmarshal.Error())
	tt.Equal(err.(*TracedError).String(), unmarshal.String())
}

func TestErrors_Format(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := New("my error")

	s := fmt.Sprintf("%s", err)
	tt.Equal("my error", s)

	v := fmt.Sprintf("%v", err)
	tt.Equal("my error", v)

	vPlus := fmt.Sprintf("%+v", err)
	tt.Equal(err.(*TracedError).String(), vPlus)
	tt.Contains(vPlus, "errors.TestErrors_Format")
	tt.Contains(vPlus, "errors/errors_test.go:")

	vSharp := fmt.Sprintf("%#v", err)
	tt.Equal(err.(*TracedError).String(), vSharp)
	tt.Contains(vSharp, "errors.TestErrors_Format")
	tt.Contains(vSharp, "errors/errors_test.go:")
}

func TestErrors_Is(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := Trace(os.ErrNotExist)
	tt.True(Is(err, os.ErrNotExist))
}

func TestErrors_Join(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	e1 := stderrors.New("E1")
	e2 := Newc(400, "E2")
	e3 := New("E3")
	e3 = Trace(e3)
	e4a := stderrors.New("E4a")
	e4b := stderrors.New("E4b")
	e4 := Join(e4a, e4b)
	j := Join(e1, e2, nil, e3, e4)
	tt.True(Is(j, e1))
	tt.True(Is(j, e2))
	tt.True(Is(j, e3))
	tt.True(Is(j, e4))
	tt.True(Is(j, e4a))
	tt.True(Is(j, e4b))
	jj, ok := j.(*TracedError)
	if tt.True(ok) {
		tt.Len(jj.stack, 1)
		tt.Equal(500, jj.StatusCode)
	}

	tt.Nil(Join(nil, nil))
	tt.Equal(e3, Join(e3, nil))
}

func TestErrors_String(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	err := Newc(400, "Oops!")
	err = Trace(err)
	s := err.(*TracedError).String()
	tt.Contains(s, "Oops!")
	tt.Contains(s, "[400]")
	tt.Contains(s, "/fabric/errors/errors_test.go:")
	firstDash := strings.Index(s, "-")
	tt.True(firstDash > 0)
	secondDash := strings.Index(s[firstDash+1:], "-")
	tt.True(secondDash > 0)
}

func TestErrors_Unwrap(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("Oops")
	err := Trace(stdErr)
	tt.Equal(stdErr, Unwrap(err))
}

func TestErrors_TraceFull(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	stdErr := stderrors.New("Oops")
	err := Trace(stdErr)
	errUp0 := TraceUp(stdErr, 0)
	errUp1 := TraceUp(stdErr, 1)
	errFull := TraceFull(stdErr, 0)

	tt.Len(err.(*TracedError).stack, 1)

	tt.Len(errUp0.(*TracedError).stack, 1)
	tt.Len(errUp1.(*TracedError).stack, 1)
	tt.NotEqual(errUp0.(*TracedError).stack[0].Function, errUp1.(*TracedError).stack[0].Function)

	tt.True(len(errFull.(*TracedError).stack) > 1)
	tt.Equal(errUp0.(*TracedError).stack[0].Function, errFull.(*TracedError).stack[0].Function)
	tt.Equal(errUp1.(*TracedError).stack[0].Function, errFull.(*TracedError).stack[1].Function)
}

func TestErrors_CatchPanic(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// String
	err := CatchPanic(func() error {
		panic("message")
	})
	tt.Error(err)
	tt.Equal("message", err.Error())

	// Error
	err = CatchPanic(func() error {
		panic(New("panic"))
	})
	tt.Error(err)
	tt.Equal("panic", err.Error())

	// Number
	err = CatchPanic(func() error {
		panic(5)
	})
	tt.Error(err)
	tt.Equal("5", err.Error())

	// Division by zero
	err = CatchPanic(func() error {
		j := 1
		j--
		i := 5 / j
		i++
		return nil
	})
	tt.Error(err)
	tt.Equal("runtime error: integer divide by zero", err.Error())

	// Nil map
	err = CatchPanic(func() error {
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
	err = CatchPanic(func() error {
		return New("standard")
	})
	tt.Error(err)
	tt.Equal("standard", err.Error())
}
