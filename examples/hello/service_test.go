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

package hello

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/hello/helloapi"
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
	_ helloapi.Client
)

func TestHello_OpenAPI(t *testing.T) {
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
				pub.GET(httpx.JoinHostAndPath(helloapi.Hostname, ":"+port+"/openapi.json")),
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

func TestHello_Mock(t *testing.T) {
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

	t.Run("hello", func(t *testing.T) { // MARKER: Hello
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Hello(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockHello(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Hello(w, r)
		assert.NoError(err)
	})

	t.Run("echo", func(t *testing.T) { // MARKER: Echo
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Echo(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockEcho(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Echo(w, r)
		assert.NoError(err)
	})

	t.Run("ping", func(t *testing.T) { // MARKER: Ping
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Ping(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockPing(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Ping(w, r)
		assert.NoError(err)
	})

	t.Run("calculator", func(t *testing.T) { // MARKER: Calculator
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Calculator(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockCalculator(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Calculator(w, r)
		assert.NoError(err)
	})

	t.Run("bus_png", func(t *testing.T) { // MARKER: BusPNG
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.BusPNG(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockBusPNG(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.BusPNG(w, r)
		assert.NoError(err)
	})

	t.Run("localization", func(t *testing.T) { // MARKER: Localization
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Localization(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockLocalization(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Localization(w, r)
		assert.NoError(err)
	})

	t.Run("root", func(t *testing.T) { // MARKER: Root
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Root(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockRoot(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Root(w, r)
		assert.NoError(err)
	})

	t.Run("tick_tock", func(t *testing.T) { // MARKER: TickTock
		assert := testarossa.For(t)

		err := mock.TickTock(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockTickTock(func(ctx context.Context) (err error) {
			return nil
		})
		err = mock.TickTock(ctx)
		assert.NoError(err)
	})
}

func TestHello_Hello(t *testing.T) { // MARKER: Hello
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()
	svc.SetGreeting("Ciao")
	svc.SetRepeat(5)

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("default_greeting", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Hello(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.Contains(body, svc.Greeting())
				assert.NotContains(body, "Maria")
				// Should contain the greeting 5 times
				assert.Equal(5, bytes.Count(body, []byte(svc.Greeting())))
			}
		}
	})

	t.Run("personalized_greeting", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Hello(ctx, "GET", "?name=Maria", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.Contains(body, svc.Greeting())
				assert.Contains(body, "Maria")
				// Should contain the greeting 5 times
				assert.Equal(5, bytes.Count(body, []byte(svc.Greeting())))
			}
		}
	})
}

func TestHello_Echo(t *testing.T) { // MARKER: Echo
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("nil_request", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Echo(ctx, "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "POST /echo "))
			}
		}
	})

	t.Run("patch_with_headers_and_body", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("X-Location", "California"),
			).
			Echo(ctx, "PATCH", "", strings.NewReader("Sunshine"))
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "PATCH /echo "))
				assert.Contains(body, "\r\nX-Location: California")
				assert.Contains(body, "\r\nSunshine")
			}
		}
	})

	t.Run("get_with_no_url", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "GET /echo "))
			}
		}
	})

	t.Run("get_with_query_string", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "?arg=12345", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("get_with_relative_url", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "/echo?arg=12345", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("get_with_absolute_url", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "https://"+svc.Hostname()+"/echo?arg=12345", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("post_with_form_data", func(t *testing.T) {
		assert := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.Echo(ctx, "POST", "", formData)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "POST /echo "))
				assert.Contains(body, "\r\nload=22222&pay=11111")
				assert.Contains(body, "\r\nContent-Type: application/x-www-form-urlencoded")
			}
		}
	})

	t.Run("post_with_query_and_form_data", func(t *testing.T) {
		assert := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.Echo(ctx, "POST", "?arg=12345", formData)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "POST /echo?arg=12345 "))
				assert.Contains(body, "\r\nload=22222&pay=11111")
				assert.Contains(body, "\r\nContent-Type: application/x-www-form-urlencoded")
			}
		}
	})

	t.Run("post_with_custom_content_type", func(t *testing.T) {
		assert := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.
			WithOptions(pub.ContentType("text/plain")).
			Echo(ctx, "POST", "", formData)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.True(strings.HasPrefix(string(body), "POST /echo "))
				assert.Contains(body, "\r\nload=22222&pay=11111")
				assert.Contains(body, "\r\nContent-Type: text/plain")
			}
		}
	})

	t.Run("post_with_multiple_headers", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.AddHeader("Echo123", "EchoEchoEcho"),
				pub.AddHeader("Echo123", "WhoaWhoaWhoa"),
			).
			Echo(ctx, "POST", "?echo=123", strings.NewReader("PostBody"))
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				assert.Contains(body, "Echo123: EchoEchoEcho")
				assert.Contains(body, "Echo123: WhoaWhoaWhoa")
				assert.Contains(body, "?echo=123")
				assert.Contains(body, "PostBody")
			}
		}
	})
}

