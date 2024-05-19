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

	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
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
	_ *metricsapi.Client
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

// CollectTestCase assists in asserting against the results of executing Collect.
type CollectTestCase struct {
	t *testing.T
	res *http.Response
	err error
	dur time.Duration
}

// StatusOK asserts no error and a status code 200.
func (tc *CollectTestCase) StatusOK() *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Equal(tc.t, tc.res.StatusCode, http.StatusOK)
	}
	return tc
}

// StatusCode asserts no error and a status code.
func (tc *CollectTestCase) StatusCode(statusCode int) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Equal(tc.t, tc.res.StatusCode, statusCode)
	}
	return tc
}

// BodyContains asserts no error and that the response contains the string or byte array value.
func (tc *CollectTestCase) BodyContains(value any) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		switch v := value.(type) {
		case []byte:
			assert.True(tc.t, bytes.Contains(body, v), "%v does not contain %v", body, v)
		case string:
			assert.Contains(tc.t, string(body), v)
		default:
			vv := fmt.Sprintf("%v", v)
			assert.Contains(tc.t, string(body), vv)
		}
	}
	return tc
}

// BodyNotContains asserts no error and that the response does not contain the string or byte array value.
func (tc *CollectTestCase) BodyNotContains(value any) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		switch v := value.(type) {
		case []byte:
			assert.False(tc.t, bytes.Contains(body, v), "%v contains %v", body, v)
		case string:
			assert.NotContains(tc.t, string(body), v)
		default:
			vv := fmt.Sprintf("%v", v)
			assert.NotContains(tc.t, string(body), vv)
		}
	}
	return tc
}

// HeaderContains asserts no error and that the named header contains the value.
func (tc *CollectTestCase) HeaderContains(headerName string, value string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Contains(tc.t, tc.res.Header.Get(headerName), value)
	}
	return tc
}

// HeaderNotContains asserts no error and that the named header does not contain a string.
func (tc *CollectTestCase) HeaderNotContains(headerName string, value string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.NotContains(tc.t, tc.res.Header.Get(headerName), value)
	}
	return tc
}

// HeaderEqual asserts no error and that the named header matches the value.
func (tc *CollectTestCase) HeaderEqual(headerName string, value string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Equal(tc.t, value, tc.res.Header.Get(headerName))
	}
	return tc
}

// HeaderNotEqual asserts no error and that the named header does not matche the value.
func (tc *CollectTestCase) HeaderNotEqual(headerName string, value string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.NotEqual(tc.t, value, tc.res.Header.Get(headerName))
	}
	return tc
}

// HeaderExists asserts no error and that the named header exists.
func (tc *CollectTestCase) HeaderExists(headerName string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.NotEmpty(tc.t, tc.res.Header.Values(headerName), "header %s does not exist", headerName)
	}
	return tc
}

// HeaderNotExists asserts no error and that the named header does not exists.
func (tc *CollectTestCase) HeaderNotExists(headerName string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Empty(tc.t, tc.res.Header.Values(headerName), "header %s exists", headerName)
	}
	return tc
}

// ContentType asserts no error and that the Content-Type header matches the expected value.
func (tc *CollectTestCase) ContentType(expected string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		assert.Equal(tc.t, expected, tc.res.Header.Get("Content-Type"))
	}
	return tc
}

/*
TagExists asserts no error and that the at least one tag matches the CSS selector query.

Examples:

	TagExists(`TR > TD > A.expandable[href]`)
	TagExists(`DIV#main_panel`)
	TagExists(`TR TD INPUT[name="x"]`)
*/
func (tc *CollectTestCase) TagExists(cssSelectorQuery string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		assert.NotEmpty(tc.t, matches, "found no tags matching %s", cssSelectorQuery)
	}
	return tc
}

/*
TagNotExists asserts no error and that the no tag matches the CSS selector query.

Example:

	TagNotExists(`TR > TD > A.expandable[href]`)
	TagNotExists(`DIV#main_panel`)
	TagNotExists(`TR TD INPUT[name="x"]`)
*/
func (tc *CollectTestCase) TagNotExists(cssSelectorQuery string) *CollectTestCase {
	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		assert.Empty(tc.t, matches, "found %d tag(s) matching %s", len(matches), cssSelectorQuery)
	}
	return tc
}

/*
TagEqual asserts no error and that the at least one of the tags matching the CSS selector query
either contains the exact text itself or has a descendant that does.

Example:

	TagEqual("TR > TD > A.expandable[href]", "Expand")
	TagEqual("DIV#main_panel > SELECT > OPTION", "Red")
*/
func (tc *CollectTestCase) TagEqual(cssSelectorQuery string, value string) *CollectTestCase {
	var textMatches func(n *html.Node) bool
	textMatches = func(n *html.Node) bool {
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			if x.Data == value || textMatches(x) {
				return true
			}
		}
		return false
	}

	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if !assert.NotEmpty(tc.t, matches, "selector %s does not match any tags", cssSelectorQuery) {
			return tc
		}
		if value == "" {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		assert.True(tc.t, found, "no tag matching %s contains %s", cssSelectorQuery, value)
	}
	return tc
}

