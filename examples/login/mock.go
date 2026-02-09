package login

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/login/loginapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ loginapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockLogin       func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Login
	mockLogout      func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Logout
	mockWelcome     func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Welcome
	mockAdminOnly   func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: AdminOnly
	mockManagerOnly func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: ManagerOnly
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockLogin sets up a mock handler for Login.
func (svc *Mock) MockLogin(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Login
	svc.mockLogin = handler
	return svc
}

// Login executes the mock handler.
func (svc *Mock) Login(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Login
	if svc.mockLogin == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockLogin(w, r)
	return errors.Trace(err)
}

// MockLogout sets up a mock handler for Logout.
func (svc *Mock) MockLogout(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Logout
	svc.mockLogout = handler
	return svc
}

// Logout executes the mock handler.
func (svc *Mock) Logout(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Logout
	if svc.mockLogout == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockLogout(w, r)
	return errors.Trace(err)
}

// MockWelcome sets up a mock handler for Welcome.
func (svc *Mock) MockWelcome(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Welcome
	svc.mockWelcome = handler
	return svc
}

// Welcome executes the mock handler.
func (svc *Mock) Welcome(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Welcome
	if svc.mockWelcome == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockWelcome(w, r)
	return errors.Trace(err)
}

// MockAdminOnly sets up a mock handler for AdminOnly.
func (svc *Mock) MockAdminOnly(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: AdminOnly
	svc.mockAdminOnly = handler
	return svc
}

// AdminOnly executes the mock handler.
func (svc *Mock) AdminOnly(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AdminOnly
	if svc.mockAdminOnly == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockAdminOnly(w, r)
	return errors.Trace(err)
}

// MockManagerOnly sets up a mock handler for ManagerOnly.
func (svc *Mock) MockManagerOnly(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: ManagerOnly
	svc.mockManagerOnly = handler
	return svc
}

// ManagerOnly executes the mock handler.
func (svc *Mock) ManagerOnly(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ManagerOnly
	if svc.mockManagerOnly == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockManagerOnly(w, r)
	return errors.Trace(err)
}
