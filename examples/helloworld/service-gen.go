/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package helloworld implements the helloworld.example microservice.

The HelloWorld microservice demonstrates the minimalist classic example.
*/
package helloworld

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/service"

	"github.com/microbus-io/fabric/examples/helloworld/intermediate"
	"github.com/microbus-io/fabric/examples/helloworld/helloworldapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ service.Service
	_ *errors.TracedError
	_ *helloworldapi.Client
)

// Hostname is the default hostname of the microservice: helloworld.example.
const Hostname = "helloworld.example"

// NewService creates a new helloworld.example microservice.
func NewService() *Service {
	s := &Service{}
	s.Intermediate = intermediate.NewService(s, Version)
	return s
}

// Mock is a mockable version of the helloworld.example microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock = intermediate.Mock

// New creates a new mockable version of the microservice.
func NewMock() *Mock {
	return intermediate.NewMock()
}

/*
Init enables a single-statement pattern for initializing the microservice.

	svc.Init(func(svc Service) {
		svc.SetGreeting("Hello")
	})
*/
func (svc *Service) Init(initializer func(svc *Service)) *Service {
	initializer(svc)
	return svc
}