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

package accesstoken

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ testarossa.Asserter
	_ accesstokenapi.Client
)

func TestAccessToken_OpenAPI(t *testing.T) {
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

	rePort := regexp.MustCompile(`:([0-9]+)(/|$)`)
	routes := []string{
		// HINT: Insert routes of functional and web endpoints here
		accesstokenapi.Mint.Route,      // MARKER: Mint
		accesstokenapi.LocalKeys.Route, // MARKER: LocalKeys
		accesstokenapi.JWKS.Route,      // MARKER: JWKS
	}
	for _, route := range routes {
		port := "443"
		matches := rePort.FindStringSubmatch(route)
		if len(matches) > 1 {
			port = matches[1]
		}
		t.Run("port_"+port, func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := tester.Request(
				ctx,
				pub.GET(httpx.JoinHostAndPath(accesstokenapi.Hostname, ":"+port+"/openapi.json")),
			)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.Contains(body, "openapi")
					assert.Contains(body, route)
				}
			}
		})
	}
}

func TestAccessToken_Mock(t *testing.T) {
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

	t.Run("mint", func(t *testing.T) { // MARKER: Mint
		assert := testarossa.For(t)

		exampleClaims := map[string]any{"sub": "user1"}
		expectedToken := "mock-token"

		_, err := mock.Mint(ctx, exampleClaims)
		assert.Contains(err.Error(), "not implemented")
		mock.MockMint(func(ctx context.Context, claims any) (token string, err error) {
			return expectedToken, nil
		})
		token, err := mock.Mint(ctx, exampleClaims)
		assert.Expect(
			token, expectedToken,
			err, nil,
		)
	})

	t.Run("jwks", func(t *testing.T) { // MARKER: JWKS
		assert := testarossa.For(t)

		expectedKeys := []accesstokenapi.JWK{}

		_, err := mock.JWKS(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockJWKS(func(ctx context.Context) (keys []accesstokenapi.JWK, err error) {
			return expectedKeys, nil
		})
		keys, err := mock.JWKS(ctx)
		assert.Expect(
			keys, expectedKeys,
			err, nil,
		)
	})

	t.Run("local_keys", func(t *testing.T) { // MARKER: LocalKeys
		assert := testarossa.For(t)

		expectedKeys := []accesstokenapi.JWK{}

		_, err := mock.LocalKeys(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockLocalKeys(func(ctx context.Context) (keys []accesstokenapi.JWK, err error) {
			return expectedKeys, nil
		})
		keys, err := mock.LocalKeys(ctx)
		assert.Expect(
			keys, expectedKeys,
			err, nil,
		)
	})

	t.Run("rotate_key", func(t *testing.T) { // MARKER: RotateKey
		assert := testarossa.For(t)

		err := mock.RotateKey(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockRotateKey(func(ctx context.Context) (err error) {
			return nil
		})
		err = mock.RotateKey(ctx)
		assert.NoError(err)
	})
}

func TestAccessToken_RotateKey(t *testing.T) { // MARKER: RotateKey
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			err := svc.RotateKey(ctx)
			assert.NoError(err)
		})
	*/

	t.Run("initial_key", func(t *testing.T) {
		assert := testarossa.For(t)

		svc.mu.RLock()
		current := svc.currentKey
		previous := svc.previousKey
		svc.mu.RUnlock()
		assert.NotNil(current)
		assert.Nil(previous)
		assert.NotZero(current.kid)
		assert.NotNil(current.privateKey)
		assert.NotNil(current.publicKey)
	})

	t.Run("no_rotate_before_interval", func(t *testing.T) {
		assert := testarossa.For(t)

		svc.mu.RLock()
		kidBefore := svc.currentKey.kid
		svc.mu.RUnlock()

		err := svc.RotateKey(ctx)
		assert.NoError(err)

		svc.mu.RLock()
		kidAfter := svc.currentKey.kid
		svc.mu.RUnlock()
		assert.Expect(kidAfter, kidBefore)
	})

	t.Run("rotate_after_interval", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set a short rotation interval for testing
		err := svc.SetKeyRotationInterval(2 * time.Hour)
		assert.NoError(err)

		svc.mu.RLock()
		kidBefore := svc.currentKey.kid
		svc.mu.RUnlock()

		// Backdate the key to simulate time passing
		svc.mu.Lock()
		svc.currentKey.createdAt = svc.currentKey.createdAt.Add(-3 * time.Hour)
		svc.mu.Unlock()

		err = svc.RotateKey(ctx)
		assert.NoError(err)

		svc.mu.RLock()
		kidAfter := svc.currentKey.kid
		previousKid := svc.previousKey.kid
		svc.mu.RUnlock()
		assert.NotEqual(kidAfter, kidBefore)
		assert.Expect(previousKid, kidBefore)
	})
}

