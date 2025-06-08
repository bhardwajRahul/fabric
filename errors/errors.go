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
	"runtime"
	"strings"
)

var statusText = map[int]string{
	// 1xx
	100: "continue",
	101: "switching protocol",
	102: "processing",
	103: "early hints",
	// 2xx
	200: "ok",
	201: "created",
	202: "accepted",
	203: "non-authoritative information",
	204: "no content",
	205: "reset content",
	206: "partial content",
	// 3xx
	300: "multiple choices",
	301: "moved permanently",
	302: "found",
	303: "see other",
	304: "not modified",
	307: "temporary redirect",
	308: "permanent redirect",
	// 4xx
	400: "bad request",
	401: "unauthorized",
	402: "payment required",
	403: "forbidden",
	404: "not found",
	405: "method not allowed",
	406: "not acceptable",
	407: "proxy authentication required",
	408: "request timeout",
	409: "conflict",
	410: "gone",
	411: "length required",
	412: "preconditions failed",
	413: "content too large",
	414: "uri too long",
	415: "unsupported media type",
	416: "range not satisfiable",
	417: "expectation failed",
	418: "i'm a teapot",
	421: "misdirected request",
	422: "unprocessable content",
	423: "locked",
	424: "failed dependency",
	425: "too early",
	426: "upgrade required",
	428: "precondition required",
	429: "too many requests",
	431: "request header fields too large",
	451: "unavailable for legal reasons",
	// 5xx
	500: "internal server error",
	501: "not implemented",
	502: "bad gateway",
	503: "service unavailable",
	504: "gateway timeout",
	505: "http version not supported",
	506: "variant also negotiates",
	507: "insufficient storage",
	508: "loop detected",
	510: "not extended",
	511: "network authentication required",
}

// As delegates to the standard Go's errors.As function.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// Is delegates to the standard Go's errors.Is function.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// Join aggregates multiple errors into one.
// The stack traces of the original errors are discarded and a new stack trace is captured.
func Join(errs ...error) error {
	var err error
	var n int
	for _, e := range errs {
		if e != nil {
			err = e
			n++
		}
	}
	switch n {
	case 0:
		return nil
	case 1:
		return TraceCaller(err)
	default:
		return TraceCaller(stderrors.Join(errs...))
	}
}

// Unwrap delegates to the standard Go's errors.Wrap function.
func Unwrap(err error) error {
	return stderrors.Unwrap(err)
}

// Tracec appends the current stack location to the error's stack trace and sets the status code.
//
// Deprecated: Use Trace
func Tracec(statusCode int, err error) error {
	if err == nil {
		return nil
	}
	return New("", []any{err, statusCode}...)
}

// Trace appends the current stack location to the error's stack trace.
// The variadic arguments behave like those of New.
func Trace(err error, a ...any) error {
	if err == nil {
		return nil
	}
	return New("", append([]any{err}, a...)...)
}

// TraceUp appends the level above the current stack location to the error's stack trace.
// Level 0 captures the location of the caller.
//
// Deprecated: Use TraceCaller
func TraceUp(err error, level int) error {
	if err == nil {
		return nil
	}
	if level < 0 {
		level = 0
	}
	tracedErr := Convert(err)
	file, function, line, ok := RuntimeTrace(1 + level)
	if ok {
		tracedErr.Stack = append(tracedErr.Stack, &StackFrame{
			File:     file,
			Function: function,
			Line:     line,
		})
	}
	return tracedErr
}

// TraceCaller appends the stack location of the caller to the error's stack trace.
func TraceCaller(err error) error {
	if err == nil {
		return nil
	}
	level := 1
	tracedErr := Convert(err)
	for {
		file, function, line, ok := RuntimeTrace(level)
		if !ok {
			return tracedErr
		}
		if strings.HasPrefix(function, "runtime.") || strings.HasPrefix(function, "errors.") {
			level++
			continue
		}
		tracedErr.Stack = append(tracedErr.Stack, &StackFrame{
			File:     file,
			Function: function,
			Line:     line,
		})
		return tracedErr
	}
}

// TraceFull appends the full stack to the error's stack trace,
// starting at the indicated level.
// Level 0 captures the location of the caller.
func TraceFull(err error, level int) error {
	if err == nil {
		return nil
	}
	if level < 0 {
		level = 0
	}
	tracedErr := Convert(err)

	levels := level - 1
	for {
		levels++
		file, function, line, ok := RuntimeTrace(1 + levels)
		if !ok {
			break
		}
		if function == "errors.CatchPanic" {
			break
		}
		if strings.HasPrefix(function, "runtime.") || strings.HasPrefix(function, "errors.") {
			continue
		}
		tracedErr.Stack = append(tracedErr.Stack, &StackFrame{
			File:     file,
			Function: function,
			Line:     line,
		})
	}
	return tracedErr
}

// Convert converts an error to one that supports stack tracing.
// If the error already supports this, it is returned as it is.
// Note: Trace should be called to include the error's trace in the stack.
func Convert(err error) *TracedError {
	if err == nil {
		return nil
	}
	if tracedErr, ok := err.(*TracedError); ok {
		if tracedErr.StatusCode == 0 {
			tracedErr.StatusCode = 500
		}
		return tracedErr
	}
	return &TracedError{
		Err:        err,
		StatusCode: 500,
	}
}

// RuntimeTrace traces back by the amount of levels
// to retrieve the runtime information used for tracing.
func RuntimeTrace(levels int) (file string, function string, line int, ok bool) {
	pc, file, line, ok := runtime.Caller(levels + 1)
	if !ok {
		return "", "", 0, false
	}
	function = "?"
	runtimeFunc := runtime.FuncForPC(pc)
	if runtimeFunc != nil {
		function = runtimeFunc.Name()
		p := strings.LastIndex(function, "/")
		if p >= 0 {
			function = function[p+1:]
		}
	}
	return file, function, line, ok
}

// StatusCode returns the HTTP status code associated with an error.
// It is the equivalent of Convert(err).StatusCode.
// If not specified, the default status code is 500.
func StatusCode(err error) int {
	if err == nil {
		return 0
	}
	return Convert(err).StatusCode
}

// CatchPanic calls the given function and returns any panic as a standard error.
func CatchPanic(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
			err = TraceFull(err, 1)
		}
	}()
	err = f()
	return
}
