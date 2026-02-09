package eventsource

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/eventsource/eventsourceapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ eventsourceapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockRegister func(ctx context.Context, email string) (allowed bool, err error) // MARKER: Register
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

// MockRegister sets up a mock handler for Register.
func (svc *Mock) MockRegister(handler func(ctx context.Context, email string) (allowed bool, err error)) *Mock { // MARKER: Register
	svc.mockRegister = handler
	return svc
}

// Register executes the mock handler.
func (svc *Mock) Register(ctx context.Context, email string) (allowed bool, err error) { // MARKER: Register
	if svc.mockRegister == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	allowed, err = svc.mockRegister(ctx, email)
	return allowed, errors.Trace(err)
}
