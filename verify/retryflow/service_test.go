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

package retryflow

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

	"github.com/microbus-io/fabric/verify/retryflow/retryflowapi"
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
	_ retryflowapi.Client
)

func TestRetryflow_Retry(t *testing.T) { // MARKER: Retry
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := retryflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("succeeds_on_target_attempt", func(t *testing.T) {
		// target=3, cap=5: Flaky retries twice then succeeds on attempt 3.
		assert := testarossa.For(t)

		finalAttempts, status, err := exec.Retry(ctx, 3)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			finalAttempts, 3,
		)
	})

	t.Run("exhausts_retries_and_fails", func(t *testing.T) {
		// target=10, cap=5: Flaky retries until cap is hit, then returns the error.
		// foremanapi.Run returns no error for a failed flow - the failure surfaces
		// via status=failed and the caller is expected to inspect final state for
		// the error context.
		assert := testarossa.For(t)

		_, status, err := exec.Retry(ctx, 10)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusFailed)
	})
}
