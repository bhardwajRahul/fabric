package sleepflow

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

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

	"github.com/microbus-io/fabric/verify/sleepflow/sleepflowapi"
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
	_ sleepflowapi.Client
)

func TestSleepflow_Delay(t *testing.T) { // MARKER: Delay
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := sleepflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("flow_sleeps_for_configured_duration", func(t *testing.T) {
		assert := testarossa.For(t)

		sleepFor := 100 * time.Millisecond
		start := time.Now()
		completed, status, err := exec.Delay(ctx, sleepFor)
		elapsed := time.Since(start)

		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			completed, true,
		)
		// Verify the flow waited at least the configured sleep duration.
		// Allow some slack on the upper bound (foreman's adaptive poll wakeup latency).
		assert.Expect(elapsed >= sleepFor, true)
	})
}
