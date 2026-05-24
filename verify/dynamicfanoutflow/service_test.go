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

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := dynamicfanoutflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("three_elements", func(t *testing.T) {
		assert := testarossa.For(t)

		// The branches strip 'items' from their local state to avoid N^2 carry, but the
		// fan-in step reconstructs from the spawn step's immutable state so 'items'
		// resurfaces post-fan-in. itemIndex / itemCount prove the branch saw its position
		// and the cohort size.
		out, status, err := exec.DynamicFanOut(ctx, []string{"x", "y", "z"}, false)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			out.ProcessedCount, 3,
			out.ItemsOut, []string{"x", "y", "z"},
			out.ListSeenIndices, []int{0, 1, 2},
			out.SetSeenCounts, []int{3},
		)
	})

	t.Run("single_element", func(t *testing.T) {
		assert := testarossa.For(t)

		out, status, err := exec.DynamicFanOut(ctx, []string{"only"}, false)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			out.ProcessedCount, 1,
			out.ItemsOut, []string{"only"},
			out.ListSeenIndices, []int{0},
			out.SetSeenCounts, []int{1},
		)
	})

	t.Run("empty_list_completes_at_for_each_source", func(t *testing.T) {
		assert := testarossa.For(t)

		// With an empty list, TaskB never runs, so TaskC is never reached.
		// The flow completes at TaskA without producing a processedCount.
		out, status, err := exec.DynamicFanOut(ctx, []string{}, false)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			out.ProcessedCount, 0,
		)
	})

	t.Run("clear_items_in_branch_suppresses_downstream", func(t *testing.T) {
		assert := testarossa.For(t)

		// Each branch calls flow.Set("items", nil) which writes null into its changes.
		// At fan-in, the replace reducer folds that null over the spawn-step's array,
		// so 'items' is absent downstream of the fan-in even though it lived in the
		// original input. Useful when the source array is large and only the
		// transformation matters past the fan-in.
		out, status, err := exec.DynamicFanOut(ctx, []string{"x", "y", "z"}, true)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			out.ProcessedCount, 3,
			out.ItemsOut, []string(nil),
			out.ListSeenIndices, []int{0, 1, 2},
			out.SetSeenCounts, []int{3},
		)
	})
}
