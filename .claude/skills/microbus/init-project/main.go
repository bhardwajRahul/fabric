package main

import (
	"context"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/coreservices/accesstoken"
	"github.com/microbus-io/fabric/coreservices/bearertoken"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/httpingress"
	"github.com/microbus-io/fabric/coreservices/openapiportal"
)

/*
main runs the application.
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
		// metrics.NewService(),
	)
	app.Add(
	// HINT: Add solution microservices here
	)
	app.Add(
		// When everything is ready, begin to accept external requests
		httpingress.NewService(),
		// smtpingress.NewService(),
	)
	err := app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(19)
	}
}
