package continueflow

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

	"github.com/microbus-io/fabric/verify/continueflow/continueflowapi"
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
	_ continueflowapi.Client
)

func TestContinueflow_Counting(t *testing.T) { // MARKER: Counting
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

	t.Run("counter_persists_across_continue_turns", func(t *testing.T) {
		assert := testarossa.For(t)

		// Turn 1: create + start a flow starting from counter=0.
		flowKey1, err := foremanClient.Create(ctx, continueflowapi.Counting.URL(), map[string]any{"counter": 0})
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey1)
		if !assert.NoError(err) {
			return
		}
		status, state, err := foremanClient.Await(ctx, flowKey1)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusCompleted)
		// counter advanced 0 -> 1
		assert.Expect(state["counter"], 1.0) // JSON unmarshal turns ints into float64

		// Turn 2: continue from the thread, no additional state.
		flowKey2, err := foremanClient.Continue(ctx, flowKey1, map[string]any{})
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey2)
		if !assert.NoError(err) {
			return
		}
		status, state, err = foremanClient.Await(ctx, flowKey2)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, foremanapi.StatusCompleted)
		// counter advanced 1 -> 2 by carrying forward across the Continue boundary
		assert.Expect(state["counter"], 2.0)
	})
}
