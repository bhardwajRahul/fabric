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
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/service"

	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
)

var (
	_ *testing.T
	_ testarossa.TestingT
	_ service.Service
	_ *tokenissuerapi.Client
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	// Add microservices to the testing app
	err = App.AddAndStartup(
		Svc,
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

func TestTokenissuer_IssueToken(t *testing.T) {
	// No parallel

	ctx := Context()
	Svc.SetAuthTokenTTL(time.Hour)
	actor := jwt.MapClaims{
		"sub":   "harry@hogwarts.edu",
		"roles": []any{"wizard"},
		"iss":   issClaim,
	}

	// Generate and validate token using the first key
	Svc.SetSecretKey("FirstKey1234567890")
	signedJWT, _ := IssueToken(t, ctx, actor).NoError().Get()
	ValidateToken(t, ctx, signedJWT).
		Expect(actor, true)

	token, _ := jwt.Parse(signedJWT, nil)
	mc := token.Claims.(jwt.MapClaims)
	testarossa.True(t, mc.VerifyIssuer(issClaim, true))
	testarossa.Equal(t, issClaim, mc["iss"])
	testarossa.NotNil(t, mc["iat"])
	testarossa.NotNil(t, mc["exp"])
	testarossa.NotNil(t, mc["roles"])
	testarossa.Equal(t, "harry@hogwarts.edu", mc["sub"])
	testarossa.Nil(t, mc["foo"])

	// A tampered token should get rejected
	parts := strings.Split(signedJWT, ".")
	claims, err := base64.RawStdEncoding.DecodeString(parts[1])
	testarossa.NoError(t, err)
	claims = bytes.ReplaceAll(claims, []byte("harry@hogwarts.edu"), []byte("dumbledore@hogwarts.edu"))
	parts[1] = base64.RawStdEncoding.EncodeToString(claims)
	tamperedToken, _ := jwt.Parse(strings.Join(parts, "."), nil)
	if testarossa.NotNil(t, tamperedToken) {
		testarossa.NotEqual(t, "", tamperedToken.Raw)
	}
	ValidateToken(t, ctx, tamperedToken.Raw).
		Expect(nil, false)

	// The token should get rejected if the key is changed
	Svc.SetSecretKey("SecondKey1234567890")
	ValidateToken(t, ctx, signedJWT).
		Expect(nil, false)

	// The token should be validated if the first key is set as an alternative key
	Svc.SetAltSecretKey("FirstKey1234567890")
	ValidateToken(t, ctx, signedJWT).
		Expect(actor, true)

	// Generate and validate a token using the second key
	signedJWT, _ = IssueToken(t, ctx, actor).NoError().Get()
	ValidateToken(t, ctx, signedJWT).
		Expect(actor, true)

	// Token should expire after 1h
	frame.Of(ctx).SetClockShift(time.Hour + time.Minute)
	ValidateToken(t, ctx, signedJWT).
		Expect(nil, false)
}

func TestTokenissuer_DevOnlySecretKey(t *testing.T) {
	// No parallel

	ctx := Context()

	Svc.SetSecretKey("")
	Svc.SetAltSecretKey("")

	actor := jwt.MapClaims{
		"sub":   "harry@hogwarts.edu",
		"roles": []any{"wizard"},
		"iss":   issClaim,
	}
	signedJWT, _ := IssueToken(t, ctx, actor).NoError().Get()
	ValidateToken(t, ctx, signedJWT).Expect(actor, true)

	token, _ := jwt.Parse(signedJWT, nil)
	mc := token.Claims.(jwt.MapClaims)
	testarossa.True(t, mc.VerifyIssuer(issClaim, true))
}

func TestTokenissuer_ValidateToken(t *testing.T) {
	t.Skip() // Tested elsewhere
}
