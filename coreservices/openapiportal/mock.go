package openapiportal

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/coreservices/openapiportal/openapiportalapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ openapiportalapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockList func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: List
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

// MockList sets up a mock handler for List.
func (svc *Mock) MockList(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: List
	svc.mockList = handler
	return svc
}

// List executes the mock handler.
func (svc *Mock) List(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	if svc.mockList == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockList(w, r)
	return errors.Trace(err)
}
