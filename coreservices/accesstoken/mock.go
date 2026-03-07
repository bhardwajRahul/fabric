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
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ accesstokenapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockRotateKey func(ctx context.Context) (err error)                                             // MARKER: RotateKey
	mockMint      func(ctx context.Context, claims any) (token string, err error) // MARKER: Mint
	mockLocalKeys func(ctx context.Context) (keys []accesstokenapi.JWK, err error)                  // MARKER: LocalKeys
	mockJWKS      func(ctx context.Context) (keys []accesstokenapi.JWK, err error)                  // MARKER: JWKS
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockRotateKey sets up a mock handler for RotateKey.
func (svc *Mock) MockRotateKey(handler func(ctx context.Context) (err error)) *Mock { // MARKER: RotateKey
	svc.mockRotateKey = handler
	return svc
}

// RotateKey executes the mock handler.
func (svc *Mock) RotateKey(ctx context.Context) (err error) { // MARKER: RotateKey
	if svc.mockRotateKey == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockRotateKey(ctx)
	return errors.Trace(err)
}

// MockMint sets up a mock handler for Mint.
func (svc *Mock) MockMint(handler func(ctx context.Context, claims any) (token string, err error)) *Mock { // MARKER: Mint
	svc.mockMint = handler
	return svc
}

// Mint executes the mock handler.
func (svc *Mock) Mint(ctx context.Context, claims any) (token string, err error) { // MARKER: Mint
	if svc.mockMint == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	token, err = svc.mockMint(ctx, claims)
	return token, errors.Trace(err)
}

// MockLocalKeys sets up a mock handler for LocalKeys.
func (svc *Mock) MockLocalKeys(handler func(ctx context.Context) (keys []accesstokenapi.JWK, err error)) *Mock { // MARKER: LocalKeys
	svc.mockLocalKeys = handler
	return svc
}

// LocalKeys executes the mock handler.
func (svc *Mock) LocalKeys(ctx context.Context) (keys []accesstokenapi.JWK, err error) { // MARKER: LocalKeys
	if svc.mockLocalKeys == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	keys, err = svc.mockLocalKeys(ctx)
	return keys, errors.Trace(err)
}

// MockJWKS sets up a mock handler for JWKS.
func (svc *Mock) MockJWKS(handler func(ctx context.Context) (keys []accesstokenapi.JWK, err error)) *Mock { // MARKER: JWKS
	svc.mockJWKS = handler
	return svc
}

// JWKS executes the mock handler.
func (svc *Mock) JWKS(ctx context.Context) (keys []accesstokenapi.JWK, err error) { // MARKER: JWKS
	if svc.mockJWKS == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	keys, err = svc.mockJWKS(ctx)
	return keys, errors.Trace(err)
}
