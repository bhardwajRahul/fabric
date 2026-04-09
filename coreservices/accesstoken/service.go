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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ accesstokenapi.Client
)

// keyPair holds an Ed25519 key pair and its metadata.
type keyPair struct {
	kid        string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	createdAt  time.Time
}

// ClaimsTransformer is a function that transforms JWT claims in place before token signing.
// Transformers are called in the order they were added and may add, modify, or remove claims.
type ClaimsTransformer func(ctx context.Context, claims jwt.MapClaims) error

/*
Service implements the internal token issuer which generates short-lived JWTs
signed with ephemeral Ed25519 keys for internal actor propagation.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	mu                 sync.RWMutex
	currentKey         *keyPair
	previousKey        *keyPair
	claimsTransformers []ClaimsTransformer
}

// AddClaimsTransformer registers a function that transforms JWT claims before token signing.
// Transformers are called in the order they were added. Must be called during initialization,
// before the service starts.
func (svc *Service) AddClaimsTransformer(transformer ClaimsTransformer) error {
	if svc.IsStarted() {
		return errors.New("claims transformer must be added before the service starts")
	}
	svc.claimsTransformers = append(svc.claimsTransformers, transformer)
	return nil
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	err = svc.generateKey(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// generateKey creates a new Ed25519 key pair and rotates the current key to previous.
func (svc *Service) generateKey(ctx context.Context) (err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return errors.Trace(err)
	}
	kid := utils.RandomIdentifier(16)
	now := svc.Now(ctx)

	svc.mu.Lock()
	svc.previousKey = svc.currentKey
	svc.currentKey = &keyPair{
		kid:        kid,
		privateKey: priv,
		publicKey:  pub,
		createdAt:  now,
	}
	svc.mu.Unlock()
	return nil
}

/*
RotateKey checks if the current key has exceeded the rotation interval and generates a new key pair if so.
*/
func (svc *Service) RotateKey(ctx context.Context) (err error) { // MARKER: RotateKey
	svc.mu.RLock()
	current := svc.currentKey
	svc.mu.RUnlock()
	if current == nil {
		err = svc.generateKey(ctx)
		return errors.Trace(err)
	}
	if svc.Now(ctx).Sub(current.createdAt) < svc.KeyRotationInterval() {
		return nil
	}
	err = svc.generateKey(ctx)
	return errors.Trace(err)
}

/*
Mint signs a JWT with the given claims. The token's lifetime is derived from the request's time budget,
falling back to DefaultTokenLifetime if no budget is set, and capped at MaxTokenLifetime.
*/
func (svc *Service) Mint(ctx context.Context, claims any) (token string, err error) { // MARKER: Mint
	svc.mu.RLock()
	current := svc.currentKey
	svc.mu.RUnlock()
	if current == nil {
		return "", errors.New("no signing key available", http.StatusServiceUnavailable)
	}

	// Determine lifetime from the time budget of the request
	lifetime := frame.Of(ctx).TimeBudget()
	if lifetime <= 0 {
		lifetime = svc.DefaultTokenLifetime()
	}
	lifetime = min(lifetime, svc.MaxTokenLifetime())
	now := svc.Now(ctx)
	exp := now.Add(lifetime)

	// Convert claims to map via JSON round-trip
	jwtClaims := jwt.MapClaims{}
	if claims != nil {
		b, jsonErr := json.Marshal(claims)
		if jsonErr != nil {
			return "", errors.Trace(jsonErr)
		}
		if jsonErr = json.Unmarshal(b, &jwtClaims); jsonErr != nil {
			return "", errors.Trace(jsonErr)
		}
	}

	// Save the original identity provider before transformers run
	idp := jwtClaims["iss"]
	// Run claims transformers
	for _, transformer := range svc.claimsTransformers {
		if err := transformer(ctx, jwtClaims); err != nil {
			return "", errors.Trace(err)
		}
	}
	// Set critical claims last so they cannot be overridden by transformers
	jwtClaims["idp"] = idp
	jwtClaims["iss"] = "https://" + accesstokenapi.Hostname
	jwtClaims["microbus"] = "1"
	jwtClaims["iat"] = now.Add(-5 * time.Second).Unix() // 5 seconds in the past to account for clock skew
	jwtClaims["exp"] = exp.Add(5 * time.Second).Unix()  // 5 seconds of grace for clock skew
	jwtClaims["jti"] = utils.RandomIdentifier(16)

	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwtClaims)
	t.Header["kid"] = current.kid

	signed, err := t.SignedString(current.privateKey)
	if err != nil {
		return "", errors.Trace(err)
	}
	return signed, nil
}

/*
LocalKeys returns this replica's current and previous public keys in JWKS format.
*/
func (svc *Service) LocalKeys(ctx context.Context) (keys []accesstokenapi.JWK, err error) { // MARKER: LocalKeys
	svc.mu.RLock()
	current := svc.currentKey
	previous := svc.previousKey
	svc.mu.RUnlock()

	var result []accesstokenapi.JWK
	for _, kp := range []*keyPair{current, previous} {
		if kp == nil {
			continue
		}
		jwk := accesstokenapi.JWK{
			KTY: "OKP",
			CRV: "Ed25519",
			X:   base64.RawURLEncoding.EncodeToString(kp.publicKey),
			KID: kp.kid,
			Use: "sig",
			ALG: "EdDSA",
		}
		result = append(result, jwk)
	}
	return result, nil
}

/*
JWKS aggregates public keys from all replicas by multicasting to LocalKeys and returns them in JWKS format.
*/
func (svc *Service) JWKS(ctx context.Context) (keys []accesstokenapi.JWK, err error) { // MARKER: JWKS
	var result []accesstokenapi.JWK
	for e := range accesstokenapi.NewMulticastClient(svc).LocalKeys(ctx) {
		peerKeys, err := e.Get()
		if err != nil {
			return nil, errors.Trace(err)
		}
		result = append(result, peerKeys...)
	}
	return result, nil
}
