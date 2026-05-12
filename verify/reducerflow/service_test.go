package reducerflow

import (
	"context"
	"io"
	"net/http"
	"sort"
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

	"github.com/microbus-io/fabric/verify/reducerflow/reducerflowapi"
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
	_ reducerflowapi.Client
)

func TestReducerflow_Reducer(t *testing.T) { // MARKER: Reducer
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := reducerflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("sum_list_and_set_reducers_apply", func(t *testing.T) {
		assert := testarossa.For(t)

		sum, list, set, status, err := exec.Reducer(ctx)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusCompleted)
		// sum* numeric add: 10 + 20 + 30
		assert.Expect(sum, 60)
		// list* append: order is by updated_at/step_id; sort to compare contents
		sort.Strings(list)
		assert.Expect(list, []string{"b", "c", "d"})
		// set* union: x appears in B and C; dedupe leaves {x, y, z}
		sort.Strings(set)
		assert.Expect(set, []string{"x", "y", "z"})
	})
}
