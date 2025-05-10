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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/httpingress/middleware"
	"github.com/microbus-io/fabric/coreservices/tokenissuer"
	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/sub"
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	// Add microservices to the testing app
	err = App.AddAndStartup(
		tokenissuer.NewService(),
		Svc.Init(func(svc *Service) {
			svc.SetTimeBudget(time.Second * 2)
			svc.SetPorts("4040,40443")
			svc.SetAllowedOrigins("allowed.origin")
			svc.SetPortMappings("4040:*->*, 40443:*->443")
			svc.Middleware().Append("HelloGoodbye", middleware.OnRoutePrefix("/greeting:555/", middleware.Group(
				func(next connector.HTTPHandler) connector.HTTPHandler {
					return func(w http.ResponseWriter, r *http.Request) error {
						r.Header.Add("Middleware", "Hello")
						err = next(w, r)
						return err // No trace
					}
				},
				func(next connector.HTTPHandler) connector.HTTPHandler {
					return func(w http.ResponseWriter, r *http.Request) error {
						err = next(w, r)
						w.Header().Add("Middleware", "Goodbye")
						return err // No trace
					}
				},
			)))
			svc.Middleware().Append("401Redirect", middleware.ErrorPageRedirect(http.StatusUnauthorized, "/login-page"))
		}),
	)
	if err != nil {
		return err
	}
	return nil
}

// Terminate gets called after the testing app shut down.
func Terminate() (err error) {
	return nil
}

func TestHttpingress_Ports(t *testing.T) {
	t.Parallel()

	con := connector.New("ports")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}
	res, err := client.Get("http://localhost:4040/ports/ok")
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
	}
	res, err = client.Get("http://localhost:40443/ports/ok")
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
	}
}

func TestHttpingress_RequestMemoryLimit(t *testing.T) {
	// No parallel
	memLimit := Svc.RequestMemoryLimit()
	Svc.SetRequestMemoryLimit(1)
	defer Svc.SetRequestMemoryLimit(memLimit)

	entered := make(chan bool)
	done := make(chan bool)
	con := connector.New("request.memory.limit")
	con.Subscribe("POST", "ok", func(w http.ResponseWriter, r *http.Request) error {
		b, _ := io.ReadAll(r.Body)
		w.Write(b)
		return nil
	})
	con.Subscribe("POST", "hold", func(w http.ResponseWriter, r *http.Request) error {
		entered <- true
		<-done
		w.Write([]byte("done"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	// Small request at 25% of capacity
	testarossa.Zero(t, Svc.reqMemoryUsed)
	payload := rand.AlphaNum64(Svc.RequestMemoryLimit() * 1024 * 1024 / 4)
	res, err := client.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, payload, string(b))
		}
	}

	// Big request at 55% of capacity
	testarossa.Zero(t, Svc.reqMemoryUsed)
	payload = rand.AlphaNum64(Svc.RequestMemoryLimit() * 1024 * 1024 * 55 / 100)
	res, err = client.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
	}

	// Two small requests that together are over 50% of capacity
	testarossa.Zero(t, Svc.reqMemoryUsed)
	payload = rand.AlphaNum64(Svc.RequestMemoryLimit() * 1024 * 1024 / 3)
	returned := make(chan bool)
	go func() {
		res, err = client.Post("http://localhost:4040/request.memory.limit/hold", "text/plain", strings.NewReader(payload))
		returned <- true
	}()
	<-entered
	testarossa.NotZero(t, Svc.reqMemoryUsed)
	res, err = client.Post("http://localhost:4040/request.memory.limit/ok", "text/plain", strings.NewReader(payload))
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
	}
	done <- true
	<-returned
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "done", string(b))
		}
	}

	testarossa.Zero(t, Svc.reqMemoryUsed)
}

func TestHttpingress_Compression(t *testing.T) {
	t.Parallel()

	con := connector.New("compression")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(bytes.Repeat([]byte("Hello123"), 1024)) // 8KB
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest("GET", "http://localhost:4040/compression/ok", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	testarossa.NoError(t, err)
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.True(t, len(b) < 8*1024)
		}
		testarossa.Equal(t, strconv.Itoa(len(b)), res.Header.Get("Content-Length"))
	}
}

func TestHttpingress_PortMapping(t *testing.T) {
	t.Parallel()

	con := connector.New("port.mapping")
	con.Subscribe("GET", "ok443", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	con.Subscribe("GET", ":555/ok555", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	// External port 4040 grants access to all internal ports
	res, err := client.Get("http://localhost:4040/port.mapping/ok443")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
	res, err = client.Get("http://localhost:4040/port.mapping:555/ok555")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
	res, err = client.Get("http://localhost:4040/port.mapping:555/ok443")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusNotFound, res.StatusCode)
	}

	// External port 40443 maps all requests to internal port 443
	res, err = client.Get("http://localhost:40443/port.mapping/ok443")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
	res, err = client.Get("http://localhost:40443/port.mapping:555/ok555")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusNotFound, res.StatusCode)
	}
	res, err = client.Get("http://localhost:40443/port.mapping:555/ok443")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
}

