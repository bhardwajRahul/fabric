/*
Copyright (c) 2023 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package httpingress implements the http.ingress.sys microservice.

The HTTP Ingress microservice relays incoming HTTP requests to the NATS bus.
*/
package httpingress

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"

	"github.com/microbus-io/fabric/services/httpingress/intermediate"
	"github.com/microbus-io/fabric/services/httpingress/httpingressapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ connector.Service
	_ *errors.TracedError
	_ *httpingressapi.Client
)

// HostName is the default host name of the microservice: http.ingress.sys.
const HostName = "http.ingress.sys"

// NewService creates a new http.ingress.sys microservice.
func NewService() connector.Service {
	s := &Service{}
	s.Intermediate = intermediate.NewService(s, Version)
	return s
}

// Mock is a mockable version of the http.ingress.sys microservice,
// allowing functions, sinks and web handlers to be mocked.
type Mock = intermediate.Mock

// New creates a new mockable version of the microservice.
func NewMock() *Mock {
	return intermediate.NewMock(Version)
}

// Config initializers
var (
	_ int
	// TimeBudget initializes the TimeBudget config property of the microservice
	TimeBudget = intermediate.TimeBudget
	// Ports initializes the Ports config property of the microservice
	Ports = intermediate.Ports
	// RequestMemoryLimit initializes the RequestMemoryLimit config property of the microservice
	RequestMemoryLimit = intermediate.RequestMemoryLimit
	// AllowedOrigins initializes the AllowedOrigins config property of the microservice
	AllowedOrigins = intermediate.AllowedOrigins
	// PortMappings initializes the PortMappings config property of the microservice
	PortMappings = intermediate.PortMappings
	// ReadTimeout initializes the ReadTimeout config property of the microservice
	ReadTimeout = intermediate.ReadTimeout
	// WriteTimeout initializes the WriteTimeout config property of the microservice
	WriteTimeout = intermediate.WriteTimeout
	// ReadHeaderTimeout initializes the ReadHeaderTimeout config property of the microservice
	ReadHeaderTimeout = intermediate.ReadHeaderTimeout
	// Middleware initializes the Middleware config property of the microservice
	Middleware = intermediate.Middleware
	// ServerLanguages initializes the ServerLanguages config property of the microservice
	ServerLanguages = intermediate.ServerLanguages
	// BlockedPaths initializes the BlockedPaths config property of the microservice
	BlockedPaths = intermediate.BlockedPaths
)
