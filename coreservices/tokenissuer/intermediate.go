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

package tokenissuer

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

	"github.com/microbus-io/fabric/coreservices/tokenissuer/resources"
	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
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
	_ tokenissuerapi.Client
)

const (
	Hostname = tokenissuerapi.Hostname
	Version  = 118
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	IssueToken(ctx context.Context, claims tokenissuerapi.MapClaims) (signedToken string, err error)                // MARKER: IssueToken
	ValidateToken(ctx context.Context, signedToken string) (claims tokenissuerapi.MapClaims, valid bool, err error) // MARKER: ValidateToken
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
	svc.SetDescription(`The token issuer microservice generates and validates JWTs.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functional endpoints
	svc.Subscribe(tokenissuerapi.IssueToken.Method, tokenissuerapi.IssueToken.Route, svc.doIssueToken)       // MARKER: IssueToken
	svc.Subscribe(tokenissuerapi.ValidateToken.Method, tokenissuerapi.ValidateToken.Route, svc.doValidateToken) // MARKER: ValidateToken

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// Configs
	svc.DefineConfig( // MARKER: AuthTokenTTL
		"AuthTokenTTL",
		cfg.Description(`AuthTokenTTL sets the TTL of the JWT.`),
		cfg.DefaultValue("720h"),
		cfg.Validation("dur [1m,]"),
	)
	svc.DefineConfig( // MARKER: SecretKey
		"SecretKey",
		cfg.Description(`SecretKey is a symmetrical key used to sign and validate the JWT when using the HMAC-SHA algorithm.`),
		cfg.Validation(`str ^(|.{64,})$`),
		cfg.Secret(),
	)
	svc.DefineConfig( // MARKER: AltSecretKey
		"AltSecretKey",
		cfg.Description(`AltSecretKey is an alternative key used to validate the JWT when using the HMAC-SHA algorithm.
Setting the previous secret key as an alternative key is useful when rotating keys.`),
		cfg.Validation(`str ^(|.{64,})$`),
		cfg.Secret(),
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
		{ // MARKER: IssueToken
			Type:    "function",
			Name:    "IssueToken",
			Method:  tokenissuerapi.IssueToken.Method,
			Route:   tokenissuerapi.IssueToken.Route,
			Summary: "IssueToken(claims MapClaims) (signedToken string)",
			Description: `IssueToken generates a new JWT with a set of claims.
The claims must be provided as a jwt.MapClaims or an object that can be JSON encoded.
See https://www.iana.org/assignments/jwt/jwt.xhtml for a list of the common claim names.`,
			InputArgs:  tokenissuerapi.IssueTokenIn{},
			OutputArgs: tokenissuerapi.IssueTokenOut{},
		},
		{ // MARKER: ValidateToken
			Type:        "function",
			Name:        "ValidateToken",
			Method:      tokenissuerapi.ValidateToken.Method,
			Route:       tokenissuerapi.ValidateToken.Route,
			Summary:     "ValidateToken(signedToken string) (claims MapClaims, valid bool)",
			Description: `ValidateToken validates a JWT previously generated by this issuer and returns the claims associated with it.`,
			InputArgs:   tokenissuerapi.ValidateTokenIn{},
			OutputArgs:  tokenissuerapi.ValidateTokenOut{},
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
AuthTokenTTL sets the TTL of the JWT.
*/
func (svc *Intermediate) AuthTokenTTL() (ttl time.Duration) { // MARKER: AuthTokenTTL
	_val := svc.Config("AuthTokenTTL")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetAuthTokenTTL sets the value of the configuration property.
*/
func (svc *Intermediate) SetAuthTokenTTL(ttl time.Duration) (err error) { // MARKER: AuthTokenTTL
	return svc.SetConfig("AuthTokenTTL", ttl.String())
}

/*
SecretKey is a symmetrical key used to sign and validate the JWT when using the HMAC-SHA algorithm.
*/
func (svc *Intermediate) SecretKey() (key string) { // MARKER: SecretKey
	return svc.Config("SecretKey")
}

/*
SetSecretKey sets the value of the configuration property.
*/
func (svc *Intermediate) SetSecretKey(key string) (err error) { // MARKER: SecretKey
	return svc.SetConfig("SecretKey", key)
}

/*
AltSecretKey is an alternative key used to validate the JWT when using the HMAC-SHA algorithm.
Setting the previous secret key as an alternative key is useful when rotating keys.
*/
func (svc *Intermediate) AltSecretKey() (key string) { // MARKER: AltSecretKey
	return svc.Config("AltSecretKey")
}

/*
SetAltSecretKey sets the value of the configuration property.
*/
func (svc *Intermediate) SetAltSecretKey(key string) (err error) { // MARKER: AltSecretKey
	return svc.SetConfig("AltSecretKey", key)
}

// doIssueToken handles marshaling for IssueToken.
func (svc *Intermediate) doIssueToken(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: IssueToken
	var in tokenissuerapi.IssueTokenIn
	var out tokenissuerapi.IssueTokenOut
	err = marshalFunction(w, r, tokenissuerapi.IssueToken.Route, &in, &out, func(_ any, _ any) error {
		out.SignedToken, err = svc.IssueToken(r.Context(), in.Claims)
		return err
	})
	return err // No trace
}

// doValidateToken handles marshaling for ValidateToken.
func (svc *Intermediate) doValidateToken(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ValidateToken
	var in tokenissuerapi.ValidateTokenIn
	var out tokenissuerapi.ValidateTokenOut
	err = marshalFunction(w, r, tokenissuerapi.ValidateToken.Route, &in, &out, func(_ any, _ any) error {
		out.Claims, out.Valid, err = svc.ValidateToken(r.Context(), in.SignedToken)
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
