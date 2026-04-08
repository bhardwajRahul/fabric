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
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/bearertoken/bearertokenapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ bearertokenapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockMint                   func(ctx context.Context, claims any) (token string, err error)  // MARKER: Mint
	mockJWKS                   func(ctx context.Context) (keys []bearertokenapi.JWK, err error) // MARKER: JWKS
	mockOnChangedPrivateKey    func(ctx context.Context) (err error)                            // MARKER: PrivateKey
	mockOnChangedAltPrivateKey func(ctx context.Context) (err error)                            // MARKER: AltPrivateKey
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

// MockJWKS sets up a mock handler for JWKS.
func (svc *Mock) MockJWKS(handler func(ctx context.Context) (keys []bearertokenapi.JWK, err error)) *Mock { // MARKER: JWKS
	svc.mockJWKS = handler
	return svc
}

// JWKS executes the mock handler.
func (svc *Mock) JWKS(ctx context.Context) (keys []bearertokenapi.JWK, err error) { // MARKER: JWKS
	if svc.mockJWKS == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	keys, err = svc.mockJWKS(ctx)
	return keys, errors.Trace(err)
}

// MockOnChangedPrivateKey sets up a mock handler for OnChangedPrivateKey.
func (svc *Mock) MockOnChangedPrivateKey(handler func(ctx context.Context) (err error)) *Mock { // MARKER: PrivateKey
	svc.mockOnChangedPrivateKey = handler
	return svc
}

// OnChangedPrivateKey executes the mock handler.
func (svc *Mock) OnChangedPrivateKey(ctx context.Context) (err error) { // MARKER: PrivateKey
	if svc.mockOnChangedPrivateKey == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnChangedPrivateKey(ctx)
	return errors.Trace(err)
}

// MockOnChangedAltPrivateKey sets up a mock handler for OnChangedAltPrivateKey.
func (svc *Mock) MockOnChangedAltPrivateKey(handler func(ctx context.Context) (err error)) *Mock { // MARKER: AltPrivateKey
	svc.mockOnChangedAltPrivateKey = handler
	return svc
}

// OnChangedAltPrivateKey executes the mock handler.
func (svc *Mock) OnChangedAltPrivateKey(ctx context.Context) (err error) { // MARKER: AltPrivateKey
	if svc.mockOnChangedAltPrivateKey == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnChangedAltPrivateKey(ctx)
	return errors.Trace(err)
}
