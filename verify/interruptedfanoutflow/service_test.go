package interruptedfanoutflow

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

	"github.com/microbus-io/fabric/verify/interruptedfanoutflow/interruptedfanoutflowapi"
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
	_ interruptedfanoutflowapi.Client
)

// asInt coerces a state map value (absent -> nil, JSON number -> float64) to int.
func asInt(v any) int {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}

func TestInterruptedfanoutflow_InterruptedFanOut(t *testing.T) { // MARKER: InterruptedFanOut
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("interrupt_then_resume_completes_with_sum_3", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := fm.Create(ctx, interruptedfanoutflowapi.InterruptedFanOut.URL(), map[string]any{})
		if !assert.NoError(err) {
			return
		}
		err = fm.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		// B interrupts on its first run; the fan-in never reaches all three
		// arrivals, so the flow parks as interrupted.
		status, _, err := fm.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusInterrupted,
		)

		// Resume B with resumed=true. It re-runs, falls through, contributes 1.
		err = fm.Resume(ctx, flowKey, map[string]any{"resumed": true})
		if !assert.NoError(err) {
			return
		}

		status, state, err := fm.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
		)
		// A + B + C each contributed 1 via the sum* reducer at fan-in.
		assert.Expect(asInt(state["sumExecuted"]), 3)
		// J surfaced the summed value, proving the fan-in saw 3.
		assert.Expect(asInt(state["totalExecuted"]), 3)
	})
}
