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

package browser

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/browser/browserapi"
)

var (
	_ context.Context
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ testarossa.Asserter
	_ browserapi.Client
)

func TestBrowser_OpenAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	ports := []string{
		// HINT: Include all ports of functional or web endpoints
		"443",
	}
	for _, port := range ports {
		t.Run("port_"+port, func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := tester.Request(
				ctx,
				pub.GET(httpx.JoinHostAndPath(browserapi.Hostname, ":"+port+"/openapi.json")),
			)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.Contains(body, "openapi")
				}
			}
		})
	}
}

func TestBrowser_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("browse", func(t *testing.T) { // MARKER: Browse
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Browse(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockBrowse(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Browse(w, r)
		assert.NoError(err)
	})
}

func TestBrowser_Browse(t *testing.T) { // MARKER: Browse
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := browserapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		httpegress.NewMock().
			MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
				req, _ := http.ReadRequest(bufio.NewReader(r.Body))
				if req.Method == "GET" && req.URL.String() == "https://lorem.ipsum/" {
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte(`<html><body>Lorem Ipsum<body></html>`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
				return nil
			}),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("empty_initial_load", func(t *testing.T) {
		assert := testarossa.For(t)

		// Load the browser without a URL
		res, err := client.Browse(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check basic HTML structure
				assert.HTMLMatch(body, "HTML", "")
				assert.HTMLMatch(body, "HEAD", "")
				assert.HTMLMatch(body, "BODY", "")

				// Check form structure
				assert.HTMLMatch(body, "FORM", "")
				assert.HTMLMatch(body, `FORM[method="GET"]`, "")
				assert.HTMLMatch(body, `FORM[action="browse"]`, "")

				// Check input elements
				assert.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")
				assert.HTMLMatch(body, `INPUT[type="submit"]`, "")

				// Verify no PRE element (no content fetched yet)
				assert.HTMLNotMatch(body, "PRE", "")
			}
		}
	})

	t.Run("fetch_url_with_content", func(t *testing.T) {
		assert := testarossa.For(t)

		// Load the browser with a URL
		res, err := client.Browse(ctx, "GET", "?url=https://lorem.ipsum/", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check basic HTML structure
				assert.HTMLMatch(body, "HTML", "")
				assert.HTMLMatch(body, "BODY", "")

				// Check form with populated URL
				assert.HTMLMatch(body, "FORM", "")
				assert.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")

				// Check that the source code is displayed in a PRE element
				assert.HTMLMatch(body, "PRE", "")
				assert.HTMLMatch(body, `PRE`, `<html>`)
				assert.HTMLMatch(body, `PRE`, `Lorem Ipsum`)
				assert.HTMLMatch(body, `PRE`, `</html>`)

				// Verify the entire expected content appears
				expectedContent := `<html><body>Lorem Ipsum<body></html>`
				assert.HTMLMatch(body, `PRE`, "^"+regexp.QuoteMeta(expectedContent)+"$")
			}
		}
	})

	t.Run("url_without_scheme", func(t *testing.T) {
		assert := testarossa.For(t)

		// Load the browser with a URL without scheme (should be prefixed with https://)
		res, err := client.Browse(ctx, "GET", "?url=lorem.ipsum", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check that the form shows the input value
				assert.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")

				// Check that content was fetched and displayed
				assert.HTMLMatch(body, "PRE", "")
				assert.HTMLMatch(body, `PRE`, `Lorem Ipsum`)
			}
		}
	})
}
