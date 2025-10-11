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

package hello

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/hello/helloapi"
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
	_ *helloapi.Client
)

func TestHello_Hello(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()
	svc.SetGreeting("Ciao")
	svc.SetRepeat(5)

	// Initialize the testers
	tester := connector.New("hello.hello.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("default_greeting", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Hello(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.Contains(body, svc.Greeting())
				tt.NotContains(body, "Maria")
				// Should contain the greeting 5 times
				tt.Equal(5, bytes.Count(body, []byte(svc.Greeting())))
			}
		}
	})

	t.Run("personalized_greeting", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Hello(ctx, "GET", "?name=Maria", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.Contains(body, svc.Greeting())
				tt.Contains(body, "Maria")
				// Should contain the greeting 5 times
				tt.Equal(5, bytes.Count(body, []byte(svc.Greeting())))
			}
		}
	})
}

func TestHello_Echo(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.echo.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("nil_request", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Echo(ctx, "", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "POST /echo "))
			}
		}
	})

	t.Run("patch_with_headers_and_body", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("X-Location", "California"),
			).
			Echo(ctx, "PATCH", "", "", strings.NewReader("Sunshine"))
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "PATCH /echo "))
				tt.Contains(body, "\r\nX-Location: California")
				tt.Contains(body, "\r\nSunshine")
			}
		}
	})

	t.Run("get_with_no_url", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "GET /echo "))
			}
		}
	})

	t.Run("get_with_query_string", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "?arg=12345", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("get_with_relative_url", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "/echo?arg=12345", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("get_with_absolute_url", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Echo(ctx, "GET", "https://"+svc.Hostname()+"/echo?arg=12345", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "GET /echo?arg=12345 "))
			}
		}
	})

	t.Run("post_with_form_data", func(t *testing.T) {
		tt := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.Echo(ctx, "POST", "", "", formData)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "POST /echo "))
				tt.Contains(body, "\r\nload=22222&pay=11111")
				tt.Contains(body, "\r\nContent-Type: application/x-www-form-urlencoded")
			}
		}
	})

	t.Run("post_with_query_and_form_data", func(t *testing.T) {
		tt := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.Echo(ctx, "POST", "?arg=12345", "", formData)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "POST /echo?arg=12345 "))
				tt.Contains(body, "\r\nload=22222&pay=11111")
				tt.Contains(body, "\r\nContent-Type: application/x-www-form-urlencoded")
			}
		}
	})

	t.Run("post_with_custom_content_type", func(t *testing.T) {
		tt := testarossa.For(t)

		formData := url.Values{
			"pay":  []string{"11111"},
			"load": []string{"22222"},
		}
		res, err := client.Echo(ctx, "POST", "", "text/plain", formData)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.True(strings.HasPrefix(string(body), "POST /echo "))
				tt.Contains(body, "\r\nload=22222&pay=11111")
				tt.Contains(body, "\r\nContent-Type: text/plain")
			}
		}
	})

	t.Run("post_with_multiple_headers", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.AddHeader("Echo123", "EchoEchoEcho"),
				pub.AddHeader("Echo123", "WhoaWhoaWhoa"),
			).
			Echo(ctx, "POST", "?echo=123", "", strings.NewReader("PostBody"))
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				tt.Contains(body, "Echo123: EchoEchoEcho")
				tt.Contains(body, "Echo123: WhoaWhoaWhoa")
				tt.Contains(body, "?echo=123")
				tt.Contains(body, "PostBody")
			}
		}
	})
}

func TestHello_Ping(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.ping.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("ping_returns_service_id", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Ping(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/plain", res.Header.Get("Content-Type"))
				// Should contain the service ID and hostname
				tt.Contains(body, svc.ID()+"."+svc.Hostname())
			}
		}
	})
}

func TestHello_Calculator(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.calculator.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		calculator.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("addition", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Calculator(ctx, "GET", "?x=500&op=+&y=580", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/html", res.Header.Get("Content-Type"))
				tt.HTMLMatch(body, `TD#result`, "1080")
				tt.HTMLMatch(body, `INPUT[name="x"]`, "")
				tt.HTMLMatch(body, `SELECT[name="op"]`, "")
				tt.HTMLMatch(body, `INPUT[name="y"]`, "")
			}
		}
	})

	t.Run("multiplication", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Calculator(ctx, "GET", "?x=5&op=*&y=80", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/html", res.Header.Get("Content-Type"))
				tt.HTMLMatch(body, `TD#result`, "400")
			}
		}
	})

	t.Run("division", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Calculator(ctx, "POST", "", "application/x-www-form-urlencoded", strings.NewReader("x=500&op=/&y=5"))
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("text/html", res.Header.Get("Content-Type"))
				tt.HTMLMatch(body, `TD#result`, "100")
			}
		}
	})
}

func TestHello_BusPNG(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.buspng.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("serve_image", func(t *testing.T) {
		tt := testarossa.For(t)

		// Read the expected image
		img, err := svc.ReadResFile("bus.png")
		tt.NoError(err)

		res, err := client.BusPNG(ctx, "")
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("image/png", res.Header.Get("Content-Type"))
				tt.Equal(body, img)
			}
		}
	})
}

func TestHello_Localization(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.localization.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("default_english", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Localization(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Contains(body, "Hello")
			}
		}
	})

	t.Run("english_locale", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "en"),
			).
			Localization(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Contains(body, "Hello")
			}
		}
	})

	t.Run("kiwi_english_locale", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "en-NZ"),
			).
			Localization(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Contains(body, "Hello")
			}
		}
	})

	t.Run("italian_locale", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.
			WithOptions(
				pub.Header("Accept-Language", "it"),
			).
			Localization(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Contains(body, "Salve")
			}
		}
	})
}

func TestHello_Root(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("hello.root.tester")
	client := helloapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("root_page", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := client.Root(ctx, "GET", "", "", nil)
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.HTMLMatch(body, "HTML", "")
				tt.HTMLMatch(body, "BODY", "")
				tt.HTMLMatch(body, "H1", "Microbus")
			}
		}
	})
}

func TestHello_TickTock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	t.Run("ticktock_runs", func(t *testing.T) {
		tt := testarossa.For(t)

		err := svc.TickTock(ctx)
		tt.NoError(err)
	})
}
