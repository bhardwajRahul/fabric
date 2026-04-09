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
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/bearertoken/bearertokenapi"
	"github.com/microbus-io/fabric/coreservices/bearertoken/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ bearertokenapi.Client
	_ *workflow.Flow
)

const (
	Hostname = bearertokenapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	OnChangedPrivateKey(ctx context.Context) (err error)             // MARKER: PrivateKey
	OnChangedAltPrivateKey(ctx context.Context) (err error)          // MARKER: AltPrivateKey
	Mint(ctx context.Context, claims any) (token string, err error)  // MARKER: Mint
	JWKS(ctx context.Context) (keys []bearertokenapi.JWK, err error) // MARKER: JWKS
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`BearerToken signs long-lived JWTs with Ed25519 keys for external actor authentication.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe( // MARKER: Mint
		"Mint", svc.doMint,
		sub.At(bearertokenapi.Mint.Method, bearertokenapi.Mint.Route),
		sub.Description(`Mint signs a JWT with the given claims.`),
		sub.Function(bearertokenapi.MintIn{}, bearertokenapi.MintOut{}),
	)
	svc.Subscribe( // MARKER: JWKS
		"JWKS", svc.doJWKS,
		sub.At(bearertokenapi.JWKS.Method, bearertokenapi.JWKS.Route),
		sub.Description(`JWKS returns the public keys of the token issuer in JWKS format.`),
		sub.Function(bearertokenapi.JWKSIn{}, bearertokenapi.JWKSOut{}),
	)

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: AuthTokenTTL
		"AuthTokenTTL",
		cfg.Description(`AuthTokenTTL sets the TTL of the JWT.`),
		cfg.DefaultValue("720h"),
		cfg.Validation("dur [1m,]"),
	)
	svc.DefineConfig( // MARKER: PrivateKey
		"PrivateKey",
		cfg.Description(`PrivateKey is the Ed25519 private key used to sign JWTs, in PEM or raw base64 format.`),
		cfg.Secret(),
	)
	svc.DefineConfig( // MARKER: AltPrivateKey
		"AltPrivateKey",
		cfg.Description(`AltPrivateKey is an alternative Ed25519 private key used during key rotation, in PEM or raw base64 format.`),
		cfg.Secret(),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
	// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	if changed("PrivateKey") { // MARKER: PrivateKey
		err = svc.OnChangedPrivateKey(ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}
	if changed("AltPrivateKey") { // MARKER: AltPrivateKey
		err = svc.OnChangedAltPrivateKey(ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

/*
AuthTokenTTL sets the TTL of the JWT.
*/
func (svc *Intermediate) AuthTokenTTL() (value time.Duration) { // MARKER: AuthTokenTTL
	_val := svc.Config("AuthTokenTTL")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetAuthTokenTTL sets the value of the configuration property.
*/
func (svc *Intermediate) SetAuthTokenTTL(value time.Duration) (err error) { // MARKER: AuthTokenTTL
	return svc.SetConfig("AuthTokenTTL", value.String())
}

/*
PrivateKey is the Ed25519 private key used to sign JWTs, in PEM or raw base64 format.
*/
func (svc *Intermediate) PrivateKey() (value string) { // MARKER: PrivateKey
	return svc.Config("PrivateKey")
}

/*
SetPrivateKey sets the value of the configuration property.
*/
func (svc *Intermediate) SetPrivateKey(value string) (err error) { // MARKER: PrivateKey
	return svc.SetConfig("PrivateKey", value)
}

/*
AltPrivateKey is an alternative Ed25519 private key used during key rotation, in PEM or raw base64 format.
*/
func (svc *Intermediate) AltPrivateKey() (value string) { // MARKER: AltPrivateKey
	return svc.Config("AltPrivateKey")
}

/*
SetAltPrivateKey sets the value of the configuration property.
*/
func (svc *Intermediate) SetAltPrivateKey(value string) (err error) { // MARKER: AltPrivateKey
	return svc.SetConfig("AltPrivateKey", value)
}

// doMint handles marshaling for Mint.
func (svc *Intermediate) doMint(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Mint
	var in bearertokenapi.MintIn
	var out bearertokenapi.MintOut
	err = marshalFunction(w, r, bearertokenapi.Mint.Route, &in, &out, func(_ any, _ any) error {
		out.Token, err = svc.Mint(r.Context(), in.Claims)
		return err // No trace
	})
	return err // No trace
}

// doJWKS handles marshaling for JWKS.
func (svc *Intermediate) doJWKS(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JWKS
	var in bearertokenapi.JWKSIn
	var out bearertokenapi.JWKSOut
	err = marshalFunction(w, r, bearertokenapi.JWKS.Route, &in, &out, func(_ any, _ any) error {
		out.Keys, err = svc.JWKS(r.Context())
		return err // No trace
	})
	return err // No trace
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
