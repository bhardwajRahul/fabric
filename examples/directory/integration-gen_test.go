/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

package directory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"

	"github.com/microbus-io/fabric/examples/directory/directoryapi"
)

var (
	_ bytes.Buffer
	_ context.Context
	_ fmt.Stringer
	_ io.Reader
	_ *http.Request
	_ *url.URL
	_ os.File
	_ time.Time
	_ strings.Builder
	_ cascadia.Sel
	_ *connector.Connector
	_ *errors.TracedError
	_ frame.Frame
	_ *httpx.BodyReader
	_ pub.Option
	_ rand.Void
	_ utils.InfiniteChan[int]
	_ assert.TestingT
	_ *html.Node
	_ *directoryapi.Client
)

var (
	// App manages the lifecycle of the microservices used in the test
	App *application.Application
	// Svc is the directory.example microservice being tested
	Svc *Service
)

func TestMain(m *testing.M) {
	var code int

	// Initialize the application
	err := func() error {
		var err error
		App = application.NewTesting()
		Svc = NewService()
		err = Initialize()
		if err != nil {
			return err
		}
		err = App.Startup()
		if err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "--- FAIL: %+v\n", err)
		code = 19
	}

	// Run the tests
	if err == nil {
		code = m.Run()
	}

	// Terminate the app
	err = func() error {
		var err error
		var lastErr error
		err = Terminate()
		if err != nil {
			lastErr = err
		}
		err = App.Shutdown()
		if err != nil {
			lastErr = err
		}
		return lastErr
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "--- FAIL: %+v\n", err)
	}

	os.Exit(code)
}

// Context creates a new context for a test.
func Context(t *testing.T) context.Context {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		t.Cleanup(cancel)
	}
	ctx = frame.ContextWithFrame(ctx)
	return ctx
}

// CreateTestCase assists in asserting against the results of executing Create.
type CreateTestCase struct {
	_t *testing.T
	_dur time.Duration
	created *directoryapi.Person
	err error
}

// Expect asserts no error and exact return values.
func (_tc *CreateTestCase) Expect(created *directoryapi.Person) *CreateTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, created, _tc.created)
	}
	return _tc
}

// Error asserts an error.
func (tc *CreateTestCase) Error(errContains string) *CreateTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *CreateTestCase) ErrorCode(statusCode int) *CreateTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *CreateTestCase) NoError() *CreateTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *CreateTestCase) CompletedIn(threshold time.Duration) *CreateTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *CreateTestCase) Assert(asserter func(t *testing.T, created *directoryapi.Person, err error)) *CreateTestCase {
	asserter(tc._t, tc.created, tc.err)
	return tc
}

// Get returns the result of executing Create.
func (tc *CreateTestCase) Get() (created *directoryapi.Person, err error) {
	return tc.created, tc.err
}

// Create executes the function and returns a corresponding test case.
func Create(t *testing.T, ctx context.Context, person *directoryapi.Person) *CreateTestCase {
	tc := &CreateTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.created, tc.err = Svc.Create(ctx, person)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// LoadTestCase assists in asserting against the results of executing Load.
type LoadTestCase struct {
	_t *testing.T
	_dur time.Duration
	person *directoryapi.Person
	ok bool
	err error
}

// Expect asserts no error and exact return values.
func (_tc *LoadTestCase) Expect(person *directoryapi.Person, ok bool) *LoadTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, person, _tc.person)
		assert.Equal(_tc._t, ok, _tc.ok)
	}
	return _tc
}

// Error asserts an error.
func (tc *LoadTestCase) Error(errContains string) *LoadTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *LoadTestCase) ErrorCode(statusCode int) *LoadTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *LoadTestCase) NoError() *LoadTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *LoadTestCase) CompletedIn(threshold time.Duration) *LoadTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *LoadTestCase) Assert(asserter func(t *testing.T, person *directoryapi.Person, ok bool, err error)) *LoadTestCase {
	asserter(tc._t, tc.person, tc.ok, tc.err)
	return tc
}

// Get returns the result of executing Load.
func (tc *LoadTestCase) Get() (person *directoryapi.Person, ok bool, err error) {
	return tc.person, tc.ok, tc.err
}

// Load executes the function and returns a corresponding test case.
func Load(t *testing.T, ctx context.Context, key directoryapi.PersonKey) *LoadTestCase {
	tc := &LoadTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.person, tc.ok, tc.err = Svc.Load(ctx, key)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// DeleteTestCase assists in asserting against the results of executing Delete.
type DeleteTestCase struct {
	_t *testing.T
	_dur time.Duration
	ok bool
	err error
}

// Expect asserts no error and exact return values.
func (_tc *DeleteTestCase) Expect(ok bool) *DeleteTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, ok, _tc.ok)
	}
	return _tc
}

