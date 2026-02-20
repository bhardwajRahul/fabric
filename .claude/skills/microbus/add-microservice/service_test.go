package myservice

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/mycompany/myproject/myservice/myserviceapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ testarossa.Asserter
	_ myserviceapi.Client
)

func TestMyService_OpenAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	rePort := regexp.MustCompile(`:([0-9]+)(/|$)`)
	routes := []string{
		// HINT: Insert routes of functional and web endpoints here
	}
	for _, route := range routes {
		port := "443"
		matches := rePort.FindStringSubmatch(route)
		if len(matches) > 1 {
			port = matches[1]
		}
		t.Run("port_"+port, func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := tester.Request(
				ctx,
				pub.GET(httpx.JoinHostAndPath(myserviceapi.Hostname, ":"+port+"/openapi.json")),
			)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.Contains(body, "openapi")
					assert.Contains(body, route)
				}
			}
		})
	}
}

func TestMyService_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})
}
