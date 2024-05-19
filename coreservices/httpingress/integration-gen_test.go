/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

package httpingress

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

	"github.com/microbus-io/fabric/coreservices/httpingress/httpingressapi"
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
	_ *httpingressapi.Client
)

var (
	// App manages the lifecycle of the microservices used in the test
	App *application.Application
	// Svc is the http.ingress.sys microservice being tested
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

// OnChangedPortsTestCase assists in asserting against the results of executing OnChangedPorts.
type OnChangedPortsTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedPortsTestCase) Error(errContains string) *OnChangedPortsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedPortsTestCase) ErrorCode(statusCode int) *OnChangedPortsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedPortsTestCase) NoError() *OnChangedPortsTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedPortsTestCase) CompletedIn(threshold time.Duration) *OnChangedPortsTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedPortsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedPortsTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing Ports.
func (tc *OnChangedPortsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedPorts executes the on changed callback and returns a corresponding test case.
func OnChangedPorts(t *testing.T, ctx context.Context) *OnChangedPortsTestCase {
	tc := &OnChangedPortsTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedPorts(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedAllowedOriginsTestCase assists in asserting against the results of executing OnChangedAllowedOrigins.
type OnChangedAllowedOriginsTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedAllowedOriginsTestCase) Error(errContains string) *OnChangedAllowedOriginsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedAllowedOriginsTestCase) ErrorCode(statusCode int) *OnChangedAllowedOriginsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedAllowedOriginsTestCase) NoError() *OnChangedAllowedOriginsTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedAllowedOriginsTestCase) CompletedIn(threshold time.Duration) *OnChangedAllowedOriginsTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedAllowedOriginsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedAllowedOriginsTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing AllowedOrigins.
func (tc *OnChangedAllowedOriginsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedAllowedOrigins executes the on changed callback and returns a corresponding test case.
func OnChangedAllowedOrigins(t *testing.T, ctx context.Context) *OnChangedAllowedOriginsTestCase {
	tc := &OnChangedAllowedOriginsTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedAllowedOrigins(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedPortMappingsTestCase assists in asserting against the results of executing OnChangedPortMappings.
type OnChangedPortMappingsTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedPortMappingsTestCase) Error(errContains string) *OnChangedPortMappingsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedPortMappingsTestCase) ErrorCode(statusCode int) *OnChangedPortMappingsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedPortMappingsTestCase) NoError() *OnChangedPortMappingsTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedPortMappingsTestCase) CompletedIn(threshold time.Duration) *OnChangedPortMappingsTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedPortMappingsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedPortMappingsTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing PortMappings.
func (tc *OnChangedPortMappingsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedPortMappings executes the on changed callback and returns a corresponding test case.
func OnChangedPortMappings(t *testing.T, ctx context.Context) *OnChangedPortMappingsTestCase {
	tc := &OnChangedPortMappingsTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedPortMappings(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedReadTimeoutTestCase assists in asserting against the results of executing OnChangedReadTimeout.
type OnChangedReadTimeoutTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedReadTimeoutTestCase) Error(errContains string) *OnChangedReadTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedReadTimeoutTestCase) ErrorCode(statusCode int) *OnChangedReadTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedReadTimeoutTestCase) NoError() *OnChangedReadTimeoutTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedReadTimeoutTestCase) CompletedIn(threshold time.Duration) *OnChangedReadTimeoutTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedReadTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedReadTimeoutTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing ReadTimeout.
func (tc *OnChangedReadTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedReadTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedReadTimeout(t *testing.T, ctx context.Context) *OnChangedReadTimeoutTestCase {
	tc := &OnChangedReadTimeoutTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedReadTimeout(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedWriteTimeoutTestCase assists in asserting against the results of executing OnChangedWriteTimeout.
type OnChangedWriteTimeoutTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedWriteTimeoutTestCase) Error(errContains string) *OnChangedWriteTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedWriteTimeoutTestCase) ErrorCode(statusCode int) *OnChangedWriteTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedWriteTimeoutTestCase) NoError() *OnChangedWriteTimeoutTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedWriteTimeoutTestCase) CompletedIn(threshold time.Duration) *OnChangedWriteTimeoutTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedWriteTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedWriteTimeoutTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing WriteTimeout.
func (tc *OnChangedWriteTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedWriteTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedWriteTimeout(t *testing.T, ctx context.Context) *OnChangedWriteTimeoutTestCase {
	tc := &OnChangedWriteTimeoutTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedWriteTimeout(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedReadHeaderTimeoutTestCase assists in asserting against the results of executing OnChangedReadHeaderTimeout.
type OnChangedReadHeaderTimeoutTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedReadHeaderTimeoutTestCase) Error(errContains string) *OnChangedReadHeaderTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedReadHeaderTimeoutTestCase) ErrorCode(statusCode int) *OnChangedReadHeaderTimeoutTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedReadHeaderTimeoutTestCase) NoError() *OnChangedReadHeaderTimeoutTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedReadHeaderTimeoutTestCase) CompletedIn(threshold time.Duration) *OnChangedReadHeaderTimeoutTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedReadHeaderTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedReadHeaderTimeoutTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing ReadHeaderTimeout.
func (tc *OnChangedReadHeaderTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedReadHeaderTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedReadHeaderTimeout(t *testing.T, ctx context.Context) *OnChangedReadHeaderTimeoutTestCase {
	tc := &OnChangedReadHeaderTimeoutTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedReadHeaderTimeout(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedServerLanguagesTestCase assists in asserting against the results of executing OnChangedServerLanguages.
type OnChangedServerLanguagesTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedServerLanguagesTestCase) Error(errContains string) *OnChangedServerLanguagesTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedServerLanguagesTestCase) ErrorCode(statusCode int) *OnChangedServerLanguagesTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedServerLanguagesTestCase) NoError() *OnChangedServerLanguagesTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedServerLanguagesTestCase) CompletedIn(threshold time.Duration) *OnChangedServerLanguagesTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedServerLanguagesTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedServerLanguagesTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing ServerLanguages.
func (tc *OnChangedServerLanguagesTestCase) Get() (err error) {
	return tc.err
}

// OnChangedServerLanguages executes the on changed callback and returns a corresponding test case.
func OnChangedServerLanguages(t *testing.T, ctx context.Context) *OnChangedServerLanguagesTestCase {
	tc := &OnChangedServerLanguagesTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedServerLanguages(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}

// OnChangedBlockedPathsTestCase assists in asserting against the results of executing OnChangedBlockedPaths.
type OnChangedBlockedPathsTestCase struct {
	t *testing.T
	err error
	dur time.Duration
}

// Error asserts an error.
func (tc *OnChangedBlockedPathsTestCase) Error(errContains string) *OnChangedBlockedPathsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedBlockedPathsTestCase) ErrorCode(statusCode int) *OnChangedBlockedPathsTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *OnChangedBlockedPathsTestCase) NoError() *OnChangedBlockedPathsTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *OnChangedBlockedPathsTestCase) CompletedIn(threshold time.Duration) *OnChangedBlockedPathsTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedBlockedPathsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedBlockedPathsTestCase {
	asserter(tc.t, tc.err)
	return tc
}

// Get returns the result of executing BlockedPaths.
func (tc *OnChangedBlockedPathsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedBlockedPaths executes the on changed callback and returns a corresponding test case.
func OnChangedBlockedPaths(t *testing.T, ctx context.Context) *OnChangedBlockedPathsTestCase {
	tc := &OnChangedBlockedPathsTestCase{t: t}
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.OnChangedBlockedPaths(ctx)
	})
	tc.dur = time.Since(t0)
	return tc
}
