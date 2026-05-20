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

package conditionalflow

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

	"github.com/microbus-io/fabric/verify/conditionalflow/conditionalflowapi"
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
	_ conditionalflowapi.Client
)

func TestConditionalflow_Conditional(t *testing.T) { // MARKER: Conditional
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := conditionalflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("score_high_takes_high_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 80)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "high",
		)
	})

	t.Run("score_low_takes_low_branch", func(t *testing.T) {
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 20)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "low",
		)
	})

	t.Run("boundary_50_takes_high_branch", func(t *testing.T) {
		// score>=50 matches high (inclusive)
		assert := testarossa.For(t)

		branch, status, err := exec.Conditional(ctx, 50)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			branch, "high",
		)
	})
}