/*
TagContains asserts no error and that the at least one of the tags matching the CSS selector query
either contains the text itself or has a descendant that does.

Example:

	TagContains("TR > TD > A.expandable[href]", "Expand")
	TagContains("DIV#main_panel > SELECT > OPTION", "Red")
*/
func (tc *CollectTestCase) TagContains(cssSelectorQuery string, value string) *CollectTestCase {
	var textMatches func(n *html.Node) bool
	textMatches = func(n *html.Node) bool {
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			if strings.Contains(x.Data, value) || textMatches(x) {
				return true
			}
		}
		return false
	}

	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if !assert.NotEmpty(tc.t, matches, "selector %s does not match any tags", cssSelectorQuery) {
			return tc
		}
		if value == "" {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		assert.True(tc.t, found, "no tag matching %s contains %s", cssSelectorQuery, value)
	}
	return tc
}

/*
TagNotEqual asserts no error and that there is no tag matching the CSS selector that
either contains the exact text itself or has a descendant that does.

Example:

	TagNotEqual("TR > TD > A[href]", "Harry Potter")
	TagNotEqual("DIV#main_panel > SELECT > OPTION", "Red")
*/
func (tc *CollectTestCase) TagNotEqual(cssSelectorQuery string, value string) *CollectTestCase {
	var textMatches func(n *html.Node) bool
	textMatches = func(n *html.Node) bool {
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			if x.Data == value || textMatches(x) {
				return true
			}
		}
		return false
	}

	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if len(matches) == 0 {
			return tc
		}
		if !assert.NotEmpty(tc.t, value, "found tag matching %s", cssSelectorQuery) {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		assert.False(tc.t, found, "found tag matching %s that contains %s", cssSelectorQuery, value)
	}
	return tc
}

/*
TagNotContains asserts no error and that there is no tag matching the CSS selector that
either contains the text itself or has a descendant that does.

Example:

	TagNotContains("TR > TD > A[href]", "Harry Potter")
	TagNotContains("DIV#main_panel > SELECT > OPTION", "Red")
*/
func (tc *CollectTestCase) TagNotContains(cssSelectorQuery string, value string) *CollectTestCase {
	var textMatches func(n *html.Node) bool
	textMatches = func(n *html.Node) bool {
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			if strings.Contains(x.Data, value) || textMatches(x) {
				return true
			}
		}
		return false
	}

	if assert.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !assert.NoError(tc.t, err, "invalid selector %s", cssSelectorQuery) {
			return tc
		}
		body := tc.res.Body.(*httpx.BodyReader).Bytes()
		doc, err := html.Parse(bytes.NewReader(body))
		if !assert.NoError(tc.t, err, "failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if len(matches) == 0 {
			return tc
		}
		if !assert.NotEmpty(tc.t, value, "found tag matching %s", cssSelectorQuery) {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		assert.False(tc.t, found, "found tag matching %s that contains %s", cssSelectorQuery, value)
	}
	return tc
}

// Error asserts an error.
func (tc *CollectTestCase) Error(errContains string) *CollectTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *CollectTestCase) ErrorCode(statusCode int) *CollectTestCase {
	if assert.Error(tc.t, tc.err) {
		assert.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *CollectTestCase) NoError() *CollectTestCase {
	assert.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *CollectTestCase) CompletedIn(threshold time.Duration) *CollectTestCase {
	assert.LessOrEqual(tc.t, tc.dur, threshold)
	return tc
}

// Assert asserts using a provided function.
func (tc *CollectTestCase) Assert(asserter func(t *testing.T, res *http.Response, err error)) *CollectTestCase {
	asserter(tc.t, tc.res, tc.err)
	return tc
}

// Get returns the result of executing Collect.
func (tc *CollectTestCase) Get() (res *http.Response, err error) {
	return tc.res, tc.err
}

/*
CollectGet performs a GET request to the Collect endpoint.

Collect returns the latest aggregated metrics.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func CollectGet(t *testing.T, ctx context.Context, url string) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	url, err = httpx.ResolveURL(metricsapi.URLOfCollect, url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	ctx = context.WithValue(ctx, frame.ContextKey, r.Header)
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.Collect(w, r.WithContext(ctx))
	})
	tc.dur = time.Since(t0)
	tc.res = w.Result()
	return tc
}

/*
CollectPost performs a POST request to the Collect endpoint.

Collect returns the latest aggregated metrics.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func CollectPost(t *testing.T, ctx context.Context, url string, contentType string, body any) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	url, err = httpx.ResolveURL(metricsapi.URLOfCollect, url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r, err := httpx.NewRequest("POST", url, body)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	ctx = context.WithValue(ctx, frame.ContextKey, r.Header)
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.Collect(w, r.WithContext(ctx))
	})
	tc.dur = time.Since(t0)
	tc.res = w.Result()
	return tc
}

/*
Collect returns the latest aggregated metrics.

If a request is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func Collect(t *testing.T, ctx context.Context, r *http.Request) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	if r == nil {
		r, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			tc.err = errors.Trace(err)
			return tc
		}
	}
	u, err := httpx.ResolveURL(metricsapi.URLOfCollect, r.URL.String())
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r.URL, err = url.Parse(u)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	ctx = context.WithValue(ctx, frame.ContextKey, r.Header)
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = utils.CatchPanic(func() error {
		return Svc.Collect(w, r.WithContext(ctx))
	})
	tc.res = w.Result()
	tc.dur = time.Since(t0)
	return tc
}
