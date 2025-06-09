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
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"strings"
)

// Ensure interfaces
var (
	_ = error(&TracedError{})
	_ = fmt.Stringer(&TracedError{})
	_ = fmt.Formatter(&TracedError{})
	_ = json.Marshaler(&TracedError{})
	_ = json.Unmarshaler(&TracedError{})
)

// TracedError is a standard Go error augmented with a stack trace, status code and property bag.
type TracedError struct {
	Err        error
	Stack      []*StackFrame
	StatusCode int
	Properties map[string]any
}

/*
New creates a new error, with a static or formatted message, optionally wrapping another error, attaching a status code or attaching properties.

In the simplest case, the pattern is a static string.

	New("network timeout")

If the pattern contains % signs, the next appropriate number of arguments are used to format the message of the new error.

	New("failed to parse '%s' for user %d", dateStr, userID)

Any additional arguments are treated like slog name=value pairs and added to the error's property bag.
Properties are not part of the error's message and can be retrieved up the call stack in a structured way.

	New("failed to execute '%s'", cmd,
		"exitCode", exitCode,
		"os", os,
	)

Two notable properties do not require a name: errors and integers.

	New("failed to parse form",
		err,
		http.StatusBadRequest,
		"path", r.URL.Path,
	)

An unnamed error is interpreted to be the original source of the error. The new error is created to wrap the original error.

	fmt.Errorf(errorMessage+": %w", originalError)

An unnamed integer is interpreted to be an HTTP status code to associate with the error. If the pattern is empty, the status text is set by default.

	New("user not found",
		http.StatusNotFound,
		"id", userID,
	)
*/
func New(pattern string, a ...any) error {
	pctArgs := strings.Count(pattern, `%`) - 2*strings.Count(pattern, `%%`)
	pctArgs = min(pctArgs, len(a))
	msg := fmt.Sprintf(pattern, a[:pctArgs]...)
	err := &TracedError{}
	i := pctArgs
	unnamed := 0
	for i < len(a) {
		if err.Properties == nil {
			err.Properties = make(map[string]any, len(a)-pctArgs)
		}
		switch k := a[i].(type) {
		case int:
			err.StatusCode = k
			if msg == "" {
				msg = statusText[k]
			}
			i++
		case error:
			if msg == "" {
				err.Err = k
			} else {
				err.Err = fmt.Errorf(msg+": %w", k)
			}
			if tracedErr, ok := k.(*TracedError); ok {
				if err.StatusCode == 0 {
					err.StatusCode = tracedErr.StatusCode
				}
				for k, v := range tracedErr.Properties {
					err.Properties[k] = v
				}
				err.Stack = tracedErr.Stack
			}
			i++
		case string:
			if i < len(a)-1 {
				err.Properties[k] = a[i+1]
				i += 2
			} else {
				err.Properties[k] = ""
				i++
			}
		default:
			unnamed++
			err.Properties[fmt.Sprintf("unnamed%d", unnamed)] = k
			i++
		}
	}
	if err.Err == nil {
		if msg == "" {
			msg = "unspecified error"
		}
		err.Err = stderrors.New(msg)
	}
	if err.StatusCode == 0 {
		err.StatusCode = 500
	}
	return TraceCaller(err)
}

// Newc creates a new error with an HTTP status code, capturing the current stack location.
//
// Deprecated: Use New
func Newc(statusCode int, text string) error {
	if text == "" {
		text = statusText[statusCode]
	}
	err := &TracedError{
		Err:        stderrors.New(text),
		StatusCode: statusCode,
	}
	return TraceCaller(err)
}

// Newcf creates a new formatted error with an HTTP status code, capturing the current stack location.
//
// Deprecated: Use New
func Newcf(statusCode int, format string, a ...any) error {
	if format == "" {
		format = statusText[statusCode]
	}
	err := &TracedError{
		Err:        fmt.Errorf(format, a...),
		StatusCode: statusCode,
	}
	return TraceCaller(err)
}

// Newf formats a new error, capturing the current stack location.
//
// Deprecated: Use New
func Newf(format string, a ...any) error {
	err := &TracedError{
		Err:        fmt.Errorf(format, a...),
		StatusCode: 500,
	}
	return TraceCaller(err)
}

// Error returns the error string.
func (e *TracedError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *TracedError) Unwrap() error {
	return e.Err
}

// String returns a human-friendly representation of the traced error.
func (e *TracedError) String() string {
	var b strings.Builder
	b.WriteString(e.Error())
	if e.StatusCode != 0 && e.StatusCode != 500 {
		b.WriteString(fmt.Sprintf("\n[%d]", e.StatusCode))
	}
	if len(e.Properties) > 0 {
		b.WriteString("\n")
	}
	for k, v := range e.Properties {
		b.WriteString("\n")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(fmt.Sprintf("%v", v))
	}
	if len(e.Stack) > 0 {
		b.WriteString("\n")
	}
	for _, stackFrame := range e.Stack {
		b.WriteString("\n")
		b.WriteString(stackFrame.String())
	}
	return b.String()
}

// StreamedError is the schema used to marshal and unmarshal the traced error.
type StreamedError struct {
	Error      string         `json:"error" jsonschema:"example=message"`
	StatusCode int            `json:"statusCode,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Stack      []*StackFrame  `json:"stack,omitempty"`
}

// MarshalJSON marshals the error to JSON.
func (e *TracedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(&StreamedError{
		Error:      e.Err.Error(),
		Stack:      e.Stack,
		StatusCode: e.StatusCode,
		Properties: e.Properties,
	})
}

// UnmarshalJSON unmarshals the error from JSON.
func (e *TracedError) UnmarshalJSON(data []byte) error {
	var j StreamedError
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	e.Err = stderrors.New(j.Error) // Type is lost
	e.Stack = j.Stack
	e.StatusCode = j.StatusCode
	e.Properties = j.Properties
	return nil
}

// Format the error based on the verb and flag.
func (e *TracedError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') || s.Flag('#') {
			io.WriteString(s, e.String())
		} else {
			io.WriteString(s, e.Error())
		}
	case 's':
		io.WriteString(s, e.Error())
	}
}
