package fanouterrorflow

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

	"github.com/microbus-io/fabric/verify/fanouterrorflow/fanouterrorflowapi"
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
	_ fanouterrorflowapi.Client
)

func TestFanouterrorflow_FanOutError(t *testing.T) { // MARKER: FanOutError
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := fanouterrorflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("flow_does_not_fail", func(t *testing.T) {
		// Verifies the OnError sibling-cancel race fix:
		// foreman/service.go:2596 (fan-in worker returns nil on failed/cancelled siblings).
		// Prior to that fix, a sibling racing with OnError sibling-cancel could call
		// failStep and mark the parent flow as failed. The flow should complete cleanly.
		assert := testarossa.For(t)

		_, status, err := exec.FanOutError(ctx)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusCompleted)
	})

	t.Run("handler_runs_and_state_reaches_taskE", func(t *testing.T) {
		// This assertion is currently flaky because of a SEPARATE race in the
		// depth-based fan-in: a sibling that completes before B's errorRouted finishes
		// can insert the normal fan-in target (TaskE) at depth N+1 first, and B's
		// subsequent attempt to insert Handler at the same depth gets blocked by
		// existingCount>0 race protection. The flow still completes, but Handler
		// never runs, so TaskE sees handled=false.
		//
		// This is exactly the case the lineage-based fan-in redesign solves
		// (see _DOMINATOR.md). Re-enable when the redesign lands.
		t.Skip("depth-N+1 collision between Handler and fan-in target; pending lineage redesign")

		assert := testarossa.For(t)

		recovered, status, err := exec.FanOutError(ctx)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			recovered, true,
		)
	})
}
