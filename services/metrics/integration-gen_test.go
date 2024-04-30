/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

package metrics

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

	"github.com/microbus-io/fabric/services/metrics/metricsapi"
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
	_ *metricsapi.Client
)

var (
	sequence int
)

var (
	// App manages the lifecycle of the microservices used in the test
	App *application.Application
	// Svc is the metrics.sys microservice being tested
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

type WebOption func(req *pub.Request) error

// GET sets the method of the request.
func GET() WebOption {
	return WebOption(pub.Method("GET"))
}

// DELETE sets the method of the request.
func DELETE() WebOption {
	return WebOption(pub.Method("DELETE"))
}

// HEAD sets the method of the request.
func HEAD() WebOption {
	return WebOption(pub.Method("HEAD"))
}

// POST sets the method and body of the request.
func POST(body any) WebOption {
	return func(req *pub.Request) error {
		pub.Method("POST")(req)
		return pub.Body(body)(req)
	}
}

// PUT sets the method and body of the request.
func PUT(body any) WebOption {
	return func(req *pub.Request) error {
		pub.Method("PUT")(req)
		return pub.Body(body)(req)
	}
}

// PATCH sets the method and body of the request.
func PATCH(body any) WebOption {
	return func(req *pub.Request) error {
		pub.Method("PATCH")(req)
		return pub.Body(body)(req)
	}
}

// Method sets the method of the request.
func Method(method string) WebOption {
	return WebOption(pub.Method(method))
}

// Header sets the header of the request. It overwrites any previously set value.
func Header(name string, value string) WebOption {
	return WebOption(pub.Header(name, value))
}

// QueryArg adds the query argument to the request.
// The same argument may have multiple values.
func QueryArg(name string, value any) WebOption {
	return WebOption(pub.QueryArg(name, value))
}

// Query adds the escaped query arguments to the request.
// The same argument may have multiple values.
func Query(escapedQueryArgs string) WebOption {
	return WebOption(pub.QueryString(escapedQueryArgs))
}

// Body sets the body of the request.
// Arguments of type io.Reader, []byte and string are serialized in binary form.
// url.Values is serialized as form data.
// All other types are serialized as JSON.
func Body(body any) WebOption {
	return WebOption(pub.Body(body))
}

// ContentType sets the Content-Type header.
func ContentType(contentType string) WebOption {
	return WebOption(pub.ContentType(contentType))
}

// CollectTestCase assists in asserting against the results of executing Collect.
type CollectTestCase struct {
	t *testing.T
	testName string
	res *http.Response
	err error
}

// Name sets a name to the test case.
func (tc *CollectTestCase) Name(testName string) *CollectTestCase {
	tc.testName = testName
	return tc
}

// StatusOK asserts no error and a status code 200.
func (tc *CollectTestCase) StatusOK() *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.NoError(t, tc.err) {
			assert.Equal(t, tc.res.StatusCode, http.StatusOK)
		}
	})
	return tc
}

// StatusCode asserts no error and a status code.
func (tc *CollectTestCase) StatusCode(statusCode int) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.NoError(t, tc.err) {
			assert.Equal(t, tc.res.StatusCode, statusCode)
		}
	})
	return tc
}

// BodyContains asserts no error and that the response contains a string or byte array.
func (tc *CollectTestCase) BodyContains(bodyContains any) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.NoError(t, tc.err) {
			body := tc.res.Body.(*httpx.BodyReader).Bytes()
			switch v := bodyContains.(type) {
			case []byte:
				assert.True(t, bytes.Contains(body, v), `"%v" does not contain "%v"`, body, v)
			case string:
				assert.True(t, bytes.Contains(body, []byte(v)), `"%s" does not contain "%s"`, string(body), v)
			default:
				vv := fmt.Sprintf("%v", v)
				assert.True(t, bytes.Contains(body, []byte(vv)), `"%s" does not contain "%s"`, string(body), vv)
			}
		}
	})
	return tc
}

// BodyNotContains asserts no error and that the response does not contain a string or byte array.
func (tc *CollectTestCase) BodyNotContains(bodyNotContains any) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.NoError(t, tc.err) {
			body := tc.res.Body.(*httpx.BodyReader).Bytes()
			switch v := bodyNotContains.(type) {
			case []byte:
				assert.False(t, bytes.Contains(body, v), `"%v" contains "%v"`, body, v)
			case string:
				assert.False(t, bytes.Contains(body, []byte(v)), `"%s" contains "%s"`, string(body), v)
			default:
				vv := fmt.Sprintf("%v", v)
				assert.False(t, bytes.Contains(body, []byte(vv)), `"%s" contains "%s"`, string(body), vv)
			}
		}
	})
	return tc
}

// HeaderContains asserts no error and that the named header contains a string.
func (tc *CollectTestCase) HeaderContains(headerName string, valueContains string) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.NoError(t, tc.err) {
			assert.True(t, strings.Contains(tc.res.Header.Get(headerName), valueContains), `header "%s: %s" does not contain "%s"`, headerName, tc.res.Header.Get(headerName), valueContains)
		}
	})
	return tc
}

// Error asserts an error.
func (tc *CollectTestCase) Error(errContains string) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Contains(t, tc.err.Error(), errContains)
		}
	})
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *CollectTestCase) ErrorCode(statusCode int) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		if assert.Error(t, tc.err) {
			assert.Equal(t, statusCode, errors.Convert(tc.err).StatusCode)
		}
	})
	return tc
}

// NoError asserts no error.
func (tc *CollectTestCase) NoError() *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		assert.NoError(t, tc.err)
	})
	return tc
}

// Assert asserts using a provided function.
func (tc *CollectTestCase) Assert(asserter func(t *testing.T, res *http.Response, err error)) *CollectTestCase {
	tc.t.Run(tc.testName, func(t *testing.T) {
		asserter(t, tc.res, tc.err)
	})
	return tc
}

// Get returns the result of executing Collect.
func (tc *CollectTestCase) Get() (res *http.Response, err error) {
	return tc.res, tc.err
}

// Collect executes the web handler and returns a corresponding test case.
func Collect(t *testing.T, ctx context.Context, options ...WebOption) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	pubOptions := []pub.Option{
		pub.URL(httpx.JoinHostAndPath("metrics.sys", `:443/collect`)),
	}
	frameHeader := frame.Of(ctx).Header()
	for h := range frameHeader {
		pubOptions = append(pubOptions, pub.Header(h, frameHeader.Get(h)))
	}
	for _, opt := range options {
		pubOptions = append(pubOptions, pub.Option(opt))
	}
	req, err := pub.NewRequest(pubOptions...)
	if err != nil {
		panic(err)
	}
	httpReq, err := http.NewRequest(req.Method, req.URL, req.Body)
	if err != nil {
		panic(err)
	}
	for name, value := range req.Header {
		httpReq.Header[name] = value
	}
	r := httpReq.WithContext(ctx)
	w := httpx.NewResponseRecorder()
	tc.err = utils.CatchPanic(func () error {
		return Svc.Collect(w, r)
	})
	tc.res = w.Result()
	return tc
}