func TestHttpingress_ForwardedHeaders(t *testing.T) {
	t.Parallel()

	con := connector.New("forwarded.headers")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
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
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	// Make a standard request
	req, err := http.NewRequest("GET", "http://localhost:4040/forwarded.headers/ok", nil)
	testarossa.NoError(t, err)
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			body := string(b)
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Host: localhost:4040\n"))
			testarossa.False(t, strings.Contains(body, "X-Forwarded-Prefix:"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Proto: http\n"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-For: "))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Path: /forwarded.headers/ok"))
		}
	}

	// Make a request appear to be coming through an upstream proxy server
	req, err = http.NewRequest("GET", "http://localhost:4040/forwarded.headers/ok", nil)
	req.Header.Set("X-Forwarded-Host", "www.example.com")
	req.Header.Set("X-Forwarded-Prefix", "/app")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Forwarded-Proto", "https")
	testarossa.NoError(t, err)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			body := string(b)
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Host: www.example.com\n"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Prefix: /app\n"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Proto: https\n"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-For: 1.2.3.4"))
			testarossa.True(t, strings.Contains(body, "X-Forwarded-Path: /forwarded.headers/ok"))
		}
	}
}

func TestHttpingress_Root(t *testing.T) {
	t.Parallel()

	client := http.Client{Timeout: time.Second * 2}
	res, err := client.Get("http://localhost:4040/")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusNotFound, res.StatusCode)
	}

	con := connector.New("root")
	con.Subscribe("GET", "", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("Root"))
		return nil
	})
	err = App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	res, err = client.Get("http://localhost:4040/")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
}

func TestHttpingress_CORS(t *testing.T) {
	t.Parallel()

	callCount := 0
	con := connector.New("cors")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		callCount++
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	// Request with no origin header
	count := callCount
	req, err := http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
	testarossa.NoError(t, err)
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
		testarossa.Equal(t, count+1, callCount)
	}

	// Request with disallowed origin header
	count = callCount
	req, err = http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
	req.Header.Set("Origin", "disallowed.origin")
	testarossa.NoError(t, err)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusForbidden, res.StatusCode)
		testarossa.Equal(t, count, callCount)
	}

	// Request with allowed origin header
	count = callCount
	req, err = http.NewRequest("GET", "http://localhost:4040/cors/ok", nil)
	req.Header.Set("Origin", "allowed.origin")
	testarossa.NoError(t, err)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
		testarossa.Equal(t, "allowed.origin", res.Header.Get("Access-Control-Allow-Origin"))
		testarossa.Equal(t, count+1, callCount)
	}

	// Preflight request with allowed origin header
	count = callCount
	req, err = http.NewRequest("OPTIONS", "http://localhost:4040/cors/ok", nil)
	req.Header.Set("Origin", "allowed.origin")
	testarossa.NoError(t, err)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusNoContent, res.StatusCode)
		testarossa.Equal(t, count, callCount)
	}
}

