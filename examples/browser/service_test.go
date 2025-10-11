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

package browser

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/browser/browserapi"
)

var (
	_ context.Context
	_ io.Closer
	_ http.Handler
	_ testing.TB
	_ *application.Application
	_ *connector.Connector
	_ *frame.Frame
	_ pub.Option
	_ testarossa.TestingT
	_ *browserapi.Client
)

func TestBrowser_Browse(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("browser.browse.tester")
	client := browserapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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
		tt := testarossa.For(t)

		// Load the browser without a URL
		res, err := client.Browse(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				// Check basic HTML structure
				tt.HTMLMatch(body, "HTML", "")
				tt.HTMLMatch(body, "HEAD", "")
				tt.HTMLMatch(body, "BODY", "")

				// Check form structure
				tt.HTMLMatch(body, "FORM", "")
				tt.HTMLMatch(body, `FORM[method="GET"]`, "")
				tt.HTMLMatch(body, `FORM[action="browse"]`, "")

				// Check input elements
				tt.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")
				tt.HTMLMatch(body, `INPUT[type="submit"]`, "")

				// Verify no PRE element (no content fetched yet)
				tt.HTMLNotMatch(body, "PRE", "")
			}
		}
	})

	t.Run("fetch_url_with_content", func(t *testing.T) {
		tt := testarossa.For(t)

		// Load the browser with a URL
		res, err := client.Browse(ctx, "GET", "?url=https://lorem.ipsum/", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				// Check basic HTML structure
				tt.HTMLMatch(body, "HTML", "")
				tt.HTMLMatch(body, "BODY", "")

				// Check form with populated URL
				tt.HTMLMatch(body, "FORM", "")
				tt.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")

				// Check that the source code is displayed in a PRE element
				tt.HTMLMatch(body, "PRE", "")
				tt.HTMLMatch(body, `PRE`, `<html>`)
				tt.HTMLMatch(body, `PRE`, `Lorem Ipsum`)
				tt.HTMLMatch(body, `PRE`, `</html>`)

				// Verify the entire expected content appears
				expectedContent := `<html><body>Lorem Ipsum<body></html>`
				tt.HTMLMatch(body, `PRE`, "^"+regexp.QuoteMeta(expectedContent)+"$")
			}
		}
	})

	t.Run("url_without_scheme", func(t *testing.T) {
		tt := testarossa.For(t)

		// Load the browser with a URL without scheme (should be prefixed with https://)
		res, err := client.Browse(ctx, "GET", "?url=lorem.ipsum", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				// Check that the form shows the input value
				tt.HTMLMatch(body, `INPUT[type="text"][name="url"]`, "")

				// Check that content was fetched and displayed
				tt.HTMLMatch(body, "PRE", "")
				tt.HTMLMatch(body, `PRE`, `Lorem Ipsum`)
			}
		}
	})
}
