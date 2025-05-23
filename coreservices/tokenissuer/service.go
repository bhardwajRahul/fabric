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
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/rand"

	"github.com/microbus-io/fabric/coreservices/tokenissuer/intermediate"
)

const issClaim = "microbus.io"

/*
Service implements the tokenissuer.core microservice.

The token issuer microservice generates and validates JWTs.
*/
type Service struct {
	*intermediate.Intermediate // DO NOT REMOVE
	devOnlySecretKey           string
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() == connector.LOCAL || svc.Deployment() == connector.TESTING {
		svc.devOnlySecretKey = strings.Repeat("0123456789abcdef", 4)
	} else if len(svc.SecretKey()) == 0 {
		return errors.New("secret key is required")
	} else if len(svc.SecretKey()) < 64 {
		svc.LogWarn(ctx, "secret key should be 512 bits (64 characters) long")
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
ValidateToken validates a JWT previously generated by this issuer and returns the actor associated with it.
*/
func (svc *Service) ValidateToken(ctx context.Context, signedToken string) (actor any, valid bool, err error) {
	// Validate the parsed's signature
	parsed := svc.parseToken(signedToken)
	if parsed == nil || !parsed.Valid {
		return nil, false, nil
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false, errors.New("expecting map of claims in token")
	}
	// Verify expiration
	if !claims.VerifyExpiresAt(svc.Now(ctx).Unix(), true) {
		return nil, false, nil
	}
	// Verify issuer
	if !claims.VerifyIssuer(issClaim, true) {
		return nil, false, nil
	}
	delete(claims, "validator")
	delete(claims, "iat")
	delete(claims, "exp")
	delete(claims, "jti")
	delete(claims, "ver")
	return claims, true, nil
}

// parseToken validates the signature of the JWT using the configured keys and returns the token if it is valid.
func (svc *Service) parseToken(signedJWT string) (validToken *jwt.Token) {
	// Try primary key
	if svc.SecretKey() != "" {
		token, _ := jwt.Parse(
			signedJWT,
			func(t *jwt.Token) (interface{}, error) {
				return []byte(svc.SecretKey()), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Name}),
		)
		if token != nil && token.Valid {
			return token
		}
	}
	// Try alternative key (if available)
	if svc.AltSecretKey() != "" {
		token, _ := jwt.Parse(
			signedJWT,
			func(t *jwt.Token) (interface{}, error) {
				return []byte(svc.AltSecretKey()), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Name}),
		)
		if token != nil && token.Valid {
			return token
		}
	}
	// Try dev secret key
	if svc.devOnlySecretKey != "" {
		token, _ := jwt.Parse(
			signedJWT,
			func(t *jwt.Token) (interface{}, error) {
				return []byte(svc.devOnlySecretKey), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Name}),
		)
		if token != nil && token.Valid {
			return token
		}
	}
	return nil
}

/*
IssueToken generates a new JWT with a set of claims.
The claims must be provided as a jwt.MapClaims or an object that can be JSON encoded.
See https://www.iana.org/assignments/jwt/jwt.xhtml for a list of the common claim names.
*/
func (svc *Service) IssueToken(ctx context.Context, claims any) (signedToken string, err error) {
	secretKey := svc.SecretKey()
	if secretKey == "" {
		secretKey = svc.devOnlySecretKey
	}
	if secretKey == "" {
		return "", errors.New("secret key not configured")
	}
	// Convert claims to jwt.MapClaims
	var mapClaims jwt.MapClaims
	if m, ok := claims.(jwt.MapClaims); ok {
		mapClaims = make(jwt.MapClaims, len(m))
		for k, v := range m {
			mapClaims[k] = v
		}
	} else {
		buf, err := json.Marshal(claims)
		if err != nil {
			return "", errors.Trace(err)
		}
		err = json.Unmarshal(buf, &mapClaims)
		if err != nil {
			return "", errors.Trace(err)
		}
	}
	if mapClaims == nil {
		mapClaims = jwt.MapClaims{}
	}
	// Create and sign JWT
	// Refer to https://www.iana.org/assignments/jwt/jwt.xhtml for common claim names
	now := svc.Now(ctx)
	mapClaims["iss"] = issClaim
	mapClaims["validator"] = svc.Hostname()             // Used by Authorization middleware to know to the hostname of the validator
	mapClaims["iat"] = now.Truncate(time.Second).Unix() // Must not be in the future
	mapClaims["exp"] = now.Add(svc.AuthTokenTTL()).Round(time.Second).Unix()
	mapClaims["jti"] = rand.AlphaNum64(24)
	mapClaims["ver"] = 1
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS512, mapClaims)
	signedToken, err = jwtToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.Trace(err)
	}
	return signedToken, nil
}