func TestHello_Ping(t *testing.T) { // MARKER: Ping
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("ping_returns_service_id", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Ping(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/plain", res.Header.Get("Content-Type"))
				// Should contain the service ID and hostname
				assert.Contains(body, svc.ID()+"."+svc.Hostname())
			}
		}
	})
}

func TestHello_Calculator(t *testing.T) { // MARKER: Calculator
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		calculator.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("addition", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Calculator(ctx, "GET", "?x=500&op=+&y=580", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/html", res.Header.Get("Content-Type"))
				assert.HTMLMatch(body, `TD#result`, "1080")
				assert.HTMLMatch(body, `INPUT[name="x"]`, "")
				assert.HTMLMatch(body, `SELECT[name="op"]`, "")
				assert.HTMLMatch(body, `INPUT[name="y"]`, "")
			}
		}
	})

	t.Run("multiplication", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Calculator(ctx, "GET", "?x=5&op=*&y=80", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/html", res.Header.Get("Content-Type"))
				assert.HTMLMatch(body, `TD#result`, "400")
			}
		}
	})

	t.Run("division", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(pub.ContentType("application/x-www-form-urlencoded")).
			Calculator(ctx, "POST", "", strings.NewReader("x=500&op=/&y=5"))
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("text/html", res.Header.Get("Content-Type"))
				assert.HTMLMatch(body, `TD#result`, "100")
			}
		}
	})
}

func TestHello_BusPNG(t *testing.T) { // MARKER: BusPNG
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("serve_image", func(t *testing.T) {
		assert := testarossa.For(t)

		// Read the expected image
		img, err := svc.ReadResFile("bus.png")
		assert.NoError(err)

		res, err := client.BusPNG(ctx, "")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("image/png", res.Header.Get("Content-Type"))
				assert.Equal(body, img)
			}
		}
	})
}

func TestHello_Localization(t *testing.T) { // MARKER: Localization
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("default_english", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Localization(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Hello")
			}
		}
	})

	t.Run("english_locale", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "en"),
			).
			Localization(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Hello")
			}
		}
	})

	t.Run("kiwi_english_locale", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "en-NZ"),
			).
			Localization(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Hello")
			}
		}
	})

	t.Run("italian_locale", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "it"),
			).
			Localization(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Salve")
			}
		}
	})
}

func TestHello_Root(t *testing.T) { // MARKER: Root
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("root_page", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Root(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.HTMLMatch(body, "HTML", "")
				assert.HTMLMatch(body, "BODY", "")
				assert.HTMLMatch(body, "H1", "Microbus")
			}
		}
	})
}

func TestHello_TickTock(t *testing.T) { // MARKER: TickTock
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	t.Run("ticktock_runs", func(t *testing.T) {
		assert := testarossa.For(t)

		err := svc.TickTock(ctx)
		assert.NoError(err)
	})
}
