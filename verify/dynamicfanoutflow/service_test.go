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

package dynamicfanoutflow

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

	"github.com/microbus-io/fabric/verify/dynamicfanoutflow/dynamicfanoutflowapi"
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
	_ dynamicfanoutflowapi.Client
)

func TestDynamicfanoutflow_DynamicFanOut(t *testing.T) { // MARKER: DynamicFanOut
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := dynamicfanoutflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	t.Run("three_elements", func(t *testing.T) {
		assert := testarossa.For(t)

		count, status, err := exec.DynamicFanOut(ctx, []string{"x", "y", "z"})
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			count, 3,
		)
	})

	t.Run("single_element", func(t *testing.T) {
		assert := testarossa.For(t)

		count, status, err := exec.DynamicFanOut(ctx, []string{"only"})
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			count, 1,
		)
	})

	t.Run("empty_list_completes_at_for_each_source", func(t *testing.T) {
		assert := testarossa.For(t)

		// With an empty list, TaskB never runs, so TaskC is never reached.
		// The flow completes at TaskA without producing a processedCount.
		count, status, err := exec.DynamicFanOut(ctx, []string{})
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			count, 0,
		)
	})
}
