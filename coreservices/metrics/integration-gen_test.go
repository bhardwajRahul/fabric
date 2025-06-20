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

// Code generated by Microbus. DO NOT EDIT.

package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	"github.com/microbus-io/testarossa"
	"golang.org/x/net/html"

	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
)

var (
	_ bytes.Buffer
	_ context.Context
	_ fmt.Stringer
	_ io.Reader
	_ *http.Request
	_ os.File
	_ time.Time
	_ *regexp.Regexp
	_ strings.Builder
	_ cascadia.Sel
	_ *connector.Connector
	_ *errors.TracedError
	_ frame.Frame
	_ *httpx.BodyReader
	_ pub.Option
	_ rand.Void
	_ utils.SyncMap[string, string]
	_ testarossa.TestingT
	_ *html.Node
	_ *metricsapi.Client
)

var (
	// App manages the lifecycle of the microservices used in the test
	App *application.Application
	// Svc is the metrics.core microservice being tested
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
		err = App.Shutdown()
		if err != nil {
			lastErr = err
		}
		err = Terminate()
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
func Context() context.Context {
	return frame.ContextWithFrame(context.Background())
}

// CollectTestCase assists in asserting against the results of executing Collect.
type CollectTestCase struct {
	t *testing.T
	dur time.Duration
	res *http.Response
	err error
}

// StatusOK asserts no error and a status code 200.
func (tc *CollectTestCase) StatusOK() *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Equal(tc.t, http.StatusOK, tc.res.StatusCode)
	}
	return tc
}

// StatusCode asserts no error and a status code.
func (tc *CollectTestCase) StatusCode(statusCode int) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Equal(tc.t, statusCode, tc.res.StatusCode)
	}
	return tc
}

// BodyContains asserts no error and that the response body contains the string or byte array value.
func (tc *CollectTestCase) BodyContains(value any) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		switch v := value.(type) {
		case []byte:
			testarossa.True(tc.t, bytes.Contains(body, v), "%v does not contain %v", body, v)
		case string:
			testarossa.Contains(tc.t, string(body), v)
		default:
			vv := utils.AnyToString(v)
			testarossa.Contains(tc.t, string(body), vv)
		}
	}
	return tc
}

// BodyNotContains asserts no error and that the response body does not contain the string or byte array value.
func (tc *CollectTestCase) BodyNotContains(value any) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		switch v := value.(type) {
		case []byte:
			testarossa.False(tc.t, bytes.Contains(body, v), "%v contains %v", body, v)
		case string:
			testarossa.NotContains(tc.t, string(body), v)
		default:
			vv := utils.AnyToString(v)
			testarossa.NotContains(tc.t, string(body), vv)
		}
	}
	return tc
}

// BodyMatchesRegexp asserts no error and that the response body matches a regexp pattern.
func (tc *CollectTestCase) BodyMatchesRegexp(pattern string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		re := regexp.MustCompile(pattern)
		testarossa.True(tc.t, re.MatchString(string(body)), "%v does not match pattern %v", string(body), pattern)
	}
	return tc
}

// HeaderContains asserts no error and that the named header contains the value.
func (tc *CollectTestCase) HeaderContains(headerName string, value string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Contains(tc.t, tc.res.Header.Get(headerName), value)
	}
	return tc
}

// HeaderNotContains asserts no error and that the named header does not contain a string.
func (tc *CollectTestCase) HeaderNotContains(headerName string, value string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.NotContains(tc.t, tc.res.Header.Get(headerName), value)
	}
	return tc
}

// HeaderEqual asserts no error and that the named header matches the value.
func (tc *CollectTestCase) HeaderEqual(headerName string, value string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Equal(tc.t, value, tc.res.Header.Get(headerName))
	}
	return tc
}

// HeaderNotEqual asserts no error and that the named header does not matche the value.
func (tc *CollectTestCase) HeaderNotEqual(headerName string, value string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.NotEqual(tc.t, value, tc.res.Header.Get(headerName))
	}
	return tc
}

// HeaderExists asserts no error and that the named header exists.
func (tc *CollectTestCase) HeaderExists(headerName string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.NotEqual(tc.t, 0, len(tc.res.Header.Values(headerName)), "Header %s does not exist", headerName)
	}
	return tc
}

// HeaderNotExists asserts no error and that the named header does not exists.
func (tc *CollectTestCase) HeaderNotExists(headerName string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Len(tc.t, tc.res.Header.Values(headerName), 0, "Header %s exists", headerName)
	}
	return tc
}

