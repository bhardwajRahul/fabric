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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/coreservices/accesstoken"
	"github.com/microbus-io/fabric/coreservices/bearertoken"
	"github.com/microbus-io/fabric/coreservices/chatgptllm"
	"github.com/microbus-io/fabric/coreservices/claudellm"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/geminillm"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/httpingress"
	"github.com/microbus-io/fabric/coreservices/httpingress/middleware"
	"github.com/microbus-io/fabric/coreservices/litellm"
	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/mcpportal"
	"github.com/microbus-io/fabric/coreservices/metrics"
	"github.com/microbus-io/fabric/coreservices/openapiportal"
	"github.com/microbus-io/fabric/devservices/agentstudio"

	"github.com/microbus-io/fabric/exampleservices/banksupport"
	"github.com/microbus-io/fabric/exampleservices/browser"
	"github.com/microbus-io/fabric/exampleservices/calculator"
	"github.com/microbus-io/fabric/exampleservices/chatbox"
	"github.com/microbus-io/fabric/exampleservices/creditflow"
	"github.com/microbus-io/fabric/exampleservices/embedder"
	"github.com/microbus-io/fabric/exampleservices/eventsink"
	"github.com/microbus-io/fabric/exampleservices/eventsource"
	"github.com/microbus-io/fabric/exampleservices/flightbooking"
	"github.com/microbus-io/fabric/exampleservices/hello"
	"github.com/microbus-io/fabric/exampleservices/helloworld"
	"github.com/microbus-io/fabric/exampleservices/login"
	"github.com/microbus-io/fabric/exampleservices/messaging"
	"github.com/microbus-io/fabric/exampleservices/weather"
	"github.com/microbus-io/fabric/exampleservices/yellowpages"
)

/*
main runs the example microservices.
*/
func main() {
	app := application.New()
	app.Add(
		// Configurator should start first
		configurator.NewService(),
	)
	app.Add(
		// Core microservices
		httpegress.NewService(),
		openapiportal.NewService(),
		mcpportal.NewService(),
		metrics.NewService(),
		bearertoken.NewService().Init(func(svc *bearertoken.Service) (err error) {
			svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
				// HINT: Enrich the claims of the external bearer token here
				return nil
			})
			return nil
		}),
		accesstoken.NewService().Init(func(svc *accesstoken.Service) (err error) {
			svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
				// HINT: Enrich the claims of the internal access token here
				return nil
			})
			return nil
		}),
		foreman.NewService(),
		llm.NewService(),
		claudellm.NewService(),
		chatgptllm.NewService(),
		geminillm.NewService(),
		litellm.NewService(),
	)
	app.Add(
		agentstudio.NewService(),
	)
	app.Add(
		// Example microservices
		helloworld.NewService(),
		hello.NewService(),
		messaging.NewService(),
		messaging.NewService(),
		messaging.NewService(),
		calculator.NewService(),
		eventsource.NewService(),
		eventsink.NewService(),
		yellowpages.NewService(),
		browser.NewService(),
		login.NewService(),
		embedder.NewService(),
		creditflow.NewService(),
		chatbox.NewService(),
		weather.NewService(),
		flightbooking.NewService(),
		banksupport.NewService(),
	)
	app.Add(
	// HINT: Add solution microservices here
	)
	app.Add(
		// When everything is ready, begin to accept external requests
		httpingress.NewService().Init(func(svc *httpingress.Service) (err error) {
			svc.Middleware().Append("LoginExample401Redirect",
				middleware.OnRoute(
					func(path string) bool {
						return strings.HasPrefix(path, "/"+login.Hostname+"/")
					},
					middleware.ErrorPageRedirect(http.StatusUnauthorized, "/"+login.Hostname+"/login"),
				),
			)
			svc.Middleware().Append("BankSupport401Redirect",
				middleware.OnRoute(
					func(path string) bool {
						return strings.HasPrefix(path, "/"+banksupport.Hostname+"/")
					},
					middleware.ErrorPageRedirect(http.StatusUnauthorized, "/"+banksupport.Hostname+"/login"),
				),
			)
			return nil
		}),
		// smtpingress.NewService(),
	)
	err := app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(19)
	}
}
