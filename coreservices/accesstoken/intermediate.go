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
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
	"github.com/microbus-io/fabric/coreservices/accesstoken/resources"
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
	_ accesstokenapi.Client
)

const (
	Hostname = accesstokenapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	RotateKey(ctx context.Context) (err error)                            // MARKER: RotateKey
	Mint(ctx context.Context, claims any) (token string, err error)       // MARKER: Mint
	LocalKeys(ctx context.Context) (keys []accesstokenapi.JWK, err error) // MARKER: LocalKeys
	JWKS(ctx context.Context) (keys []accesstokenapi.JWK, err error)      // MARKER: JWKS
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
	svc.SetDescription(`AccessToken generates short-lived JWTs signed with ephemeral Ed25519 keys for internal actor propagation.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(accesstokenapi.Mint.Method, accesstokenapi.Mint.Route, svc.doMint)                               // MARKER: Mint
	svc.Subscribe(accesstokenapi.LocalKeys.Method, accesstokenapi.LocalKeys.Route, svc.doLocalKeys, sub.NoQueue()) // MARKER: LocalKeys
	svc.Subscribe(accesstokenapi.JWKS.Method, accesstokenapi.JWKS.Route, svc.doJWKS)                               // MARKER: JWKS

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here
	svc.StartTicker("RotateKey", 10*time.Minute, svc.RotateKey) // MARKER: RotateKey

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: KeyRotationInterval
		"KeyRotationInterval",
		cfg.Description(`KeyRotationInterval is the duration between Ed25519 key rotations.`),
		cfg.DefaultValue("6h"),
		cfg.Validation("dur [2h,]"),
	)
	svc.DefineConfig( // MARKER: DefaultTokenLifetime
		"DefaultTokenLifetime",
		cfg.Description(`DefaultTokenLifetime is the token lifetime used when no time budget is present in the request.`),
		cfg.DefaultValue("20s"),
		cfg.Validation("dur [1s,15m]"),
	)
	svc.DefineConfig( // MARKER: MaxTokenLifetime
		"MaxTokenLifetime",
		cfg.Description(`MaxTokenLifetime is the maximum token lifetime regardless of the request's time budget.`),
		cfg.DefaultValue("15m"),
		cfg.Validation("dur [1s,15m]"),
	)

	// HINT: Add inbound event sinks here

	_ = marshalFunction
	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
		{ // MARKER: JWKS
			Type:        "function",
			Name:        "JWKS",
			Method:      accesstokenapi.JWKS.Method,
			Route:       accesstokenapi.JWKS.Route,
			Summary:     "JWKS() (keys []JWK)",
			Description: `JWKS aggregates public keys from all replicas and returns them in JWKS format.`,
			InputArgs:   accesstokenapi.JWKSIn{},
			OutputArgs:  accesstokenapi.JWKSOut{},
		},
		{ // MARKER: LocalKeys
			Type:        "function",
			Name:        "LocalKeys",
			Method:      accesstokenapi.LocalKeys.Method,
			Route:       accesstokenapi.LocalKeys.Route,
			Summary:     "LocalKeys() (keys []JWK)",
			Description: `LocalKeys returns this replica's current and previous public keys in JWKS format.`,
			InputArgs:   accesstokenapi.LocalKeysIn{},
			OutputArgs:  accesstokenapi.LocalKeysOut{},
		},
		{ // MARKER: Mint
			Type:        "function",
			Name:        "Mint",
			Method:      accesstokenapi.Mint.Method,
			Route:       accesstokenapi.Mint.Route,
			Summary:     "Mint(claims MapClaims) (token string)",
			Description: `Mint signs a JWT with the given claims. The token's lifetime is derived from the request's time budget, falling back to DefaultTokenLifetime if no budget is set, and capped at MaxTokenLifetime.`,
			InputArgs:   accesstokenapi.MintIn{},
			OutputArgs:  accesstokenapi.MintOut{},
		},
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
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
	return nil
}

/*
KeyRotationInterval is the duration between Ed25519 key rotations.
*/
func (svc *Intermediate) KeyRotationInterval() (value time.Duration) { // MARKER: KeyRotationInterval
	_val := svc.Config("KeyRotationInterval")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetKeyRotationInterval sets the value of the configuration property.
*/
func (svc *Intermediate) SetKeyRotationInterval(value time.Duration) (err error) { // MARKER: KeyRotationInterval
	return svc.SetConfig("KeyRotationInterval", value.String())
}

/*
DefaultTokenLifetime is the token lifetime used when no time budget is present in the request.
*/
func (svc *Intermediate) DefaultTokenLifetime() (value time.Duration) { // MARKER: DefaultTokenLifetime
	_val := svc.Config("DefaultTokenLifetime")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetDefaultTokenLifetime sets the value of the configuration property.
*/
func (svc *Intermediate) SetDefaultTokenLifetime(value time.Duration) (err error) { // MARKER: DefaultTokenLifetime
	return svc.SetConfig("DefaultTokenLifetime", value.String())
}

/*
MaxTokenLifetime is the maximum token lifetime regardless of the request's time budget.
*/
func (svc *Intermediate) MaxTokenLifetime() (value time.Duration) { // MARKER: MaxTokenLifetime
	_val := svc.Config("MaxTokenLifetime")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetMaxTokenLifetime sets the value of the configuration property.
*/
func (svc *Intermediate) SetMaxTokenLifetime(value time.Duration) (err error) { // MARKER: MaxTokenLifetime
	return svc.SetConfig("MaxTokenLifetime", value.String())
}

// doJWKS handles marshaling for JWKS.
func (svc *Intermediate) doJWKS(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JWKS
	var in accesstokenapi.JWKSIn
	var out accesstokenapi.JWKSOut
	err = marshalFunction(w, r, accesstokenapi.JWKS.Route, &in, &out, func(_ any, _ any) error {
		out.Keys, err = svc.JWKS(r.Context())
		return err
	})
	return err // No trace
}

// doLocalKeys handles marshaling for LocalKeys.
func (svc *Intermediate) doLocalKeys(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: LocalKeys
	var in accesstokenapi.LocalKeysIn
	var out accesstokenapi.LocalKeysOut
	err = marshalFunction(w, r, accesstokenapi.LocalKeys.Route, &in, &out, func(_ any, _ any) error {
		out.Keys, err = svc.LocalKeys(r.Context())
		return err
	})
	return err // No trace
}

// doMint handles marshaling for Mint.
func (svc *Intermediate) doMint(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Mint
	var in accesstokenapi.MintIn
	var out accesstokenapi.MintOut
	err = marshalFunction(w, r, accesstokenapi.Mint.Route, &in, &out, func(_ any, _ any) error {
		out.Token, err = svc.Mint(r.Context(), in.Claims)
		return err
	})
	return err // No trace
}

// marshalFunction handled marshaling for functional endpoints.
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
