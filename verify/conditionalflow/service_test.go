package conditionalflow

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

	"github.com/microbus-io/fabric/verify/conditionalflow/conditionalflowapi"
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
	_ conditionalflowapi.Client
)

func TestConditionalflow_Conditional(t *testing.T) { // MARKER: Conditional
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := conditionalflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("score_high_takes_high_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 80)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			branch, "high",
		)
	})

	t.Run("score_low_takes_low_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 20)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			branch, "low",
		)
	})

	t.Run("boundary_50_takes_high_branch", func(t *testing.T) {
		// score>=50 matches high (inclusive)
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 50)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			branch, "high",
		)
	})
}
