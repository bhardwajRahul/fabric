package switchflow

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

	"github.com/microbus-io/fabric/verify/switchflow/switchflowapi"
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
	_ switchflowapi.Client
)

func TestSwitchflow_Switch(t *testing.T) { // MARKER: Switch
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := switchflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("amount_above_high_threshold_takes_high_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Switch(ctx, 50000)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "high",
		)
	})

	t.Run("amount_in_mid_band_takes_mid_branch", func(t *testing.T) {
		// 5000 fails the >=10000 predicate, satisfies the >=1000 predicate.
		assert := testarossa.For(t)

		branch, status, err := exec.Switch(ctx, 5000)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "mid",
		)
	})

	t.Run("amount_below_thresholds_takes_default_branch", func(t *testing.T) {
		// 100 fails both predicates; only the when="true" arm matches.
		assert := testarossa.For(t)

		branch, status, err := exec.Switch(ctx, 100)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "low",
		)
	})

	t.Run("boundary_10000_takes_high_branch", func(t *testing.T) {
		// amount==10000 satisfies the first predicate inclusively; later arms are skipped
		// even though the when="true" default would also match. First-match wins.
		assert := testarossa.For(t)

		branch, status, err := exec.Switch(ctx, 10000)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "high",
		)
	})

	t.Run("boundary_1000_takes_mid_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Switch(ctx, 1000)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "mid",
		)
	})
}

func TestSwitchflow_SwitchNoMatch(t *testing.T) { // MARKER: SwitchNoMatch
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := switchflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("no_match_completes_flow_without_branching", func(t *testing.T) {
		// 100 fails every Switch predicate (no default arm exists). The router step
		// completes, no successor is created, and the flow transitions to completed
		// with the side-channel branch field absent (zero value).
		assert := testarossa.For(t)

		branch, status, err := exec.SwitchNoMatch(ctx, 100)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "",
		)
	})

	t.Run("matching_input_still_routes_normally", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.SwitchNoMatch(ctx, 5000)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "mid",
		)
	})
}
