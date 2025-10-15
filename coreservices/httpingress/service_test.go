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

package httpingress

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/httpingress/middleware"
	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
	"github.com/microbus-io/fabric/coreservices/tokenissuer"
	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
)

func TestHttpingress_Incoming(t *testing.T) {
	// No t.Parallel: starting a web server
	ctx := t.Context()

	entered := make(chan bool)
	done := make(chan bool)
	callCount := 0
	var request *http.Request
	countActors := 0

	// Initialize the microservice under test
	svc := NewService()
	svc.SetTimeBudget(time.Second * 2)
	svc.SetPorts("4040,40443")
	svc.SetAllowedOrigins("allowed.origin")
	svc.SetPortMappings("4040:*->*, 40443:*->443")
	svc.Middleware().Append("HelloGoodbye", middleware.OnRoutePrefix("/greeting:555/", middleware.Group(
		func(next connector.HTTPHandler) connector.HTTPHandler {
			return func(w http.ResponseWriter, r *http.Request) (err error) {
				r.Header.Add("Middleware", "Hello")
				return next(w, r) // No trace
			}
		},
		func(next connector.HTTPHandler) connector.HTTPHandler {
			return func(w http.ResponseWriter, r *http.Request) (err error) {
				err = next(w, r)
				w.Header().Add("Middleware", "Goodbye")
				return err // No trace
			}
		},
	)))
	svc.Middleware().Append("401Redirect", middleware.ErrorPageRedirect(http.StatusUnauthorized, "/login-page"))

	// Initialize the testers
	tester := connector.New("metrics.collect.tester")
	client := metricsapi.NewClient(tester)
	_ = client
	httpClient := http.Client{Timeout: time.Second * 4}

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
		connector.New("ports").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("request.memory.limit").Init(func(c *connector.Connector) {
			c.Subscribe("POST", "ok", func(w http.ResponseWriter, r *http.Request) error {
				b, _ := io.ReadAll(r.Body)
				w.Write(b)
				return nil
			})
			c.Subscribe("POST", "hold", func(w http.ResponseWriter, r *http.Request) error {
				entered <- true
				<-done
				w.Write([]byte("done"))
				return nil
			})
		}),
		connector.New("compression").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("Content-Type", "text/plain")
				w.Write(bytes.Repeat([]byte("Hello123"), 1024)) // 8KB
				return nil
			})
		}),
		connector.New("port.mapping").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok443", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
			c.Subscribe("GET", ":555/ok555", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("forwarded.headers").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				var sb strings.Builder
				for _, h := range []string{"X-Forwarded-Host", "X-Forwarded-Prefix", "X-Forwarded-Proto", "X-Forwarded-For", "X-Forwarded-Path"} {
					if r.Header.Get(h) != "" {
						sb.WriteString(h)
						sb.WriteString(": ")
						sb.WriteString(r.Header.Get(h))
						sb.WriteString("\n")
					}
				}
				w.Write([]byte(sb.String()))
				return nil
			})
		}),
		connector.New("root").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("Root"))
				return nil
			})
		}),
		connector.New("cors").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				callCount++
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("parse.form").Init(func(c *connector.Connector) {
			c.Subscribe("POST", "ok", func(w http.ResponseWriter, r *http.Request) error {
				err := r.ParseForm()
				if err != nil {
					return errors.Trace(err)
				}
				w.Write([]byte("ok"))
				return nil
			})
			c.Subscribe("POST", "more", func(w http.ResponseWriter, r *http.Request) error {
				r.Body = http.MaxBytesReader(w, r.Body, 12*1024*1024)
				err := r.ParseForm()
				if err != nil {
					return errors.Trace(err)
				}
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("internal.headers").Init(func(c *connector.Connector) {
			c.Subscribe("GET", ":555/ok", func(w http.ResponseWriter, r *http.Request) error {
				request = r
				w.Header().Set(frame.HeaderPrefix+"In-Response", "STOP")
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("greeting").Init(func(c *connector.Connector) {
			c.Subscribe("GET", ":555/ok", func(w http.ResponseWriter, r *http.Request) error {
				request = r
				w.Write([]byte("ok"))
				return nil
			})
			c.Subscribe("GET", ":500/ok", func(w http.ResponseWriter, r *http.Request) error {
				request = r
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("blocked.paths").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "admin.php", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
			c.Subscribe("GET", "admin.ppp", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("no.cache").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("ok"))
				return nil
			})
		}),
		connector.New("auth.token.entry").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				if ok, _ := frame.Of(r).IfActor(`iss`); ok {
					countActors++
				}
				return nil
			})
		}),
		connector.New("authorization").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "protected", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("Access Granted"))
				return nil
			}, sub.Actor("role=='major'"))
			c.Subscribe("GET", "//login-page", func(w http.ResponseWriter, r *http.Request) error {
				w.Write([]byte("Login"))
				return nil
			})
		}),
		connector.New("multi.value.headers").Init(func(c *connector.Connector) {
			c.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
				request = r
				w.Header()["Multi-Value"] = []string{
					"Return 1",
					"Return 2",
				}
				return nil
			})
		}),
	)
	app.RunInTest(t)

	t.Run("ports", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := httpClient.Get("http://localhost:4040/ports/ok")
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
			}
		}
		res, err = httpClient.Get("http://localhost:40443/ports/ok")
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
			}
		}
	})

	t.Run("request_memory_limit", func(t *testing.T) {
		tt := testarossa.For(t)

		origLimit := svc.RequestMemoryLimit()
		svc.SetRequestMemoryLimit(1) // 1MB
		defer svc.SetRequestMemoryLimit(origLimit)

		// Small request at 25% of capacity
		tt.Zero(svc.reqMemoryUsed)
		payload := rand.AlphaNum64(svc.RequestMemoryLimit() * 1024 * 1024 / 4)
		res, err := httpClient.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal(payload, string(b))
			}
		}

		// Big request at 55% of capacity
		tt.Zero(svc.reqMemoryUsed)
		payload = rand.AlphaNum64(svc.RequestMemoryLimit() * 1024 * 1024 * 55 / 100)
		res, err = httpClient.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
		if tt.NoError(err) {
			tt.Equal(http.StatusRequestEntityTooLarge, res.StatusCode)
		}

		// Two small requests that together are over 50% of capacity
		tt.Zero(svc.reqMemoryUsed)
		payload = rand.AlphaNum64(svc.RequestMemoryLimit() * 1024 * 1024 / 3)
		returned := make(chan bool)
		go func() {
			res, err = httpClient.Post("http://localhost:4040/request.memory.limit/hold", "text/plain", strings.NewReader(payload))
			returned <- true
		}()
		<-entered
		tt.NotZero(svc.reqMemoryUsed)
		res, err = httpClient.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
		if tt.NoError(err) {
			tt.Equal(http.StatusRequestEntityTooLarge, res.StatusCode)
		}
		done <- true
		<-returned
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("done", string(b))
			}
		}

		tt.Zero(svc.reqMemoryUsed)
	})

	t.Run("compression", func(t *testing.T) {
		tt := testarossa.For(t)

		req, err := http.NewRequest("GET", "http://localhost:4040/compression/ok", nil)
		tt.NoError(err)
		req.Header.Set("Accept-Encoding", "gzip")
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal("gzip", res.Header.Get("Content-Encoding"))
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.True(len(b) < 8*1024)
			}
			tt.Equal(strconv.Itoa(len(b)), res.Header.Get("Content-Length"))
		}
	})

	t.Run("port_mapping", func(t *testing.T) {
		tt := testarossa.For(t)

		// External port 4040 grants access to all internal ports
		res, err := httpClient.Get("http://localhost:4040/port.mapping/ok443")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
		}
		res, err = httpClient.Get("http://localhost:4040/port.mapping:555/ok555")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
		}
		res, err = httpClient.Get("http://localhost:4040/port.mapping:555/ok443")
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, res.StatusCode)
		}

		// External port 40443 maps all requests to internal port 443
		res, err = httpClient.Get("http://localhost:40443/port.mapping/ok443")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
		}
		res, err = httpClient.Get("http://localhost:40443/port.mapping:555/ok555")
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, res.StatusCode)
		}
		res, err = httpClient.Get("http://localhost:40443/port.mapping:555/ok443")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
		}
	})

	t.Run("forwarded_headers", func(t *testing.T) {
		tt := testarossa.For(t)

		// Make a standard request
		req, err := http.NewRequest("GET", "http://localhost:4040/forwarded.headers/ok", nil)
		tt.NoError(err)
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				body := string(b)
				tt.True(strings.Contains(body, "X-Forwarded-Host: localhost:4040\n"))
				tt.False(strings.Contains(body, "X-Forwarded-Prefix:"))
				tt.True(strings.Contains(body, "X-Forwarded-Proto: http\n"))
				tt.True(strings.Contains(body, "X-Forwarded-For: "))
				tt.True(strings.Contains(body, "X-Forwarded-Path: /forwarded.headers/ok"))
			}
		}

		// Make a request appear to be coming through an upstream proxy server
		req, err = http.NewRequest("GET", "http://localhost:4040/forwarded.headers/ok", nil)
		tt.NoError(err)
		req.Header.Set("X-Forwarded-Host", "www.example.com")
		req.Header.Set("X-Forwarded-Prefix", "/app")
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		req.Header.Set("X-Forwarded-Proto", "https")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				body := string(b)
				tt.True(strings.Contains(body, "X-Forwarded-Host: www.example.com\n"))
				tt.True(strings.Contains(body, "X-Forwarded-Prefix: /app\n"))
				tt.True(strings.Contains(body, "X-Forwarded-Proto: https\n"))
				tt.True(strings.Contains(body, "X-Forwarded-For: 1.2.3.4"))
				tt.True(strings.Contains(body, "X-Forwarded-Path: /forwarded.headers/ok"))
			}
		}
	})

	t.Run("root", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := httpClient.Get("http://localhost:4040/")
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Expect(body, []byte("Root"))
			}
		}
	})

	t.Run("cors", func(t *testing.T) {
		tt := testarossa.For(t)

		// Request with no origin header
		count := callCount
		req, err := http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
		tt.NoError(err)
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
			tt.Equal(count+1, callCount)
		}

		// Request with disallowed origin header
		count = callCount
		req, err = http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
		tt.NoError(err)
		req.Header.Set("Origin", "disallowed.origin")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusForbidden, res.StatusCode)
			tt.Equal(count, callCount)
		}

		// Request with allowed origin header
		count = callCount
		req, err = http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
		tt.NoError(err)
		req.Header.Set("Origin", "allowed.origin")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
			tt.Equal("allowed.origin", res.Header.Get("Access-Control-Allow-Origin"))
			tt.Equal(count+1, callCount)
		}

		// Preflight request with allowed origin header
		count = callCount
		req, err = http.NewRequest("OPTIONS", "http://localhost:4040/cors/ok", nil)
		tt.NoError(err)
		req.Header.Set("Origin", "allowed.origin")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusNoContent, res.StatusCode)
			tt.Equal(count, callCount)
		}
	})

	t.Run("parse_form", func(t *testing.T) {
		tt := testarossa.For(t)

		// Under 10MB
		var buf bytes.Buffer
		buf.WriteString("x=")
		buf.WriteString(rand.AlphaNum64(9 * 1024 * 1024))
		res, err := httpClient.Post("http://localhost:4040/parse.form/ok", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
			}
		}

		// Go sets a 10MB limit on forms by default
		// https://go.dev/src/net/http/request.go#L1258
		buf.WriteString(rand.AlphaNum64(2 * 1024 * 1024)) // Now 11MB
		res, err = httpClient.Post("http://localhost:4040/parse.form/ok", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
		if tt.NoError(err) {
			tt.Equal(http.StatusRequestEntityTooLarge, res.StatusCode)
		}

		// MaxBytesReader can be used to extend the limit
		res, err = httpClient.Post("http://localhost:4040/parse.form/more", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
			}
		}

		// Going above the MaxBytesReader limit
		buf.WriteString(rand.AlphaNum64(2 * 1024 * 1024)) // Now 13MB
		res, err = httpClient.Post("http://localhost:4040/parse.form/more", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
		if tt.NoError(err) {
			tt.Equal(http.StatusRequestEntityTooLarge, res.StatusCode)
		}
	})

	t.Run("block_internal_headers", func(t *testing.T) {
		tt := testarossa.For(t)

		req, err := http.NewRequest("GET", "http://localhost:4040/internal.headers:555/ok", nil)
		tt.NoError(err)
		req.Header.Set(frame.HeaderPrefix+"In-Request", "STOP")
		req.Header.Set(strings.ToUpper(frame.HeaderPrefix)+"In-Request-Upper", "STOP")
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			// No Microbus headers should be accepted from client
			tt.Equal("", request.Header.Get(frame.HeaderPrefix+"In-Request"))
			tt.Equal("", request.Header.Get(strings.ToUpper(frame.HeaderPrefix+"In-Request-Upper")))
			// Microbus headers generated internally should pass through the middleware chain
			tt.Equal(Hostname, frame.Of(request).FromHost())

			// No Microbus headers should leak outside
			tt.Equal("", res.Header.Get(frame.HeaderPrefix+"In-Response"))
			tt.Equal("", res.Header.Get(strings.ToUpper(frame.HeaderPrefix+"In-Request-Upper")))
			for h := range res.Header {
				tt.False(strings.HasPrefix(h, frame.HeaderPrefix))
			}
		}
	})

	t.Run("on_route", func(t *testing.T) {
		tt := testarossa.For(t)

		req, err := http.NewRequest("GET", "http://localhost:4040/greeting:555/ok", nil)
		tt.NoError(err)
		req.Header.Set("Authorization", "Bearer 123456")
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
				// Headers should pass through
				tt.Equal("Bearer 123456", request.Header.Get("Authorization"))
				// Middleware added a request header
				tt.Equal("Hello", request.Header.Get("Middleware"))
				// Middleware added a response header
				tt.Equal("Goodbye", res.Header.Get("Middleware"))
			}
		}

		req, err = http.NewRequest("GET", "http://localhost:4040/greeting:500/ok", nil)
		tt.NoError(err)
		req.Header.Set("Authorization", "Bearer 123456")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.Equal("ok", string(b))
				// Headers should pass through
				tt.Equal("Bearer 123456", request.Header.Get("Authorization"))
				// Middleware did not run on this route
				tt.Equal("", request.Header.Get("Middleware"))
				tt.Equal("", res.Header.Get("Middleware"))
			}
		}
	})

	t.Run("blocked_paths", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := httpClient.Get("http://localhost:4040/blocked.paths/admin.php")
		if tt.NoError(err) {
			tt.Equal(http.StatusNotFound, res.StatusCode)
		}
		res, err = httpClient.Get("http://localhost:4040/blocked.paths/admin.ppp")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
		}
	})

	t.Run("default_fav_icon", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := httpClient.Get("http://localhost:4040/favicon.ico")
		if tt.NoError(err) {
			tt.Equal(http.StatusOK, res.StatusCode)
			tt.Equal("image/x-icon", res.Header.Get("Content-Type"))
			icon, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				tt.NotZero(len(icon))
			}
		}
	})

	t.Run("no_cache_response_headers", func(t *testing.T) {
		tt := testarossa.For(t)

		res, err := httpClient.Get("http://localhost:4040/no.cache/ok")
		if tt.NoError(err) {
			tt.Contains(res.Header.Get("Cache-Control"), "no-cache")
			tt.Contains(res.Header.Get("Cache-Control"), "no-store")
			tt.Contains(res.Header.Get("Cache-Control"), "max-age=0")
		}
	})

	t.Run("auth_token_entry", func(t *testing.T) {
		tt := testarossa.For(t)

		now := time.Now().Truncate(time.Second)

		req, err := http.NewRequest("GET", "http://localhost:4040/auth.token.entry/ok", nil)
		tt.NoError(err)

		// No token
		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(0, countActors)

		// Token by unknown issuer
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": "my.issuer",
			"iat": now.Unix(),
			"exp": now.Add(time.Hour).Unix(),
		})
		signedJWT, err := jwtToken.SignedString([]byte("some-key"))
		tt.NoError(err)
		req.Header.Set("Authorization", "Bearer "+signedJWT)

		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(0, countActors)

		// Attempt to impersonate issuer (wrong key)
		jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": tokenissuerapi.Hostname,
			"iat": now.Unix(),
			"exp": now.Add(time.Hour).Unix(),
		})
		signedJWT, err = jwtToken.SignedString([]byte("wrong-key"))
		tt.NoError(err)
		req.Header.Set("Authorization", "Bearer "+signedJWT)

		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(0, countActors)

		// Do not accept incoming Microbus-Actor header
		req.Header.Del("Authorization")
		req.Header.Set(frame.HeaderActor, `{"iss":"`+tokenissuerapi.Hostname+`"}`)

		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(0, countActors)

		// Valid as Authorization Bearer header
		signedJWT, err = tokenissuerapi.NewClient(tester).IssueToken(ctx, nil)
		tt.NoError(err)
		req.Header.Del(frame.HeaderActor)
		req.Header.Set("Authorization", "Bearer "+signedJWT)

		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(1, countActors)

		// Also in Authorization cookie
		req.Header.Del("Authorization")
		req.AddCookie(&http.Cookie{
			Name:     "Authorization",
			Value:    signedJWT,
			MaxAge:   60,
			HttpOnly: true,
			Secure:   false,
			Path:     "/",
		})

		_, err = httpClient.Do(req)
		tt.NoError(err)
		tt.Equal(2, countActors)
	})

	t.Run("authorization", func(t *testing.T) {
		tt := testarossa.For(t)

		req, err := http.NewRequest("GET", "http://localhost:4040/authorization/protected", nil)
		tt.NoError(err)

		// Request not originating from a browser should be denied
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusUnauthorized, res.StatusCode)
		}

		// Request origination from a browser should be redirected to the login page
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Dest", "document")
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			body, _ := io.ReadAll(res.Body)
			tt.Equal("Login", string(body))
		}

		// Request with insufficient auth token should be rejected
		signedToken, err := tokenissuerapi.NewClient(tester).IssueToken(ctx, jwt.MapClaims{"role": "minor"})
		tt.NoError(err)
		req.AddCookie(&http.Cookie{
			Name:     "Authorization",
			Value:    signedToken,
			MaxAge:   60,
			HttpOnly: true,
			Secure:   false,
			Path:     "/",
		})
		tt.Len(req.Cookies(), 1)
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			tt.Equal(http.StatusForbidden, res.StatusCode)
		}

		// Request with valid auth token should be served
		signedToken, err = tokenissuerapi.NewClient(tester).IssueToken(ctx, jwt.MapClaims{"role": "major"})
		tt.NoError(err)
		req.Header.Del("Cookie")
		req.AddCookie(&http.Cookie{
			Name:     "Authorization",
			Value:    signedToken,
			MaxAge:   60,
			HttpOnly: true,
			Secure:   false,
			Path:     "/",
		})
		tt.Len(req.Cookies(), 1)
		res, err = httpClient.Do(req)
		if tt.NoError(err) {
			body, _ := io.ReadAll(res.Body)
			tt.Equal("Access Granted", string(body))
		}
	})

	t.Run("multi_value_headers", func(t *testing.T) {
		tt := testarossa.For(t)

		req, err := http.NewRequest("GET", "http://localhost:4040/multi.value.headers/ok", nil)
		tt.NoError(err)
		req.Header["Multi-Value"] = []string{
			"Send 1",
			"Send 2",
			"Send 3",
		}
		res, err := httpClient.Do(req)
		if tt.NoError(err) {
			if tt.Len(request.Header["Multi-Value"], 3) {
				tt.Equal("Send 1", request.Header["Multi-Value"][0])
				tt.Equal("Send 2", request.Header["Multi-Value"][1])
				tt.Equal("Send 3", request.Header["Multi-Value"][2])
			}
			if tt.Len(res.Header["Multi-Value"], 2) {
				tt.Equal("Return 1", res.Header["Multi-Value"][0])
				tt.Equal("Return 2", res.Header["Multi-Value"][1])
			}
		}
	})
}

