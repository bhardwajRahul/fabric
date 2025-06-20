/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

// Code generated by Microbus. DO NOT EDIT.

package intermediate

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"

	"github.com/microbus-io/fabric/examples/login/loginapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ loginapi.Client
)

// Mock is a mockable version of the login.example microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockLogin func(w http.ResponseWriter, r *http.Request) (err error)
	mockLogout func(w http.ResponseWriter, r *http.Request) (err error)
	mockWelcome func(w http.ResponseWriter, r *http.Request) (err error)
	mockAdminOnly func(w http.ResponseWriter, r *http.Request) (err error)
	mockManagerOnly func(w http.ResponseWriter, r *http.Request) (err error)
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	m := &Mock{}
	m.Intermediate = NewService(m, 7357) // Stands for TEST
	return m
}

// OnStartup makes sure that the mock is not executed in a non-dev environment.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in '%s' deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is a no op.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockLogin sets up a mock handler for the Login endpoint.
func (svc *Mock) MockLogin(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock {
	svc.mockLogin = handler
	return svc
}

// Login runs the mock handler set by MockLogin.
func (svc *Mock) Login(w http.ResponseWriter, r *http.Request) (err error) {
	if svc.mockLogin == nil {
		return errors.New("mocked endpoint 'Login' not implemented")
	}
	err = svc.mockLogin(w, r)
	return errors.Trace(err)
}

// MockLogout sets up a mock handler for the Logout endpoint.
func (svc *Mock) MockLogout(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock {
	svc.mockLogout = handler
	return svc
}

// Logout runs the mock handler set by MockLogout.
func (svc *Mock) Logout(w http.ResponseWriter, r *http.Request) (err error) {
	if svc.mockLogout == nil {
		return errors.New("mocked endpoint 'Logout' not implemented")
	}
	err = svc.mockLogout(w, r)
	return errors.Trace(err)
}

// MockWelcome sets up a mock handler for the Welcome endpoint.
func (svc *Mock) MockWelcome(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock {
	svc.mockWelcome = handler
	return svc
}

// Welcome runs the mock handler set by MockWelcome.
func (svc *Mock) Welcome(w http.ResponseWriter, r *http.Request) (err error) {
	if svc.mockWelcome == nil {
		return errors.New("mocked endpoint 'Welcome' not implemented")
	}
	err = svc.mockWelcome(w, r)
	return errors.Trace(err)
}

// MockAdminOnly sets up a mock handler for the AdminOnly endpoint.
func (svc *Mock) MockAdminOnly(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock {
	svc.mockAdminOnly = handler
	return svc
}

// AdminOnly runs the mock handler set by MockAdminOnly.
func (svc *Mock) AdminOnly(w http.ResponseWriter, r *http.Request) (err error) {
	if svc.mockAdminOnly == nil {
		return errors.New("mocked endpoint 'AdminOnly' not implemented")
	}
	err = svc.mockAdminOnly(w, r)
	return errors.Trace(err)
}

// MockManagerOnly sets up a mock handler for the ManagerOnly endpoint.
func (svc *Mock) MockManagerOnly(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock {
	svc.mockManagerOnly = handler
	return svc
}

// ManagerOnly runs the mock handler set by MockManagerOnly.
func (svc *Mock) ManagerOnly(w http.ResponseWriter, r *http.Request) (err error) {
	if svc.mockManagerOnly == nil {
		return errors.New("mocked endpoint 'ManagerOnly' not implemented")
	}
	err = svc.mockManagerOnly(w, r)
	return errors.Trace(err)
}
