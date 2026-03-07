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

package bearertoken

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/bearertoken/bearertokenapi"
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
	_ bearertokenapi.Client
)

// generateTestPEM creates a new Ed25519 private key and returns it as PEM-encoded string.
func generateTestPEM() string {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}
	return string(pem.EncodeToMemory(block))
}

func TestBearerToken_OpenAPI(t *testing.T) {
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
		bearertokenapi.Mint.Route, // MARKER: Mint
		bearertokenapi.JWKS.Route, // MARKER: JWKS
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
				pub.GET(httpx.JoinHostAndPath(bearertokenapi.Hostname, ":"+port+"/openapi.json")),
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

func TestBearerToken_Mock(t *testing.T) {
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

		expectedKeys := []bearertokenapi.JWK{}

		_, err := mock.JWKS(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockJWKS(func(ctx context.Context) (keys []bearertokenapi.JWK, err error) {
			return expectedKeys, nil
		})
		keys, err := mock.JWKS(ctx)
		assert.Expect(
			keys, expectedKeys,
			err, nil,
		)
	})

	t.Run("on_changed_private_key_pem", func(t *testing.T) { // MARKER: PrivateKeyPEM
		assert := testarossa.For(t)

		err := mock.OnChangedPrivateKeyPEM(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockOnChangedPrivateKeyPEM(func(ctx context.Context) (err error) {
			return nil
		})
		err = mock.OnChangedPrivateKeyPEM(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_alt_private_key_pem", func(t *testing.T) { // MARKER: AltPrivateKeyPEM
		assert := testarossa.For(t)

		err := mock.OnChangedAltPrivateKeyPEM(ctx)
		assert.Contains(err.Error(), "not implemented")
		mock.MockOnChangedAltPrivateKeyPEM(func(ctx context.Context) (err error) {
			return nil
		})
		err = mock.OnChangedAltPrivateKeyPEM(ctx)
		assert.NoError(err)
	})
}

func TestBearerToken_Mint(t *testing.T) { // MARKER: Mint
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := bearertokenapi.NewClient(tester)
	_ = client

	// Configure a test key
	testPEM := generateTestPEM()
	svc.Init(func(svc *Service) (err error) {
		return svc.SetPrivateKeyPEM(testPEM)
	})

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
			svc.mu.RLock()
			pubKey := svc.primary.publicKey
			svc.mu.RUnlock()
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
				return pubKey, nil
			})
			if assert.NoError(err) {
				assert.True(parsed.Valid)
				mapClaims := parsed.Claims.(jwt.MapClaims)
				assert.Expect(mapClaims["sub"], "user123")
				assert.Expect(mapClaims["tenant"], "acme")
				assert.Expect(mapClaims["iss"], "microbus://"+bearertokenapi.Hostname)
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
				assert.Expect(kid, svc.primary.kid)
				svc.mu.RUnlock()
			}
		}
	})

	t.Run("no_key_configured", func(t *testing.T) {
		assert := testarossa.For(t)

		// Clear the key
		svc.mu.Lock()
		savedKey := svc.primary
		svc.primary = nil
		svc.mu.Unlock()
		defer func() {
			svc.mu.Lock()
			svc.primary = savedKey
			svc.mu.Unlock()
		}()

		_, err := client.Mint(ctx, map[string]any{"sub": "x"})
		assert.Error(err)
	})
}

func TestBearerToken_JWKS(t *testing.T) { // MARKER: JWKS
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := bearertokenapi.NewClient(tester)
	_ = client

	// Configure test keys
	testPEM := generateTestPEM()
	altPEM := generateTestPEM()
	svc.Init(func(svc *Service) (err error) {
		return svc.SetPrivateKeyPEM(testPEM)
	})

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

	t.Run("returns_primary_key", func(t *testing.T) {
		assert := testarossa.For(t)

		keys, err := client.JWKS(ctx)
		if assert.NoError(err) {
			assert.Len(keys, 1)
			assert.Expect(keys[0].KTY, "OKP")
			assert.Expect(keys[0].CRV, "Ed25519")
			assert.Expect(keys[0].ALG, "EdDSA")
			assert.Expect(keys[0].Use, "sig")
			assert.NotZero(keys[0].X)
			assert.NotZero(keys[0].KID)
		}
	})

	t.Run("returns_both_keys", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set alt key
		err := svc.SetAltPrivateKeyPEM(altPEM)
		assert.NoError(err)
		defer svc.SetAltPrivateKeyPEM("")

		keys, err := client.JWKS(ctx)
		if assert.NoError(err) {
			assert.Len(keys, 2)
			assert.NotEqual(keys[0].KID, keys[1].KID)
		}
	})

	t.Run("verify_token_with_jwks", func(t *testing.T) {
		assert := testarossa.For(t)

		// Mint a token
		token, err := client.Mint(ctx, map[string]any{"sub": "test"})
		assert.NoError(err)

		// Get JWKS
		jwks, err := client.JWKS(ctx)
		assert.NoError(err)

		// Parse unverified to get kid
		parser := jwt.NewParser()
		parsed, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
		assert.NoError(err)
		kid := parsed.Header["kid"].(string)

		// Find matching key in JWKS
		var pubKeyBytes []byte
		for _, jwk := range jwks {
			if jwk.KID == kid {
				pubKeyBytes, err = base64.RawURLEncoding.DecodeString(jwk.X)
				assert.NoError(err)
				break
			}
		}
		assert.NotNil(pubKeyBytes)

		// Verify the token
		pubKey := ed25519.PublicKey(pubKeyBytes)
		verified, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			return pubKey, nil
		})
		if assert.NoError(err) {
			assert.True(verified.Valid)
		}
	})
}

func TestBearerToken_AddClaimsTransformer(t *testing.T) {
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
	client := bearertokenapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(svc, tester)
	app.RunInTest(t)

	t.Run("enriches_claims", func(t *testing.T) {
		assert := testarossa.For(t)

		claims := map[string]any{"sub": "user123"}
		token, err := client.Mint(ctx, claims)
		if assert.NoError(err) {
			svc.mu.RLock()
			pubKey := svc.primary.publicKey
			svc.mu.RUnlock()
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
				return pubKey, nil
			})
			if assert.NoError(err) {
				mapClaims := parsed.Claims.(jwt.MapClaims)
				assert.Expect(mapClaims["role"], "admin")
				assert.Expect(mapClaims["greeting"], "hello user123")
			}
		}
	})
}
