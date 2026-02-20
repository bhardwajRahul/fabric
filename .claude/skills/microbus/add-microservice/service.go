package myservice

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"

	"github.com/mycompany/myproject/myservice/myserviceapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ myserviceapi.Client
)

/*
Service implements myservice which does X.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}