func TestHttpingress_ResolveInternalURL(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	portMappings := map[string]string{
		"8080:*": "*",
		"443:*":  "443",
		"80:*":   "443",
	}

	testCases := []string{
		"https://proxy:8080/service:555/path?arg=val",
		"https://service:555/path?arg=val",

		"https://proxy:8080/service:443/path",
		"https://service/path",

		"https://proxy:8080/service:80/path",
		"https://service:80/path",

		"https://proxy:8080/service/path",
		"https://service/path",

		"http://proxy:8080/service:555/path",
		"https://service:555/path",

		"https://proxy:443/service:555/path",
		"https://service/path",

		"https://proxy:443/service:443/path",
		"https://service/path",

		"https://proxy:443/service/path",
		"https://service/path",

		"https://proxy:80/service/path",
		"https://service/path",
	}
	for i := 0; i < len(testCases); i += 2 {
		x, err := url.Parse(testCases[i])
		tt.NoError(err)
		u, err := url.Parse(testCases[i+1])
		tt.NoError(err)
		ru, err := resolveInternalURL(x, portMappings)
		tt.NoError(err)
		tt.Equal(u, ru)
	}
}

func TestHttpingress_OnChangedPorts(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedAllowedOrigins(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedPortMappings(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedReadTimeout(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedWriteTimeout(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedReadHeaderTimeout(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedBlockedPaths(t *testing.T) {
	t.Skip() // Not tested
}

func TestHttpingress_OnChangedServerLanguages(t *testing.T) {
	t.Skip() // Not tested
}
