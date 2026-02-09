package browser

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/browser/browserapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ browserapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockBrowse func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Browse
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

// MockBrowse sets up a mock handler for Browse.
func (svc *Mock) MockBrowse(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Browse
	svc.mockBrowse = handler
	return svc
}

// Browse executes the mock handler.
func (svc *Mock) Browse(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Browse
	if svc.mockBrowse == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockBrowse(w, r)
	return errors.Trace(err)
}