// Error asserts an error.
func (tc *DeleteTestCase) Error(errContains string) *DeleteTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *DeleteTestCase) ErrorCode(statusCode int) *DeleteTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *DeleteTestCase) NoError() *DeleteTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *DeleteTestCase) CompletedIn(threshold time.Duration) *DeleteTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *DeleteTestCase) Assert(asserter func(t *testing.T, ok bool, err error)) *DeleteTestCase {
	asserter(tc._t, tc.ok, tc.err)
	return tc
}

// Get returns the result of executing Delete.
func (tc *DeleteTestCase) Get() (ok bool, err error) {
	return tc.ok, tc.err
}

// Delete executes the function and returns a corresponding test case.
func Delete(t *testing.T, ctx context.Context, key directoryapi.PersonKey) *DeleteTestCase {
	tc := &DeleteTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.ok, tc.err = Svc.Delete(ctx, key)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// UpdateTestCase assists in asserting against the results of executing Update.
type UpdateTestCase struct {
	_t *testing.T
	_dur time.Duration
	updated *directoryapi.Person
	ok bool
	err error
}

// Expect asserts no error and exact return values.
func (_tc *UpdateTestCase) Expect(updated *directoryapi.Person, ok bool) *UpdateTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, updated, _tc.updated)
		assert.Equal(_tc._t, ok, _tc.ok)
	}
	return _tc
}

// Error asserts an error.
func (tc *UpdateTestCase) Error(errContains string) *UpdateTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *UpdateTestCase) ErrorCode(statusCode int) *UpdateTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *UpdateTestCase) NoError() *UpdateTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *UpdateTestCase) CompletedIn(threshold time.Duration) *UpdateTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *UpdateTestCase) Assert(asserter func(t *testing.T, updated *directoryapi.Person, ok bool, err error)) *UpdateTestCase {
	asserter(tc._t, tc.updated, tc.ok, tc.err)
	return tc
}

// Get returns the result of executing Update.
func (tc *UpdateTestCase) Get() (updated *directoryapi.Person, ok bool, err error) {
	return tc.updated, tc.ok, tc.err
}

// Update executes the function and returns a corresponding test case.
func Update(t *testing.T, ctx context.Context, person *directoryapi.Person) *UpdateTestCase {
	tc := &UpdateTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.updated, tc.ok, tc.err = Svc.Update(ctx, person)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// LoadByEmailTestCase assists in asserting against the results of executing LoadByEmail.
type LoadByEmailTestCase struct {
	_t *testing.T
	_dur time.Duration
	person *directoryapi.Person
	ok bool
	err error
}

// Expect asserts no error and exact return values.
func (_tc *LoadByEmailTestCase) Expect(person *directoryapi.Person, ok bool) *LoadByEmailTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, person, _tc.person)
		assert.Equal(_tc._t, ok, _tc.ok)
	}
	return _tc
}

// Error asserts an error.
func (tc *LoadByEmailTestCase) Error(errContains string) *LoadByEmailTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *LoadByEmailTestCase) ErrorCode(statusCode int) *LoadByEmailTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *LoadByEmailTestCase) NoError() *LoadByEmailTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *LoadByEmailTestCase) CompletedIn(threshold time.Duration) *LoadByEmailTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *LoadByEmailTestCase) Assert(asserter func(t *testing.T, person *directoryapi.Person, ok bool, err error)) *LoadByEmailTestCase {
	asserter(tc._t, tc.person, tc.ok, tc.err)
	return tc
}

// Get returns the result of executing LoadByEmail.
func (tc *LoadByEmailTestCase) Get() (person *directoryapi.Person, ok bool, err error) {
	return tc.person, tc.ok, tc.err
}

// LoadByEmail executes the function and returns a corresponding test case.
func LoadByEmail(t *testing.T, ctx context.Context, email string) *LoadByEmailTestCase {
	tc := &LoadByEmailTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.person, tc.ok, tc.err = Svc.LoadByEmail(ctx, email)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// ListTestCase assists in asserting against the results of executing List.
type ListTestCase struct {
	_t *testing.T
	_dur time.Duration
	keys []directoryapi.PersonKey
	err error
}

// Expect asserts no error and exact return values.
func (_tc *ListTestCase) Expect(keys []directoryapi.PersonKey) *ListTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, keys, _tc.keys)
	}
	return _tc
}

// Error asserts an error.
func (tc *ListTestCase) Error(errContains string) *ListTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *ListTestCase) ErrorCode(statusCode int) *ListTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *ListTestCase) NoError() *ListTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *ListTestCase) CompletedIn(threshold time.Duration) *ListTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *ListTestCase) Assert(asserter func(t *testing.T, keys []directoryapi.PersonKey, err error)) *ListTestCase {
	asserter(tc._t, tc.keys, tc.err)
	return tc
}

// Get returns the result of executing List.
func (tc *ListTestCase) Get() (keys []directoryapi.PersonKey, err error) {
	return tc.keys, tc.err
}

// List executes the function and returns a corresponding test case.
func List(t *testing.T, ctx context.Context) *ListTestCase {
	tc := &ListTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.keys, tc.err = Svc.List(ctx)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}
