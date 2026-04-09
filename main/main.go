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
	"github.com/microbus-io/fabric/coreservices/claudellm"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/geminillm"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/httpingress"
	"github.com/microbus-io/fabric/coreservices/httpingress/middleware"
	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/metrics"
	"github.com/microbus-io/fabric/coreservices/openaillm"
	"github.com/microbus-io/fabric/coreservices/openapiportal"
	"github.com/microbus-io/fabric/examples/browser"
	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/chatbox"
	"github.com/microbus-io/fabric/examples/creditflow"
	"github.com/microbus-io/fabric/examples/eventsink"
	"github.com/microbus-io/fabric/examples/eventsource"
	"github.com/microbus-io/fabric/examples/hello"
	"github.com/microbus-io/fabric/examples/helloworld"
	"github.com/microbus-io/fabric/examples/login"
	"github.com/microbus-io/fabric/examples/messaging"
	"github.com/microbus-io/fabric/examples/yellowpages"
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
		openaillm.NewService(),
		geminillm.NewService(),
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
	)
	app.Add(
		// HINT: Add solution microservices here
		creditflow.NewService(),
		chatbox.NewService(),
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
