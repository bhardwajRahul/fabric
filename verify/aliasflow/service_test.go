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

package aliasflow

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

	"github.com/microbus-io/fabric/verify/aliasflow/aliasflowapi"
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
	_ aliasflowapi.Client
)


// outcomeStatus extracts the Status from a FlowOutcome, returning "" on nil.
func outcomeStatus(o *workflow.FlowOutcome) string {
	if o == nil {
		return ""
	}
	return o.Status
}

// outcomeState extracts the State from a FlowOutcome, returning nil on nil.
func outcomeState(o *workflow.FlowOutcome) map[string]any {
	if o == nil {
		return nil
	}
	return o.State
}

// outcomeStatusState extracts the Status and State from a FlowOutcome.
func outcomeStatusState(o *workflow.FlowOutcome) (string, map[string]any) {
	if o == nil {
		return "", nil
	}
	return o.Status, o.State
}

func TestAliasflow_Alias(t *testing.T) { // MARKER: Alias
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := aliasflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetSQLConnectionPool(1) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("default_path_runs_s_a_b_c", func(t *testing.T) {
		assert := testarossa.For(t)

		path, status, err := exec.Alias(ctx, "")
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			path, "ABC",
		)
	})

	t.Run("alt_path_runs_s_bPrime_d", func(t *testing.T) {
		assert := testarossa.For(t)

		path, status, err := exec.Alias(ctx, "alt")
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			path, "BD",
		)
	})

	t.Run("history_distinguishes_b_and_bPrime_by_node_name", func(t *testing.T) {
		assert := testarossa.For(t)

		// Run the default path: history should include "b" but not "bPrime".
		flowKey, err := foremanClient.Create(ctx, aliasflowapi.Alias.URL(), aliasflowapi.AliasIn{Branch: ""}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		outcome, err := foremanClient.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)

		history, err := foremanClient.History(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		nodeNames := map[string]int{}
		for _, s := range history {
			nodeNames[s.TaskName]++
		}
		assert.Expect(nodeNames["s"], 1)
		assert.Expect(nodeNames["a"], 1)
		assert.Expect(nodeNames["b"], 1)
		assert.Expect(nodeNames["c"], 1)
		assert.Expect(nodeNames["bPrime"], 0)
		assert.Expect(nodeNames["d"], 0)

		// Run the alt path: history should include "bPrime" but not "b".
		flowKey, err = foremanClient.Create(ctx, aliasflowapi.Alias.URL(), aliasflowapi.AliasIn{Branch: "alt"}, nil)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		outcome, err = foremanClient.Await(ctx, flowKey)

		status = outcomeStatus(outcome)
		if !assert.NoError(err) {
			return
		}
		assert.Expect(status, workflow.StatusCompleted)

		history, err = foremanClient.History(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		nodeNames = map[string]int{}
		for _, s := range history {
			nodeNames[s.TaskName]++
		}
		assert.Expect(nodeNames["s"], 1)
		assert.Expect(nodeNames["bPrime"], 1)
		assert.Expect(nodeNames["d"], 1)
		assert.Expect(nodeNames["a"], 0)
		assert.Expect(nodeNames["b"], 0)
		assert.Expect(nodeNames["c"], 0)
	})
}
