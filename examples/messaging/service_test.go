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

package messaging

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
)

func TestMessaging_Home(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("messaging.home.tester")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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

func TestMessaging_NoQueue(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("messaging.noqueue.tester")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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

func TestMessaging_DefaultQueue(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("messaging.defaultqueue.tester")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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

func TestMessaging_CacheLoad(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("messaging.cacheload.tester")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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

func TestMessaging_CacheStore(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc1 := NewService()
	svc2 := NewService()

	// Initialize the testers
	tester := connector.New("messaging.cachestore.tester")
	client := messagingapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
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
