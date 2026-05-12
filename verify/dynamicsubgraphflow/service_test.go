package dynamicsubgraphflow

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

	"github.com/microbus-io/fabric/verify/dynamicsubgraphflow/dynamicsubgraphflowapi"
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
	_ dynamicsubgraphflowapi.Client
)

func TestDynamicsubgraphflow_DynamicSubgraph(t *testing.T) { // MARKER: DynamicSubgraph
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := dynamicsubgraphflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("parent_re_runs_after_dynamic_subgraph_completes", func(t *testing.T) {
		// value=5: Parent signals Subgraph(value=5). Inner runs:
		//   InnerA(5) -> innerStage=10
		//   InnerB(10) -> innerResult=13, innerDone=true
		// Parent re-runs with merged state, returns parentResult="parent:13".
		assert := testarossa.For(t)

		parentResult, status, err := exec.DynamicSubgraph(ctx, 5)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			parentResult, "parent:13",
		)
	})
}
