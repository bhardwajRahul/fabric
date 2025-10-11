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

package httpegress

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
)

func TestHttpegress_MakeRequest(t *testing.T) {
	// No t.Parallel: starting a web server
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("httpegress.makerequest.tester")
	client := httpegressapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Start a standard web server
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		r.Write(w)
	})
	http.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
	})
	httpServer := &http.Server{
		Addr: "127.0.0.1:5050",
	}
	go func() {
		httpServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		httpServer.Shutdown(context.Background())
	})
	time.Sleep(200 * time.Millisecond) // Give enough time for web server to start

	t.Run("get", func(t *testing.T) {
		tt := testarossa.For(t)

		// Echo
		resp, err := client.Get(ctx, "http://127.0.0.1:5050/echo")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, resp.StatusCode)
			raw, _ := io.ReadAll(resp.Body)
			tt.Contains(string(raw), "GET /echo HTTP/1.1\r\n")
			tt.Contains(string(raw), "Host: 127.0.0.1:5050\r\n")
			tt.Contains(string(raw), "User-Agent: Go-http-client")
		}

		// Not found
		resp, err = client.Get(ctx, "http://127.0.0.1:5050/x")
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, resp.StatusCode)
		}

		// Bad URL
		_, err = client.Get(ctx, "not a url")
		tt.Error(err)

		// Shorter deadline
		shortCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err = client.Get(shortCtx, "http://127.0.0.1:5050/slow")
		cancel()
		if tt.Error(err) {
			tt.Contains(err.Error(), "timeout")
		}
	})

	t.Run("post", func(t *testing.T) {
		tt := testarossa.For(t)

		// Echo
		resp, err := client.Post(ctx, "http://127.0.0.1:5050/echo", "text/plain", strings.NewReader("Lorem Ipsum Dolor Sit Amet"))
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, resp.StatusCode)
			raw, _ := io.ReadAll(resp.Body)
			tt.Contains(string(raw), "POST /echo HTTP/1.1\r\n")
			tt.Contains(string(raw), "Host: 127.0.0.1:5050\r\n")
			tt.Contains(string(raw), "User-Agent: Go-http-client")
			tt.Contains(string(raw), "Content-Type: text/plain\r\n")
			tt.Contains(string(raw), "Lorem Ipsum Dolor Sit Amet")
		}

		// Not found
		resp, err = client.Post(ctx, "http://127.0.0.1:5050/x", "", strings.NewReader("nothing"))
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, resp.StatusCode)
		}

		// Bad URL
		_, err = client.Post(ctx, "not a url", "", strings.NewReader("nothing"))
		tt.Error(err)
	})

	t.Run("do", func(t *testing.T) {
		tt := testarossa.For(t)

		// Echo
		req, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:5050/echo", bytes.NewReader([]byte("Lorem Ipsum")))
		req.Header["Multi-Value"] = []string{"Foo", "Bar"}
		tt.NoError(err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(ctx, req)
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, resp.StatusCode)
			raw, _ := io.ReadAll(resp.Body)
			tt.Contains(string(raw), "PUT /echo HTTP/1.1\r\n")
			tt.Contains(string(raw), "Host: 127.0.0.1:5050\r\n")
			tt.Contains(string(raw), "User-Agent: Go-http-client")
			tt.Contains(string(raw), "Content-Type: text/plain\r\n")
			tt.Contains(string(raw), "Multi-Value: Foo\r\n")
			tt.Contains(string(raw), "Multi-Value: Bar\r\n")
			tt.Contains(string(raw), "\r\n\r\nLorem Ipsum")
		}

		// Not found
		req, err = http.NewRequest(http.MethodPatch, "http://127.0.0.1:5050/x", bytes.NewReader([]byte("Lorem Ipsum")))
		tt.NoError(err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err = client.Do(ctx, req)
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, resp.StatusCode)
		}

		// Bad URL
		req, err = http.NewRequest(http.MethodDelete, "not a url", nil)
		tt.NoError(err)
		req.Header.Set("Content-Type", "text/plain")

		_, err = client.Do(ctx, req)
		tt.Error(err)
	})
}

func TestHttpegress_Mocked(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	tt := testarossa.For(t)

	// Initialize the mocked microservice
	mock := NewMock().
		MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if req.Method == "DELETE" && req.URL.String() == "https://example.com/ex/5" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"deleted":true}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})

	// Initialize the testers
	tester := connector.New("httpegress.mocked.tester")
	client := httpegressapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		mock,
		tester,
	)
	app.RunInTest(t)

	req, err := http.NewRequest("DELETE", "https://example.com/ex/5", nil)
	tt.NoError(err)
	resp, err := client.Do(ctx, req)
	if tt.NoError(err) && tt.Equal(http.StatusOK, resp.StatusCode) {
		tt.Equal("application/json", resp.Header.Get("Content-Type"))
		raw, err := io.ReadAll(resp.Body)
		if tt.NoError(err) {
			tt.Equal(string(raw), `{"deleted":true}`)
		}
	}
}
