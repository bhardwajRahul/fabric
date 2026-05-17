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

package errorflow

import (
	"context"
	"io"
	"net/http"
	"strings"
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

	"github.com/microbus-io/fabric/verify/errorflow/errorflowapi"
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
	_ errorflowapi.Client
)

func TestErrorflow_Error(t *testing.T) { // MARKER: Error
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := errorflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("normal_path", func(t *testing.T) {
		assert := testarossa.For(t)

		finalResult, status, err := exec.Error(ctx, "ok")
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			finalResult, "final:normal",
		)
	})

	t.Run("error_handled_path", func(t *testing.T) {
		assert := testarossa.For(t)

		finalResult, status, err := exec.Error(ctx, "fail")
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusCompleted)
		// finalResult is "final:recovered:triggered failure\nstatusCode=500\n..."; check prefix
		assert.Expect(strings.HasPrefix(finalResult, "final:recovered:triggered failure"), true)
	})
}
