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

package tokenissuer

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
)

func TestTokenissuer_IssueToken(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	svc.SetAuthTokenTTL(time.Hour)
	svc.SetSecretKey(rand.AlphaNum64(64))
	// svc.SetAltSecretKey(key)

	// Initialize the testers
	tester := connector.New("tokenissuer.issuetoken.tester")
	client := tokenissuerapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("issue_and_validate_token", func(t *testing.T) {
		assert := testarossa.For(t)

		claims := map[string]any{
			"sub":   "harry@hogwarts.edu",
			"roles": []any{"wizard"},
			"iss":   issClaim,
		}

		signedToken, err := client.IssueToken(ctx, claims)
		assert.Expect(
			strings.HasPrefix(signedToken, "ey"), true,
			err, nil,
		)
		actor, valid, err := client.ValidateToken(ctx, signedToken)
		assert.Expect(
			actor, claims,
			valid, true,
			err, nil,
		)

		token, _ := jwt.Parse(signedToken, nil)
		mc := token.Claims.(jwt.MapClaims)
		assert.True(mc.VerifyIssuer(issClaim, true))
		assert.Equal(issClaim, mc["iss"])
		assert.NotNil(mc["iat"])
		assert.NotNil(mc["exp"])
		assert.NotNil(mc["roles"])
		assert.Equal("harry@hogwarts.edu", mc["sub"])
		assert.Nil(mc["foo"])
	})

	t.Run("reject_tampered_token", func(t *testing.T) {
		assert := testarossa.For(t)

		signedToken, err := client.IssueToken(
			ctx,
			map[string]any{
				"sub":   "harry@hogwarts.edu",
				"roles": []any{"wizard"},
				"iss":   issClaim,
			},
		)
		assert.Expect(
			strings.HasPrefix(signedToken, "ey"), true,
			err, nil,
		)

		// A tampered token should get rejected
		parts := strings.Split(signedToken, ".")
		claims, err := base64.RawStdEncoding.DecodeString(parts[1])
		assert.NoError(err)
		claims = bytes.ReplaceAll(claims, []byte("harry@hogwarts.edu"), []byte("dumbledore@hogwarts.edu"))
		parts[1] = base64.RawStdEncoding.EncodeToString(claims)
		tamperedToken, _ := jwt.Parse(strings.Join(parts, "."), nil)
		if assert.NotNil(tamperedToken) {
			assert.NotEqual("", tamperedToken.Raw)
		}

		actor, valid, err := client.ValidateToken(ctx, tamperedToken.Raw)
		assert.Expect(
			actor, nil,
			valid, false,
			err, nil,
		)
	})

	t.Run("reject_if_key_changes", func(t *testing.T) {
		assert := testarossa.For(t)

		signedToken, err := client.IssueToken(
			ctx,
			map[string]any{
				"sub":   "harry@hogwarts.edu",
				"roles": []any{"wizard"},
				"iss":   issClaim,
			},
		)
		assert.NoError(err)

		_, valid, err := client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, true, err, nil)

		svc.SetSecretKey(rand.AlphaNum64(64))

		_, valid, err = client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, false, err, nil)
	})

	t.Run("key_rotation", func(t *testing.T) {
		assert := testarossa.For(t)

		signedToken, err := client.IssueToken(
			ctx,
			map[string]any{
				"sub":   "harry@hogwarts.edu",
				"roles": []any{"wizard"},
				"iss":   issClaim,
			},
		)
		assert.NoError(err)

		_, valid, err := client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, true, err, nil)

		svc.SetAltSecretKey(svc.SecretKey())
		svc.SetSecretKey(rand.AlphaNum64(64))

		_, valid, err = client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, true, err, nil)
	})

	t.Run("token_expiration", func(t *testing.T) {
		assert := testarossa.For(t)

		signedToken, err := client.IssueToken(
			ctx,
			map[string]any{
				"sub":   "harry@hogwarts.edu",
				"roles": []any{"wizard"},
				"iss":   issClaim,
			},
		)
		assert.NoError(err)

		_, valid, err := client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, true, err, nil)

		futureCtx := frame.CloneContext(ctx)
		frame.Of(futureCtx).SetClockShift(time.Hour + time.Minute)

		_, valid, err = client.ValidateToken(futureCtx, signedToken)
		assert.Expect(valid, false, err, nil)
	})

	t.Run("dev_only_secret_key", func(t *testing.T) {
		assert := testarossa.For(t)

		svc.SetSecretKey("")
		svc.SetAltSecretKey("")

		signedToken, err := client.IssueToken(
			ctx,
			map[string]any{
				"sub":   "harry@hogwarts.edu",
				"roles": []any{"wizard"},
				"iss":   issClaim,
			},
		)
		assert.NoError(err)

		_, valid, err := client.ValidateToken(ctx, signedToken)
		assert.Expect(valid, true, err, nil)

		token, _ := jwt.Parse(signedToken, nil)
		mc := token.Claims.(jwt.MapClaims)
		assert.True(mc.VerifyIssuer(issClaim, true))

		svc.SetSecretKey(rand.AlphaNum64(64))
	})
}

func TestTokenissuer_ValidateToken(t *testing.T) {
	t.Skip() // Tested elsewhere
}