func TestHttpingress_ParseForm(t *testing.T) {
	t.Parallel()

	con := connector.New("parse.form")
	con.Subscribe("POST", "ok", func(w http.ResponseWriter, r *http.Request) error {
		err := r.ParseForm()
		if err != nil {
			return errors.Trace(err)
		}
		w.Write([]byte("ok"))
		return nil
	})
	con.Subscribe("POST", "more", func(w http.ResponseWriter, r *http.Request) error {
		r.Body = http.MaxBytesReader(w, r.Body, 12*1024*1024)
		err := r.ParseForm()
		if err != nil {
			return errors.Trace(err)
		}
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	// Under 10MB
	var buf bytes.Buffer
	buf.WriteString("x=")
	buf.WriteString(rand.AlphaNum64(9 * 1024 * 1024))
	res, err := client.Post("http://localhost:4040/parse.form/ok", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
	}

	// Go sets a 10MB limit on forms by default
	// https://go.dev/src/net/http/request.go#L1258
	buf.WriteString(rand.AlphaNum64(2 * 1024 * 1024)) // Now 11MB
	res, err = client.Post("http://localhost:4040/parse.form/ok", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
	}

	// MaxBytesReader can be used to extend the limit
	res, err = client.Post("http://localhost:4040/parse.form/more", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
	}

	// Going above the MaxBytesReader limit
	buf.WriteString(rand.AlphaNum64(2 * 1024 * 1024)) // Now 13MB
	res, err = client.Post("http://localhost:4040/parse.form/more", "application/x-www-form-urlencoded", bytes.NewReader(buf.Bytes()))
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
	}
}

func TestHttpingress_InternalHeaders(t *testing.T) {
	t.Parallel()

	con := connector.New("internal.headers")
	con.Subscribe("GET", ":555/ok", func(w http.ResponseWriter, r *http.Request) error {
		// No Microbus headers should be accepted from client
		testarossa.Equal(t, "", r.Header.Get(frame.HeaderPrefix+"In-Request"))
		testarossa.Equal(t, "", r.Header.Get(strings.ToUpper(frame.HeaderPrefix+"In-Request-Upper")))
		// Microbus headers generated internally should pass through the middleware chain
		testarossa.Equal(t, Hostname, frame.Of(r).FromHost())

		w.Header().Set(frame.HeaderPrefix+"In-Response", "STOP")
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	req, err := http.NewRequest("GET", "http://localhost:4040/internal.headers:555/ok", nil)
	testarossa.NoError(t, err)
	req.Header.Set(frame.HeaderPrefix+"In-Request", "STOP")
	req.Header.Set(strings.ToUpper(frame.HeaderPrefix)+"In-Request-Upper", "STOP")
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		// No Microbus headers should leak outside
		testarossa.Equal(t, "", res.Header.Get(frame.HeaderPrefix+"In-Response"))
		testarossa.Equal(t, "", res.Header.Get(strings.ToUpper(frame.HeaderPrefix+"In-Request-Upper")))
		for h := range res.Header {
			testarossa.False(t, strings.HasPrefix(h, frame.HeaderPrefix))
		}
	}
}

func TestHttpingress_OnRoute(t *testing.T) {
	t.Parallel()

	con := connector.New("greeting")
	con.Subscribe("GET", ":555/ok", func(w http.ResponseWriter, r *http.Request) error {
		// Headers should pass through
		testarossa.Equal(t, "Bearer 123456", r.Header.Get("Authorization"))
		// Middleware added a request header
		testarossa.Equal(t, "Hello", r.Header.Get("Middleware"))
		w.Write([]byte("ok"))
		return nil
	})
	con.Subscribe("GET", ":500/ok", func(w http.ResponseWriter, r *http.Request) error {
		// Headers should pass through
		testarossa.Equal(t, "Bearer 123456", r.Header.Get("Authorization"))
		// Middleware did not run on this route
		testarossa.Equal(t, "", r.Header.Get("Middleware"))
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	req, err := http.NewRequest("GET", "http://localhost:4040/greeting:555/ok", nil)
	testarossa.NoError(t, err)
	req.Header.Set("Authorization", "Bearer 123456")
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
		// Middleware added a response header
		testarossa.Equal(t, "Goodbye", res.Header.Get("Middleware"))
	}

	req, err = http.NewRequest("GET", "http://localhost:4040/greeting:500/ok", nil)
	testarossa.NoError(t, err)
	req.Header.Set("Authorization", "Bearer 123456")
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		b, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.Equal(t, "ok", string(b))
		}
		// Middleware did not run on this route
		testarossa.Equal(t, "", res.Header.Get("Middleware"))
	}
}

func TestHttpingress_BlockedPaths(t *testing.T) {
	t.Parallel()

	con := connector.New("blocked.paths")
	con.Subscribe("GET", "admin.php", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	con.Subscribe("GET", "admin.ppp", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}

	req, err := http.NewRequest("GET", "http://localhost:4040/blocked.paths/admin.php", nil)
	testarossa.NoError(t, err)
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusNotFound, res.StatusCode)
	}
	req, err = http.NewRequest("GET", "http://localhost:4040/blocked.paths/admin.ppp", nil)
	testarossa.NoError(t, err)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
	}
}

func TestHttpingress_DefaultFavIcon(t *testing.T) {
	t.Parallel()

	client := http.Client{Timeout: time.Second * 2}

	req, err := http.NewRequest("GET", "http://localhost:4040/favicon.ico", nil)
	testarossa.NoError(t, err)
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusOK, res.StatusCode)
		testarossa.Equal(t, "image/x-icon", res.Header.Get("Content-Type"))
		icon, err := io.ReadAll(res.Body)
		if testarossa.NoError(t, err) {
			testarossa.NotZero(t, len(icon))
		}
	}
}

func TestHttpingress_NoCache(t *testing.T) {
	t.Parallel()

	con := connector.New("no.cache")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("ok"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}
	res, err := client.Get("http://localhost:4040/no.cache/ok")
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, "no-store", res.Header.Get("Cache-Control"))
	}
}

