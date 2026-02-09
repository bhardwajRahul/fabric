package helloworld

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/helloworld/helloworldapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ helloworldapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockHelloWorld func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: HelloWorld
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

// MockHelloWorld sets up a mock handler for HelloWorld.
func (svc *Mock) MockHelloWorld(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: HelloWorld
	svc.mockHelloWorld = handler
	return svc
}

// HelloWorld executes the mock handler.
func (svc *Mock) HelloWorld(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HelloWorld
	if svc.mockHelloWorld == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockHelloWorld(w, r)
	return errors.Trace(err)
}
