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

package messaging

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
)

var (
	_ context.Context
	_ *testing.T
	_ *application.Application
	_ *connector.Connector
	_ pub.Option
	_ testarossa.TestingT
	_ messagingapi.Client
)

func TestMessaging_OpenAPI(t *testing.T) {
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
				pub.GET(httpx.JoinHostAndPath(messagingapi.Hostname, ":"+port+"/openapi.json")),
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

func TestMessaging_Mock(t *testing.T) {
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

	t.Run("home", func(t *testing.T) { // MARKER: Home
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Home(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockHome(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Home(w, r)
		assert.NoError(err)
	})

	t.Run("no_queue", func(t *testing.T) { // MARKER: NoQueue
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.NoQueue(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockNoQueue(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.NoQueue(w, r)
		assert.NoError(err)
	})

	t.Run("default_queue", func(t *testing.T) { // MARKER: DefaultQueue
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.DefaultQueue(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockDefaultQueue(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.DefaultQueue(w, r)
		assert.NoError(err)
	})

	t.Run("cache_load", func(t *testing.T) { // MARKER: CacheLoad
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.CacheLoad(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockCacheLoad(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.CacheLoad(w, r)
		assert.NoError(err)
	})

	t.Run("cache_store", func(t *testing.T) { // MARKER: CacheStore
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.CacheStore(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockCacheStore(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.CacheStore(w, r)
		assert.NoError(err)
	})
}

func TestMessaging_Home(t *testing.T) { // MARKER: Home
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc1,
		svc2,
		tester,
	)
	app.RunInTest(t)

	t.Run("both_replicas_found", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Home(ctx, "")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, svc1.ID())
				assert.Contains(body, svc2.ID())
			}
		}
	})
}

func TestMessaging_NoQueue(t *testing.T) { // MARKER: NoQueue
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc1,
		svc2,
		tester,
	)
	app.RunInTest(t)

	t.Run("no_queue", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.NoQueue(ctx, "")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "NoQueue")
				assert.True(bytes.Contains(body, []byte(svc1.ID())) || bytes.Contains(body, []byte(svc2.ID())))
			}
		}
	})
}

func TestMessaging_DefaultQueue(t *testing.T) { // MARKER: DefaultQueue
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc1,
		svc2,
		tester,
	)
	app.RunInTest(t)

	t.Run("default_queue", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.DefaultQueue(ctx, "")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "DefaultQueue")
				assert.True(bytes.Contains(body, []byte(svc1.ID())) || bytes.Contains(body, []byte(svc2.ID())))
			}
		}
	})
}

func TestMessaging_CacheLoad(t *testing.T) { // MARKER: CacheLoad
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc1,
		svc2,
		tester,
	)
	app.RunInTest(t)

	t.Run("store_and_load", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.CacheLoad(ctx, "?key=key1")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "key: key1")
				assert.Contains(body, "found: no")
			}
		}

		_, err = client.CacheStore(ctx, "?key=key1&value=value1")
		assert.NoError(err)

		res, err = client.CacheLoad(ctx, "?key=key1")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "key: key1")
				assert.Contains(body, "found: yes")
				assert.Contains(body, "value: value1")
			}
		}
	})

	t.Run("missing_key", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := client.CacheLoad(ctx, "")
		assert.Contains(err, "missing")
	})
}

func TestMessaging_CacheStore(t *testing.T) { // MARKER: CacheStore
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc1,
		svc2,
		tester,
	)
	app.RunInTest(t)

	t.Run("store_and_get", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.CacheStore(ctx, "?key=aaa&value=111")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "key: aaa")
				assert.Contains(body, "value: 111")
			}
		}
		res, err = client.CacheStore(ctx, "?key=bbb&value=222")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "key: bbb")
				assert.Contains(body, "value: 222")
			}
		}

		res, err = client.CacheLoad(ctx, "?key=aaa")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "found: yes")
			}
		}
		res, err = client.CacheLoad(ctx, "?key=bbb")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "found: yes")
			}
		}
		res, err = client.CacheLoad(ctx, "?key=ccc")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "found: no")
			}
		}
	})

	t.Run("missing_key", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := client.CacheStore(ctx, "")
		assert.Contains(err, "missing")

		_, err = client.CacheStore(ctx, "?value=nokey")
		assert.Contains(err, "missing")
	})

	t.Run("missing_value", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := client.CacheStore(ctx, "?key=novalue")
		assert.Contains(err, "missing")

		res, err := client.CacheLoad(ctx, "?key=novalue")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "found: no")
			}
		}
	})
}