// ContentType asserts no error and that the Content-Type header matches the expected value.
func (tc *CollectTestCase) ContentType(expected string) *CollectTestCase {
	if testarossa.NoError(tc.t, tc.err) {
		testarossa.Equal(tc.t, expected, tc.res.Header.Get("Content-Type"))
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
	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		testarossa.NotEqual(tc.t, 0, len(matches), "Found no tags matching %s", cssSelectorQuery)
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
	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		testarossa.Len(tc.t, matches, 0, "Found %d tag(s) matching %s", len(matches), cssSelectorQuery)
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

	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if !testarossa.NotEqual(tc.t, 0, len(matches), "Selector %s does not match any tags", cssSelectorQuery) {
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
		testarossa.True(tc.t, found, "No tag matching %s contains %s", cssSelectorQuery, value)
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

	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if !testarossa.NotEqual(tc.t, 0, len(matches), "Selector %s does not match any tags", cssSelectorQuery) {
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
		testarossa.True(tc.t, found, "No tag matching %s contains %s", cssSelectorQuery, value)
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

	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if len(matches) == 0 {
			return tc
		}
		if !testarossa.NotEqual(tc.t, "", value, "Found tag matching %s", cssSelectorQuery) {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		testarossa.False(tc.t, found, "Found tag matching %s that contains %s", cssSelectorQuery, value)
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

	if testarossa.NoError(tc.t, tc.err) {
		selector, err := cascadia.Compile(cssSelectorQuery)
		if !testarossa.NoError(tc.t, err, "Invalid selector %s", cssSelectorQuery) {
			return tc
		}
		var body []byte
		if br, ok := tc.res.Body.(*httpx.BodyReader); ok {
			body = br.Bytes()
		} else {
			var err error
			body, err = io.ReadAll(tc.res.Body)
			if !testarossa.NoError(tc.t, err, "Failed to read body") {
				return tc
			}
			tc.res.Body = io.NopCloser(bytes.NewReader(body))
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if !testarossa.NoError(tc.t, err, "Failed to parse HTML") {
			return tc
		}
		matches := selector.MatchAll(doc)
		if len(matches) == 0 {
			return tc
		}
		if !testarossa.NotEqual(tc.t, "", value, "Found tag matching %s", cssSelectorQuery) {
			return tc
		}
		found := false
		for _, match := range matches {
			if textMatches(match) {
				found = true
				break
			}
		}
		testarossa.False(tc.t, found, "Found tag matching %s that contains %s", cssSelectorQuery, value)
	}
	return tc
}

// Error asserts an error.
func (tc *CollectTestCase) Error(errContains string) *CollectTestCase {
	if testarossa.Error(tc.t, tc.err) {
		testarossa.Contains(tc.t, tc.err.Error(), errContains)
	}
	return tc
}

// ErrorCode asserts an error by its status code.
func (tc *CollectTestCase) ErrorCode(statusCode int) *CollectTestCase {
	if testarossa.Error(tc.t, tc.err) {
		testarossa.Equal(tc.t, statusCode, errors.Convert(tc.err).StatusCode)
	}
	return tc
}

// NoError asserts no error.
func (tc *CollectTestCase) NoError() *CollectTestCase {
	testarossa.NoError(tc.t, tc.err)
	return tc
}

// CompletedIn checks that the duration of the operation is less than or equal the threshold.
func (tc *CollectTestCase) CompletedIn(threshold time.Duration) *CollectTestCase {
	testarossa.True(tc.t, tc.dur <= threshold)
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
Collect_Get performs a GET request to the Collect endpoint.

Collect returns the latest aggregated metrics.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func Collect_Get(t *testing.T, ctx context.Context, url string) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	url, err = httpx.ResolveURL(metricsapi.URLOfCollect, url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	ctx = frame.CloneContext(ctx)
	r, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r.Header = frame.Of(ctx).Header()
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = errors.CatchPanic(func() error {
		return Svc.Collect(w, r)
	})
	tc.dur = time.Since(t0)
	tc.res = w.Result()
	return tc
}

/*
Collect_Post performs a POST request to the Collect endpoint.

Collect returns the latest aggregated metrics.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func Collect_Post(t *testing.T, ctx context.Context, url string, contentType string, body any) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	url, err = httpx.ResolveURL(metricsapi.URLOfCollect, url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	ctx = frame.CloneContext(ctx)
	r, err := httpx.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r.Header = frame.Of(ctx).Header()
	err = httpx.SetRequestBody(r, body)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = errors.CatchPanic(func() error {
		return Svc.Collect(w, r)
	})
	tc.dur = time.Since(t0)
	tc.res = w.Result()
	return tc
}

/*
Collect returns the latest aggregated metrics.

If a request is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func Collect(t *testing.T, r *http.Request) *CollectTestCase {
	tc := &CollectTestCase{t: t}
	var err error
	if r == nil {
		r, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			tc.err = errors.Trace(err)
			return tc
		}
	}
	url, err := httpx.ResolveURL(metricsapi.URLOfCollect, r.URL.String())
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	r.URL, err = httpx.ParseURL(url)
	if err != nil {
		tc.err = errors.Trace(err)
		return tc
	}
	for k, vv := range frame.Of(r.Context()).Header() {
		r.Header[k] = vv
	}
	ctx := frame.ContextWithFrameOf(r.Context(), r.Header)
	r = r.WithContext(ctx)
	r.Header = frame.Of(ctx).Header()
	w := httpx.NewResponseRecorder()
	t0 := time.Now()
	tc.err = errors.CatchPanic(func() error {
		return Svc.Collect(w, r)
	})
	tc.res = w.Result()
	tc.dur = time.Since(t0)
	return tc
}
