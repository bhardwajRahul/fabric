package interruptflow

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

	"github.com/microbus-io/fabric/verify/interruptflow/interruptflowapi"
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
	_ interruptflowapi.Client
)

func TestInterruptflow_Interruptor(t *testing.T) { // MARKER: Interruptor
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("interrupt_then_resume_completes_flow", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create + Start the flow. AwaitInput will interrupt because userInput is missing.
		flowKey, err := foremanClient.Create(ctx, interruptflowapi.Interruptor.URL(), map[string]any{"prompt": "Hello"})
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		// Await for the flow to stop. It should stop at interrupted, not completed.
		status, _, err := foremanClient.Await(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusInterrupted)

		// Resume with the missing userInput. AwaitInput re-runs and falls through.
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"userInput": "world"})
		if !assert.NoError(err) {
			return
		}

		// Await again. Now the flow should complete and the result should be "Hello, world".
		status, state, err := foremanClient.Await(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusCompleted)
		assert.Expect(state["result"], "Hello, world")
	})
}
