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

package onerrorsiblingsflow

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

	"github.com/microbus-io/fabric/verify/onerrorsiblingsflow/onerrorsiblingsflowapi"
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
	_ onerrorsiblingsflowapi.Client
)

// TestOnerrorsiblingsflow_FanOutError verifies that OnError routing on a cohort member does NOT
// cancel its cohort siblings: A1 errors and routes to a Handler, while A2 and A3 keep running.
// All four contributions (Handler, A2, A3 — A1 has been recovered through Handler) merge at the
// fan-in J and the flow completes successfully.
func TestOnerrorsiblingsflow_FanOutError(t *testing.T) { // MARKER: FanOutError
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := onerrorsiblingsflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("flow_completes_with_handler_and_siblings", func(t *testing.T) {
		assert := testarossa.For(t)

		recovered, siblingsRan, status, err := exec.FanOutError(ctx)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			recovered, true,    // Handler ran and TaskB never marked markB
			siblingsRan, true,  // TaskC and TaskD both ran to completion (not cancelled by OnError routing)
		)
	})
}
