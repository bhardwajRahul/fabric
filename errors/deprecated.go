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

// Deprecated: This package is superseded by github.com/microbus-io/errors.
package errors

import (
	"github.com/microbus-io/errors"
)

// Deprecated: This package is superseded by github.com/microbus-io/errors.
type TracedError = errors.TracedError

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func New(pattern string, args ...any) error {
	return errors.New(pattern, args...)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Newc(statusCode int, text string) error {
	return New(text, statusCode)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Newcf(statusCode int, format string, a ...any) error {
	return New(format, append(a, statusCode)...)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Newf(format string, a ...any) error {
	return New(format, a...)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
type StreamedError = errors.StreamedError

// Deprecated: This package is superseded by github.com/microbus-io/errors.
type StackFrame = errors.StackFrame

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Trace(err error, a ...any) error {
	return errors.Trace(err, a...)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Tracec(statusCode int, err error) error {
	return errors.Trace(err, statusCode)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func Convert(err error) *TracedError {
	return errors.Convert(err)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func StatusCode(err error) int {
	return errors.StatusCode(err)
}

// Deprecated: This package is superseded by github.com/microbus-io/errors.
func CatchPanic(f func() error) (err error) {
	return errors.CatchPanic(f)
}
