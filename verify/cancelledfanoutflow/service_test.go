/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cancelledfanoutflow

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

	"github.com/microbus-io/fabric/verify/cancelledfanoutflow/cancelledfanoutflowapi"
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
	_ cancelledfanoutflowapi.Client
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

func TestCancelledfanoutflow_CancelledFanOut(t *testing.T) { // MARKER: CancelledFanOut
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		// Pin the foreman to a single worker. SetConfig is only permitted once the
		// connector is in the TESTING deployment, so it must run via .Init (which
		// fires during connector init) rather than a bare post-creation call.
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetWorkers(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("cancel_mid_fan_out", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := fm.Create(ctx, cancelledfanoutflowapi.CancelledFanOut.URL(), map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
		err = fm.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		// The single worker picks up exactly one branch, which then blocks for 2s.
		// Cancel after 1s, while that branch is mid-sleep.
		time.Sleep(1 * time.Second)
		err = fm.Cancel(ctx, flowKey)
		assert.NoError(err)

		status, state, err := fm.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCancelled,
		)
		// Only the branch the lone worker picked up entered execution; the other
		// two were cancelled while still pending and never ran.
		assert.Expect(svc.Executed(), 1)
		// The running branch's result was discarded and the fan-in (J) never ran,
		// so the summed sumExecuted is absent/0 in the final state.
		assert.Expect(asInt(state["sumExecuted"]), 0)
		assert.Expect(asInt(state["totalExecuted"]), 0)
	})
}

// TestCancelledfanoutflow_AwaitTimeoutDoesNotInvalidate validates that a timed-out
// synchronous Await/Run does NOT leave the flow in an invalid or stuck state. The
// concern: a cancelled request ctx could break a foreman DB write mid-fan-in. The
// foreman's workers are decoupled from the Await ctx, so a fresh Await must still
// observe the flow run to clean, correct completion.
func TestCancelledfanoutflow_AwaitTimeoutDoesNotInvalidate(t *testing.T) {
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

	t.Run("timed_out_await_then_fresh_await_completes", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := fm.Create(ctx, cancelledfanoutflowapi.CancelledFanOut.URL(), map[string]any{}, nil)
		if !assert.NoError(err) {
			return
		}
		err = fm.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}

		// Await with a context that expires while A/B/C are still sleeping (~2s),
		// forcing the synchronous Await path to abort mid-flight.
		shortCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		_, _, errShort := fm.Await(shortCtx, flowKey)
		cancel()
		assert.Expect(errShort != nil, true) // deadline exceeded; flow still running

		// A fresh Await must still see the flow complete correctly. If a cancelled
		// ctx had broken a foreman write mid-fan-in, this would hang or return a
		// non-terminal/corrupted result.
		status, state, err := fm.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
		)
		assert.Expect(asInt(state["sumExecuted"]), 3) // A+B+C all contributed; fan-in intact
	})
}
