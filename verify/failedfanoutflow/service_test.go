package failedfanoutflow

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/failedfanoutflow/failedfanoutflowapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ httpx.BodyReader
	_ *workflow.Flow
	_ testarossa.Asserter
	_ failedfanoutflowapi.Client
)

func TestFailedfanoutflow_FailedFanOut(t *testing.T) { // MARKER: FailedFanOut
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := failedfanoutflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("failing_branch_fails_the_flow", func(t *testing.T) {
		assert := testarossa.For(t)

		// B always errors and there is no OnError transition, so failStep cascades
		// the whole flow to failed regardless of how A/C race. Flow failure is an
		// outcome status, not a transport error, so err is nil.
		_, _, status, err := exec.FailedFanOut(ctx)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusFailed)
	})
}
