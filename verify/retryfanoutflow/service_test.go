package retryfanoutflow

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

	"github.com/microbus-io/fabric/verify/retryfanoutflow/retryfanoutflowapi"
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
	_ retryfanoutflowapi.Client
)

func TestRetryfanoutflow_RetryFanOut(t *testing.T) { // MARKER: RetryFanOut
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := retryfanoutflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("ordered_despite_random_retries", func(t *testing.T) {
		assert := testarossa.For(t)

		// Input [0..99]; each branch increments its element by one. Despite a random
		// 10% per-attempt failure scrambling completion order, the list* reducer
		// appends in fan_out_ordinal order, so listResult must be exactly [1..100].
		input := make([]int, 100)
		for i := range input {
			input[i] = i
		}
		want := make([]int, 100)
		for i := range want {
			want[i] = i + 1
		}

		listResult, status, err := exec.RetryFanOut(ctx, input)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			listResult, want,
		)
	})
}
