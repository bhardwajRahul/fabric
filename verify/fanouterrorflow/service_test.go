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

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := fanouterrorflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
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
		assert.Expect(status, workflow.StatusCompleted)
	})

	t.Run("handler_runs_and_state_reaches_taskE", func(t *testing.T) {
		// With the lineage-based fan-in (graph.SetFanIn("taskE")), the OnError
		// path is coordinated by the spawn cohort, not by step_depth, so the
		// old depth-N+1 collision between Handler and the fan-in target cannot
		// occur: a sibling completing before B's errorRouted finishes no longer
		// blocks Handler's insert. Handler must run and TaskE must observe
		// recovered=true. Stress with -count=N to surface any regression.
		assert := testarossa.For(t)

		recovered, status, err := exec.FanOutError(ctx)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			recovered, true,
		)
	})
}
