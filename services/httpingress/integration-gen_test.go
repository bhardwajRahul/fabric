/*
Copyright (c) 2023 Microbus LLC and various contributors

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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/shardedsql"
	"github.com/microbus-io/fabric/utils"

	"github.com/stretchr/testify/assert"

	"github.com/microbus-io/fabric/services/httpingress/httpingressapi"
)

var (
	_ bytes.Buffer
	_ context.Context
	_ fmt.Stringer
	_ io.Reader
	_ *http.Request
	_ os.File
	_ time.Time
	_ strings.Builder
	_ *connector.Connector
	_ *errors.TracedError
	_ frame.Frame
	_ *httpx.BodyReader
	_ pub.Option
	_ *shardedsql.DB
	_ utils.InfiniteChan[int]
	_ assert.TestingT
	_ *httpingressapi.Client
)

var (
	sequence int
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
	return context.Background()
}

// OnChangedPortsTestCase assists in asserting against the results of executing OnChangedPorts.
type OnChangedPortsTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedPortsTestCase) Name(testName string) *OnChangedPortsTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedPortsTestCase) Error(errContains string) *OnChangedPortsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedPortsTestCase) ErrorCode(statusCode int) *OnChangedPortsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedPortsTestCase) NoError() *OnChangedPortsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedPortsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedPortsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing Ports.
func (tc *OnChangedPortsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedPorts executes the on changed callback and returns a corresponding test case.
func OnChangedPorts(t *testing.T, ctx context.Context) *OnChangedPortsTestCase {
	tc := &OnChangedPortsTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedPorts(ctx)
	})
	return tc
}

// OnChangedAllowedOriginsTestCase assists in asserting against the results of executing OnChangedAllowedOrigins.
type OnChangedAllowedOriginsTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedAllowedOriginsTestCase) Name(testName string) *OnChangedAllowedOriginsTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedAllowedOriginsTestCase) Error(errContains string) *OnChangedAllowedOriginsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedAllowedOriginsTestCase) ErrorCode(statusCode int) *OnChangedAllowedOriginsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedAllowedOriginsTestCase) NoError() *OnChangedAllowedOriginsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedAllowedOriginsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedAllowedOriginsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing AllowedOrigins.
func (tc *OnChangedAllowedOriginsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedAllowedOrigins executes the on changed callback and returns a corresponding test case.
func OnChangedAllowedOrigins(t *testing.T, ctx context.Context) *OnChangedAllowedOriginsTestCase {
	tc := &OnChangedAllowedOriginsTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedAllowedOrigins(ctx)
	})
	return tc
}

// OnChangedPortMappingsTestCase assists in asserting against the results of executing OnChangedPortMappings.
type OnChangedPortMappingsTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedPortMappingsTestCase) Name(testName string) *OnChangedPortMappingsTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedPortMappingsTestCase) Error(errContains string) *OnChangedPortMappingsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedPortMappingsTestCase) ErrorCode(statusCode int) *OnChangedPortMappingsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedPortMappingsTestCase) NoError() *OnChangedPortMappingsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedPortMappingsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedPortMappingsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing PortMappings.
func (tc *OnChangedPortMappingsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedPortMappings executes the on changed callback and returns a corresponding test case.
func OnChangedPortMappings(t *testing.T, ctx context.Context) *OnChangedPortMappingsTestCase {
	tc := &OnChangedPortMappingsTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedPortMappings(ctx)
	})
	return tc
}

// OnChangedReadTimeoutTestCase assists in asserting against the results of executing OnChangedReadTimeout.
type OnChangedReadTimeoutTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedReadTimeoutTestCase) Name(testName string) *OnChangedReadTimeoutTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedReadTimeoutTestCase) Error(errContains string) *OnChangedReadTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedReadTimeoutTestCase) ErrorCode(statusCode int) *OnChangedReadTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedReadTimeoutTestCase) NoError() *OnChangedReadTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedReadTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedReadTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing ReadTimeout.
func (tc *OnChangedReadTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedReadTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedReadTimeout(t *testing.T, ctx context.Context) *OnChangedReadTimeoutTestCase {
	tc := &OnChangedReadTimeoutTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedReadTimeout(ctx)
	})
	return tc
}

// OnChangedWriteTimeoutTestCase assists in asserting against the results of executing OnChangedWriteTimeout.
type OnChangedWriteTimeoutTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedWriteTimeoutTestCase) Name(testName string) *OnChangedWriteTimeoutTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedWriteTimeoutTestCase) Error(errContains string) *OnChangedWriteTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedWriteTimeoutTestCase) ErrorCode(statusCode int) *OnChangedWriteTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedWriteTimeoutTestCase) NoError() *OnChangedWriteTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedWriteTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedWriteTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing WriteTimeout.
func (tc *OnChangedWriteTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedWriteTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedWriteTimeout(t *testing.T, ctx context.Context) *OnChangedWriteTimeoutTestCase {
	tc := &OnChangedWriteTimeoutTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedWriteTimeout(ctx)
	})
	return tc
}

// OnChangedReadHeaderTimeoutTestCase assists in asserting against the results of executing OnChangedReadHeaderTimeout.
type OnChangedReadHeaderTimeoutTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedReadHeaderTimeoutTestCase) Name(testName string) *OnChangedReadHeaderTimeoutTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedReadHeaderTimeoutTestCase) Error(errContains string) *OnChangedReadHeaderTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedReadHeaderTimeoutTestCase) ErrorCode(statusCode int) *OnChangedReadHeaderTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedReadHeaderTimeoutTestCase) NoError() *OnChangedReadHeaderTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedReadHeaderTimeoutTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedReadHeaderTimeoutTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing ReadHeaderTimeout.
func (tc *OnChangedReadHeaderTimeoutTestCase) Get() (err error) {
	return tc.err
}

// OnChangedReadHeaderTimeout executes the on changed callback and returns a corresponding test case.
func OnChangedReadHeaderTimeout(t *testing.T, ctx context.Context) *OnChangedReadHeaderTimeoutTestCase {
	tc := &OnChangedReadHeaderTimeoutTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedReadHeaderTimeout(ctx)
	})
	return tc
}

// OnChangedServerLanguagesTestCase assists in asserting against the results of executing OnChangedServerLanguages.
type OnChangedServerLanguagesTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedServerLanguagesTestCase) Name(testName string) *OnChangedServerLanguagesTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedServerLanguagesTestCase) Error(errContains string) *OnChangedServerLanguagesTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedServerLanguagesTestCase) ErrorCode(statusCode int) *OnChangedServerLanguagesTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedServerLanguagesTestCase) NoError() *OnChangedServerLanguagesTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedServerLanguagesTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedServerLanguagesTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing ServerLanguages.
func (tc *OnChangedServerLanguagesTestCase) Get() (err error) {
	return tc.err
}

// OnChangedServerLanguages executes the on changed callback and returns a corresponding test case.
func OnChangedServerLanguages(t *testing.T, ctx context.Context) *OnChangedServerLanguagesTestCase {
	tc := &OnChangedServerLanguagesTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedServerLanguages(ctx)
	})
	return tc
}

// OnChangedBlockedPathsTestCase assists in asserting against the results of executing OnChangedBlockedPaths.
type OnChangedBlockedPathsTestCase struct {
	t *testing.T
	testName string
	err error
}

// Name sets a name to the test case.
func (tc *OnChangedBlockedPathsTestCase) Name(testName string) *OnChangedBlockedPathsTestCase {
	tc.testName = testName
	return tc
}

// Error asserts an error.
func (tc *OnChangedBlockedPathsTestCase) Error(errContains string) *OnChangedBlockedPathsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *OnChangedBlockedPathsTestCase) ErrorCode(statusCode int) *OnChangedBlockedPathsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *OnChangedBlockedPathsTestCase) NoError() *OnChangedBlockedPathsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *OnChangedBlockedPathsTestCase) Assert(asserter func(t *testing.T, err error)) *OnChangedBlockedPathsTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.err)
	})
	return tc
}

// Get returns the result of executing BlockedPaths.
func (tc *OnChangedBlockedPathsTestCase) Get() (err error) {
	return tc.err
}

// OnChangedBlockedPaths executes the on changed callback and returns a corresponding test case.
func OnChangedBlockedPaths(t *testing.T, ctx context.Context) *OnChangedBlockedPathsTestCase {
	tc := &OnChangedBlockedPathsTestCase{t: t}
	tc.err = utils.CatchPanic(func () error {
		return Svc.OnChangedBlockedPaths(ctx)
	})
	return tc
}
