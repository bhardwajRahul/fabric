/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

package eventsink

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

	"github.com/microbus-io/fabric/examples/eventsink/eventsinkapi"
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
	_ *eventsinkapi.Client
)

var (
	// App manages the lifecycle of the microservices used in the test
	App *application.Application
	// Svc is the eventsink.example microservice being tested
	Svc *Service
)

func TestMain(m *testing.M) {
	var code int

	// Initialize the application
	err := func() error {
		var err error
		App = application.NewTesting()
		Svc = NewService().(*Service)
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
	return context.WithValue(context.Background(), frame.ContextKey, http.Header{})
}

// RegisteredTestCase assists in asserting against the results of executing Registered.
type RegisteredTestCase struct {
	_t *testing.T
	emails []string
	err error
	_dur time.Duration
}

// Expect asserts no error and exact return values.
func (_tc *RegisteredTestCase) Expect(emails []string) *RegisteredTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, emails, _tc.emails)
	}
	return _tc
}

// Error asserts an error.
func (tc *RegisteredTestCase) Error(errContains string) *RegisteredTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *RegisteredTestCase) ErrorCode(statusCode int) *RegisteredTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *RegisteredTestCase) NoError() *RegisteredTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *RegisteredTestCase) CompletedIn(threshold time.Duration) *RegisteredTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *RegisteredTestCase) Assert(asserter func(t *testing.T, emails []string, err error)) *RegisteredTestCase {
	asserter(tc._t, tc.emails, tc.err)
	return tc
}

// Get returns the result of executing Registered.
func (tc *RegisteredTestCase) Get() (emails []string, err error) {
	return tc.emails, tc.err
}

// Registered executes the function and returns a corresponding test case.
func Registered(t *testing.T, ctx context.Context) *RegisteredTestCase {
	tc := &RegisteredTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.emails, tc.err = Svc.Registered(ctx)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// OnAllowRegisterTestCase assists in asserting against the results of executing OnAllowRegister.
type OnAllowRegisterTestCase struct {
	_t *testing.T
	allow bool
	err error
	_dur time.Duration
}

// Expect asserts no error and exact return values.
func (_tc *OnAllowRegisterTestCase) Expect(allow bool) *OnAllowRegisterTestCase {
	if assert.NoError(_tc._t, _tc.err) {
		assert.Equal(_tc._t, allow, _tc.allow)
	}
	return _tc
}

// Error asserts an error.
func (tc *OnAllowRegisterTestCase) Error(errContains string) *OnAllowRegisterTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnAllowRegisterTestCase) ErrorCode(statusCode int) *OnAllowRegisterTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *OnAllowRegisterTestCase) NoError() *OnAllowRegisterTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnAllowRegisterTestCase) CompletedIn(threshold time.Duration) *OnAllowRegisterTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnAllowRegisterTestCase) Assert(asserter func(t *testing.T, allow bool, err error)) *OnAllowRegisterTestCase {
	asserter(tc._t, tc.allow, tc.err)
	return tc
}

// Get returns the result of executing OnAllowRegister.
func (tc *OnAllowRegisterTestCase) Get() (allow bool, err error) {
	return tc.allow, tc.err
}

// OnAllowRegister executes the function and returns a corresponding test case.
func OnAllowRegister(t *testing.T, ctx context.Context, email string) *OnAllowRegisterTestCase {
	tc := &OnAllowRegisterTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.allow, tc.err = Svc.OnAllowRegister(ctx, email)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}

// OnRegisteredTestCase assists in asserting against the results of executing OnRegistered.
type OnRegisteredTestCase struct {
	_t *testing.T
	err error
	_dur time.Duration
}

// Expect asserts no error and exact return values.
func (_tc *OnRegisteredTestCase) Expect() *OnRegisteredTestCase {
	assert.NoError(_tc._t, _tc.err)
	return _tc
}

// Error asserts an error.
func (tc *OnRegisteredTestCase) Error(errContains string) *OnRegisteredTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Contains(tc._t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnRegisteredTestCase) ErrorCode(statusCode int) *OnRegisteredTestCase {
	if assert.Error(tc._t, tc.err) {
		assert.Equal(tc._t, statusCode, errors.StatusCode(tc.err))
	}
	return tc
}

// NoError asserts no error.
func (tc *OnRegisteredTestCase) NoError() *OnRegisteredTestCase {
	assert.NoError(tc._t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnRegisteredTestCase) CompletedIn(threshold time.Duration) *OnRegisteredTestCase {
	assert.LessOrEqual(tc._t, tc._dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnRegisteredTestCase) Assert(asserter func(t *testing.T, err error)) *OnRegisteredTestCase {
	asserter(tc._t, tc.err)
	return tc
}

// Get returns the result of executing OnRegistered.
func (tc *OnRegisteredTestCase) Get() (err error) {
	return tc.err
}

// OnRegistered executes the function and returns a corresponding test case.
func OnRegistered(t *testing.T, ctx context.Context, email string) *OnRegisteredTestCase {
	tc := &OnRegisteredTestCase{_t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		tc.err = Svc.OnRegistered(ctx, email)
		return tc.err
	})
	tc._dur = time.Since(t0)
	return tc
}