func TestAccessToken_Mint(t *testing.T) { // MARKER: Mint
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := accesstokenapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			token, err := client.Mint(ctx, claims)
			assert.Expect(
				token, expectedToken,
				err, nil,
			)
		})
	*/

	t.Run("basic_mint", func(t *testing.T) {
		assert := testarossa.For(t)

		claims := map[string]any{
			"sub":    "user123",
			"tenant": "acme",
		}
		token, err := client.Mint(ctx, claims)
		if assert.NoError(err) {
			assert.NotZero(token)

			// Parse and verify the token
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
				svc.mu.RLock()
				defer svc.mu.RUnlock()
				return svc.currentKey.publicKey, nil
			})
			if assert.NoError(err) {
				assert.True(parsed.Valid)
				mapClaims := parsed.Claims.(jwt.MapClaims)
				assert.Expect(mapClaims["sub"], "user123")
				assert.Expect(mapClaims["tenant"], "acme")
				assert.Expect(mapClaims["iss"], "microbus://"+accesstokenapi.Hostname)
				assert.NotZero(mapClaims["jti"])
				assert.NotZero(mapClaims["iat"])
				assert.NotZero(mapClaims["exp"])
			}
		}
	})

	t.Run("kid_in_header", func(t *testing.T) {
		assert := testarossa.For(t)

		claims := map[string]any{"sub": "user1"}
		token, err := client.Mint(ctx, claims)
		if assert.NoError(err) {
			parser := jwt.NewParser()
			parsed, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
			if assert.NoError(err) {
				kid, ok := parsed.Header["kid"].(string)
				assert.True(ok)
				svc.mu.RLock()
				assert.Expect(kid, svc.currentKey.kid)
				svc.mu.RUnlock()
			}
		}
	})
}

func TestAccessToken_LocalKeys(t *testing.T) { // MARKER: LocalKeys
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := accesstokenapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			keys, err := client.LocalKeys(ctx)
			assert.NoError(err)
		})
	*/

	t.Run("returns_current_key", func(t *testing.T) {
		assert := testarossa.For(t)

		keys, err := client.LocalKeys(ctx)
		if assert.NoError(err) {
			// Only current key, no previous yet
			assert.Len(keys, 1)
			assert.Expect(keys[0].KTY, "OKP")
			assert.Expect(keys[0].CRV, "Ed25519")
			assert.Expect(keys[0].ALG, "EdDSA")
			assert.Expect(keys[0].Use, "sig")
			assert.NotZero(keys[0].X)
			assert.NotZero(keys[0].KID)
		}
	})

	t.Run("returns_both_keys_after_rotation", func(t *testing.T) {
		assert := testarossa.For(t)

		// Force rotation by backdating
		err := svc.SetKeyRotationInterval(2 * time.Hour)
		assert.NoError(err)
		svc.mu.Lock()
		svc.currentKey.createdAt = svc.currentKey.createdAt.Add(-3 * time.Hour)
		svc.mu.Unlock()
		err = svc.RotateKey(ctx)
		assert.NoError(err)

		keys, err := client.LocalKeys(ctx)
		if assert.NoError(err) {
			assert.Len(keys, 2)
			// Both should have different KIDs
			assert.NotEqual(keys[0].KID, keys[1].KID)
		}
	})
}

func TestAccessToken_JWKS(t *testing.T) { // MARKER: JWKS
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := accesstokenapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			keys, err := client.JWKS(ctx)
			assert.NoError(err)
		})
	*/

	t.Run("aggregates_keys", func(t *testing.T) {
		assert := testarossa.For(t)

		keys, err := client.JWKS(ctx)
		if assert.NoError(err) {
			// Single replica, should have at least 1 key
			assert.True(len(keys) >= 1)
			assert.Expect(keys[0].KTY, "OKP")
			assert.Expect(keys[0].CRV, "Ed25519")
		}
	})

	t.Run("includes_rotated_keys", func(t *testing.T) {
		assert := testarossa.For(t)

		// Force rotation
		err := svc.SetKeyRotationInterval(2 * time.Hour)
		assert.NoError(err)
		svc.mu.Lock()
		svc.currentKey.createdAt = svc.currentKey.createdAt.Add(-3 * time.Hour)
		svc.mu.Unlock()
		err = svc.RotateKey(ctx)
		assert.NoError(err)

		keys, err := client.JWKS(ctx)
		if assert.NoError(err) {
			assert.Len(keys, 2)
		}
	})
}

func TestAccessToken_AddClaimsTransformer(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Initialize the microservice with claims transformers
	svc := NewService()
	err := svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
		claims["role"] = "admin"
		return nil
	})
	assert.NoError(err)
	err = svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
		if sub, ok := claims["sub"].(string); ok {
			claims["greeting"] = "hello " + sub
		}
		return nil
	})
	assert.NoError(err)

	// Initialize the testers
	tester := connector.New("tester.client")
	client := accesstokenapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(svc, tester)
	app.RunInTest(t)

	t.Run("enriches_claims", func(t *testing.T) {
		assert := testarossa.For(t)

		claims := map[string]any{"sub": "user123"}
		token, err := client.Mint(ctx, claims)
		if assert.NoError(err) {
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
				svc.mu.RLock()
				defer svc.mu.RUnlock()
				return svc.currentKey.publicKey, nil
			})
			if assert.NoError(err) {
				mapClaims := parsed.Claims.(jwt.MapClaims)
				assert.Expect(mapClaims["role"], "admin")
				assert.Expect(mapClaims["greeting"], "hello user123")
			}
		}
	})
}