func TestHttpingress_AuthTokenEntry(t *testing.T) {
	t.Parallel()

	ctx := Context()
	now := time.Now().Truncate(time.Second)

	countActors := 0
	con := connector.New("auth.token.entry")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		if ok, _ := frame.Of(r).IfActor(`iss`); ok {
			countActors++
		}
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 4}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:4040/auth.token.entry/ok", nil)
	testarossa.NoError(t, err)

	// No token
	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 0, countActors)

	// Token by unknown issuer
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "my.issuer",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	signedJWT, err := jwtToken.SignedString([]byte("some-key"))
	testarossa.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+signedJWT)

	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 0, countActors)

	// Attempt to impersonate issuer (wrong key)
	jwtToken = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": tokenissuerapi.Hostname,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	signedJWT, err = jwtToken.SignedString([]byte("wrong-key"))
	testarossa.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+signedJWT)

	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 0, countActors)

	// Do not accept incoming Microbus-Actor header
	req.Header.Del("Authorization")
	req.Header.Set(frame.HeaderActor, `{"iss":"`+tokenissuerapi.Hostname+`"}`)

	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 0, countActors)

	// Valid as Authorization Bearer header
	signedJWT, err = tokenissuerapi.NewClient(Svc).IssueToken(ctx, nil)
	testarossa.NoError(t, err)
	req.Header.Del(frame.HeaderActor)
	req.Header.Set("Authorization", "Bearer "+signedJWT)

	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 1, countActors)

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

	_, err = client.Do(req)
	testarossa.NoError(t, err)
	testarossa.Equal(t, 2, countActors)
}

func TestHttpingress_Authorization(t *testing.T) {
	t.Parallel()

	ctx := Context()

	con := connector.New("authorization")
	con.Subscribe("GET", "protected", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("Access Granted"))
		return nil
	}, sub.Actor("role=='major'"))
	con.Subscribe("GET", "//login-page", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("Login"))
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:4040/authorization/protected", nil)
	testarossa.NoError(t, err)

	// Request not originating from a browser should be denied
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusUnauthorized, res.StatusCode)
	}

	// Request origination from a browser should be redirected to the login page
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		body, _ := io.ReadAll(res.Body)
		testarossa.Equal(t, "Login", string(body))
	}

	// Request with insufficient auth token should be rejected
	signedToken, err := tokenissuerapi.NewClient(Svc).IssueToken(ctx, jwt.MapClaims{"role": "minor"})
	testarossa.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:     "Authorization",
		Value:    signedToken,
		MaxAge:   60,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})
	testarossa.Len(t, req.Cookies(), 1)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		testarossa.Equal(t, http.StatusForbidden, res.StatusCode)
	}

	// Request with valid auth token should be served
	signedToken, err = tokenissuerapi.NewClient(Svc).IssueToken(ctx, jwt.MapClaims{"role": "major"})
	testarossa.NoError(t, err)
	req.Header.Del("Cookie")
	req.AddCookie(&http.Cookie{
		Name:     "Authorization",
		Value:    signedToken,
		MaxAge:   60,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})
	testarossa.Len(t, req.Cookies(), 1)
	res, err = client.Do(req)
	if testarossa.NoError(t, err) {
		body, _ := io.ReadAll(res.Body)
		testarossa.Equal(t, "Access Granted", string(body))
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

func TestHttpingress_MultiValueHeaders(t *testing.T) {
	t.Parallel()

	con := connector.New("multi.value.headers")
	con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
		if testarossa.Len(t, r.Header["Multi-Value"], 3) {
			testarossa.Equal(t, "Send 1", r.Header["Multi-Value"][0])
			testarossa.Equal(t, "Send 2", r.Header["Multi-Value"][1])
			testarossa.Equal(t, "Send 3", r.Header["Multi-Value"][2])
		}
		w.Header()["Multi-Value"] = []string{
			"Return 1",
			"Return 2",
		}
		return nil
	})
	err := App.AddAndStartup(con)
	testarossa.NoError(t, err)
	defer con.Shutdown()

	client := http.Client{} // Timeout: time.Second * 2}
	req, err := http.NewRequest("GET", "http://localhost:4040/multi.value.headers/ok", nil)
	testarossa.NoError(t, err)
	req.Header["Multi-Value"] = []string{
		"Send 1",
		"Send 2",
		"Send 3",
	}
	res, err := client.Do(req)
	if testarossa.NoError(t, err) {
		if testarossa.Len(t, res.Header["Multi-Value"], 2) {
			testarossa.Equal(t, "Return 1", res.Header["Multi-Value"][0])
			testarossa.Equal(t, "Return 2", res.Header["Multi-Value"][1])
		}
	}
}
