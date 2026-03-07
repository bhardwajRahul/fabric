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
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/coreservices/bearertoken/bearertokenapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ bearertokenapi.Client
)

// keyEntry holds a parsed Ed25519 key pair and its metadata.
type keyEntry struct {
	kid        string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// ClaimsTransformer is a function that transforms JWT claims in place before token signing.
// Transformers are called in the order they were added and may add, modify, or remove claims.
type ClaimsTransformer func(ctx context.Context, claims jwt.MapClaims) error

/*
Service implements the external token issuer which signs long-lived JWTs
with Ed25519 keys for external actor authentication.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	mu                 sync.RWMutex
	primary            *keyEntry
	alt                *keyEntry
	issClaim           string
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
	svc.issClaim = "microbus://" + svc.Hostname()
	err = svc.loadKey(svc.PrivateKeyPEM(), &svc.primary)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.loadKey(svc.AltPrivateKeyPEM(), &svc.alt)
	if err != nil {
		return errors.Trace(err)
	}
	// Auto-generate a key in LOCAL or TESTING if none is configured
	if svc.primary == nil && (svc.Deployment() == connector.LOCAL || svc.Deployment() == connector.TESTING) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return errors.Trace(err)
		}
		hash := sha256.Sum256(pub)
		kid := base64.RawURLEncoding.EncodeToString(hash[:8])
		svc.mu.Lock()
		svc.primary = &keyEntry{
			kid:        kid,
			privateKey: priv,
			publicKey:  pub,
		}
		svc.mu.Unlock()
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// loadKey parses a PEM-encoded Ed25519 private key and stores it in the target.
func (svc *Service) loadKey(pemData string, target **keyEntry) (err error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if pemData == "" {
		*target = nil
		return nil
	}

	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return errors.Trace(err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return errors.New("PEM key is not Ed25519")
	}

	pubKey := edKey.Public().(ed25519.PublicKey)

	// Derive a stable kid from the public key
	hash := sha256.Sum256(pubKey)
	kid := base64.RawURLEncoding.EncodeToString(hash[:8])

	*target = &keyEntry{
		kid:        kid,
		privateKey: edKey,
		publicKey:  pubKey,
	}
	return nil
}

// OnChangedPrivateKeyPEM is called when the PrivateKeyPEM config changes.
func (svc *Service) OnChangedPrivateKeyPEM(ctx context.Context) (err error) { // MARKER: PrivateKeyPEM
	return errors.Trace(svc.loadKey(svc.PrivateKeyPEM(), &svc.primary))
}

// OnChangedAltPrivateKeyPEM is called when the AltPrivateKeyPEM config changes.
func (svc *Service) OnChangedAltPrivateKeyPEM(ctx context.Context) (err error) { // MARKER: AltPrivateKeyPEM
	return errors.Trace(svc.loadKey(svc.AltPrivateKeyPEM(), &svc.alt))
}

/*
Mint signs a JWT with the given claims.
*/
func (svc *Service) Mint(ctx context.Context, claims any) (token string, err error) { // MARKER: Mint
	svc.mu.RLock()
	key := svc.primary
	svc.mu.RUnlock()

	if key == nil {
		return "", errors.New("no signing key configured", http.StatusServiceUnavailable)
	}

	now := svc.Now(ctx)

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
	// Run claims transformers
	for _, transformer := range svc.claimsTransformers {
		if err := transformer(ctx, jwtClaims); err != nil {
			return "", errors.Trace(err)
		}
	}
	// Set critical claims last so they cannot be overridden by transformers
	jwtClaims["iss"] = svc.issClaim
	jwtClaims["iat"] = now.Add(-5 * time.Minute).Unix()                     // 5 minutes in the past
	jwtClaims["exp"] = now.Add(svc.AuthTokenTTL() + 5*time.Minute).Unix() // 5 minutes of grace for clock skew
	jwtClaims["jti"] = utils.RandomIdentifier(24)

	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwtClaims)
	t.Header["kid"] = key.kid

	signed, err := t.SignedString(key.privateKey)
	if err != nil {
		return "", errors.Trace(err)
	}
	return signed, nil
}

/*
JWKS returns the public keys of the token issuer in JWKS format.
*/
func (svc *Service) JWKS(ctx context.Context) (keys []bearertokenapi.JWK, err error) { // MARKER: JWKS
	svc.mu.RLock()
	primary := svc.primary
	alt := svc.alt
	svc.mu.RUnlock()

	var result []bearertokenapi.JWK
	for _, ke := range []*keyEntry{primary, alt} {
		if ke == nil {
			continue
		}
		jwk := bearertokenapi.JWK{
			KTY: "OKP",
			CRV: "Ed25519",
			X:   base64.RawURLEncoding.EncodeToString(ke.publicKey),
			KID: ke.kid,
			Use: "sig",
			ALG: "EdDSA",
		}
		result = append(result, jwk)
	}
	return result, nil
}
