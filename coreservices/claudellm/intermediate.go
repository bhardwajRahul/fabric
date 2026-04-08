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

package claudellm

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
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
	"github.com/microbus-io/fabric/coreservices/claudellm/resources"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
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
	_ claudellmapi.Client
	_ *workflow.Flow
)

const (
	Hostname = claudellmapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) // MARKER: Turn
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
	svc.SetDescription(`The Claude LLM provider microservice implements the Turn endpoint for the Anthropic Claude API.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(claudellmapi.Turn.Method, claudellmapi.Turn.Route, svc.doTurn) // MARKER: Turn

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: BaseURL
		"BaseURL",
		cfg.Description(`BaseURL is the base URL of the Claude API.`),
		cfg.DefaultValue("https://api.anthropic.com"),
		cfg.Validation("url"),
	)
	svc.DefineConfig( // MARKER: APIKey
		"APIKey",
		cfg.Description(`APIKey is the API key for the Claude API.`),
		cfg.Secret(),
	)
	svc.DefineConfig( // MARKER: Model
		"Model",
		cfg.Description(`Model is the Claude model identifier to use.`),
		cfg.DefaultValue("claude-haiku-4-5"),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
}

// doTurn handles marshaling for Turn.
func (svc *Intermediate) doTurn(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Turn
	var in claudellmapi.TurnIn
	var out claudellmapi.TurnOut
	err = marshalFunction(w, r, claudellmapi.Turn.Route, &in, &out, func(_ any, _ any) error {
		out.Completion, err = svc.Turn(r.Context(), in.Messages, in.Tools)
		return err // No trace
	})
	return err // No trace
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
		{ // MARKER: Turn
			Type:    "function",
			Name:    "Turn",
			Method:  claudellmapi.Turn.Method,
			Route:   claudellmapi.Turn.Route,
			Summary: "Turn(messages []Message, tools []ToolDef) (completion *TurnCompletion)",
			Description: `Turn executes a single LLM turn using the Claude provider.

Input:
  - messages: messages is the conversation history
  - tools: tools is the resolved tool definitions

Output:
  - completion: completion is the LLM's response`,
			InputArgs:  claudellmapi.TurnIn{},
			OutputArgs: claudellmapi.TurnOut{},
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
BaseURL is the base URL of the Claude API.
*/
func (svc *Intermediate) BaseURL() (value string) { // MARKER: BaseURL
	return svc.Config("BaseURL")
}

/*
SetBaseURL sets the value of the configuration property.
*/
func (svc *Intermediate) SetBaseURL(value string) (err error) { // MARKER: BaseURL
	return svc.SetConfig("BaseURL", value)
}

/*
APIKey is the API key for the Claude API.
*/
func (svc *Intermediate) APIKey() (value string) { // MARKER: APIKey
	return svc.Config("APIKey")
}

/*
SetAPIKey sets the value of the configuration property.
*/
func (svc *Intermediate) SetAPIKey(value string) (err error) { // MARKER: APIKey
	return svc.SetConfig("APIKey", value)
}

/*
Model is the Claude model identifier to use.
*/
func (svc *Intermediate) Model() (value string) { // MARKER: Model
	return svc.Config("Model")
}

/*
SetModel sets the value of the configuration property.
*/
func (svc *Intermediate) SetModel(value string) (err error) { // MARKER: Model
	return svc.SetConfig("Model", value)
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
