/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
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
	// 2xx
	200: "ok",
	206: "partial content",
	// 3xx
	301: "moved permanently",
	302: "found",
	304: "not modified",
	307: "temporary redirect",
	308: "permanent redirect",
	// 4xx
	400: "bad request",
	401: "unauthorized",
	403: "forbidden",
	404: "not found",
	405: "method not allowed",
	408: "request timeout",
	413: "payload too large",
	// 5xx
	500: "internal server error",
	501: "not implemented",
	503: "service unavailable",
	508: "loop detected",
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
	if n == 0 {
		return nil
	}
	if n == 1 {
		return TraceUp(err, 1)
	}
	return TraceUp(stderrors.Join(errs...), 1)
}

// Unwrap delegates to the standard Go's errors.Wrap function.
func Unwrap(err error) error {
	return stderrors.Unwrap(err)
}

// New creates a new error, capturing the current stack location.
func New(text string) error {
	return TraceUp(stderrors.New(text), 1)
}

// Newc creates a new error with an HTTP status code, capturing the current stack location.
func Newc(statusCode int, text string) error {
	if text == "" {
		text = statusText[statusCode]
	}
	err := TraceUp(stderrors.New(text), 1)
	err.(*TracedError).StatusCode = statusCode
	return err
}

// Newcf creates a new formatted error with an HTTP status code, capturing the current stack location.
func Newcf(statusCode int, format string, a ...any) error {
	if format == "" {
		format = statusText[statusCode]
	}
	err := TraceUp(fmt.Errorf(format, a...), 1)
	err.(*TracedError).StatusCode = statusCode
	return err
}

// Newf formats a new error, capturing the current stack location.
func Newf(format string, a ...any) error {
	return TraceUp(fmt.Errorf(format, a...), 1)
}

// Trace appends the current stack location to the error's stack trace.
func Trace(err error) error {
	return TraceUp(err, 1)
}

// TraceUp appends the level above the current stack location to the error's stack trace.
// Level 0 captures the location of the caller.
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
		tracedErr.stack = append(tracedErr.stack, &trace{
			File:     file,
			Function: function,
			Line:     line,
		})
	}
	return tracedErr
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
		if strings.HasPrefix(function, "runtime.") {
			continue
		}
		if function == "utils.CatchPanic" {
			break
		}
		tracedErr.stack = append(tracedErr.stack, &trace{
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
		return tracedErr
	}
	return &TracedError{
		error:      err,
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

// BadRequest returns a new 400 error.
func BadRequest() error {
	return Newc(400, "")
}

// Unauthorized returns a new 401 error.
func Unauthorized() error {
	return Newc(401, "")
}

// Forbidden returns a new 403 error.
func Forbidden() error {
	return Newc(403, "")
}

// NotFound returns a new 404 error.
func NotFound() error {
	return Newc(404, "")
}

// NotFound returns a new 408 error.
func RequestTimeout() error {
	return Newc(408, "")
}

// NotImplemented returns a new 501 error.
func NotImplemented() error {
	return Newc(501, "")
}
