package tokenissuer

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ tokenissuerapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockIssueToken    func(ctx context.Context, claims tokenissuerapi.MapClaims) (signedToken string, err error)                        // MARKER: IssueToken
	mockValidateToken func(ctx context.Context, signedToken string) (claims tokenissuerapi.MapClaims, valid bool, err error) // MARKER: ValidateToken
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

// MockIssueToken sets up a mock handler for IssueToken.
func (svc *Mock) MockIssueToken(handler func(ctx context.Context, claims tokenissuerapi.MapClaims) (signedToken string, err error)) *Mock { // MARKER: IssueToken
	svc.mockIssueToken = handler
	return svc
}

// IssueToken executes the mock handler.
func (svc *Mock) IssueToken(ctx context.Context, claims tokenissuerapi.MapClaims) (signedToken string, err error) { // MARKER: IssueToken
	if svc.mockIssueToken == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	signedToken, err = svc.mockIssueToken(ctx, claims)
	return signedToken, errors.Trace(err)
}

// MockValidateToken sets up a mock handler for ValidateToken.
func (svc *Mock) MockValidateToken(handler func(ctx context.Context, signedToken string) (claims tokenissuerapi.MapClaims, valid bool, err error)) *Mock { // MARKER: ValidateToken
	svc.mockValidateToken = handler
	return svc
}

// ValidateToken executes the mock handler.
func (svc *Mock) ValidateToken(ctx context.Context, signedToken string) (claims tokenissuerapi.MapClaims, valid bool, err error) { // MARKER: ValidateToken
	if svc.mockValidateToken == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	claims, valid, err = svc.mockValidateToken(ctx, signedToken)
	return claims, valid, errors.Trace(err)
}
