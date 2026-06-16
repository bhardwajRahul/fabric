package foreman

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *regexp.Regexp
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ httpx.BodyReader
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ *workflow.Flow
	_ testarossa.Asserter
	_ foremanapi.Client
)

// TestForeman_StartsEngine confirms the service builds and starts the embedded dwarf engine under the
// TESTING deployment (the StartupInTest(plane) path), and that Shutdown drains it cleanly via t.Cleanup.
func TestForeman_StartsEngine(t *testing.T) {
	assert := testarossa.For(t)

	svc := NewService()
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	// If OnStartup's engine wiring (build + StartupInTest) failed, RunInTest would have failed the test.
	assert.NotNil(svc.engine)
}
