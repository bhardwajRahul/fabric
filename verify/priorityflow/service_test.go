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

package priorityflow

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

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

	"github.com/microbus-io/fabric/verify/priorityflow/priorityflowapi"
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
	_ priorityflowapi.Client
)

func TestPriorityflow_Priority(t *testing.T) { // MARKER: Priority
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		// Pin the foreman to a single worker. SetConfig is only permitted once the
		// connector is in the TESTING deployment, so it must run via .Init (which
		// fires during connector init) rather than a bare post-creation call.
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetWorkers(1) }),
		tester,
	)
	app.RunInTest(t)

	// startFlow creates and starts one Priority flow at the given priority,
	// tagging it so the recorded dispatch order is identifiable.
	startFlow := func(assert *testarossa.Asserter, tag string, priority int, delayMs int) string {
		flowKey, err := fm.Create(ctx, priorityflowapi.Priority.URL(),
			map[string]any{"tag": tag, "delayMs": delayMs},
			&workflow.FlowOptions{Priority: priority},
		)
		if !assert.NoError(err) {
			return ""
		}
		err = fm.Start(ctx, flowKey)
		assert.NoError(err)
		return flowKey
	}

	t.Run("strict_priority_ordering", func(t *testing.T) {
		assert := testarossa.For(t)

		// The holder is the global minimum priority, so it is always selected
		// first and occupies the lone worker while the test set is created. By
		// the time it finishes, the whole set is pending and the worker drains
		// it strictly by the foreman's selection (single fairness key).
		holder := startFlow(assert, "_holder", 1, 1500)

		// (tag, priority) created in an order that does not match priority order.
		type spec struct {
			tag      string
			priority int
		}
		specs := []spec{
			{"p5a", 5}, {"p2a", 2}, {"p9a", 9}, {"p2b", 2}, {"p5b", 5}, {"p3a", 3},
		}
		var keys []string
		for _, s := range specs {
			keys = append(keys, startFlow(assert, s.tag, s.priority, 50))
		}

		// Ensure every test flow is committed pending before the holder frees.
		time.Sleep(300 * time.Millisecond)

		status, _, err := fm.Await(ctx, holder)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusCompleted)
		for _, k := range keys {
			status, _, err := fm.Await(ctx, k)
			assert.NoError(err)
			assert.Expect(status, foremanapi.StatusCompleted)
		}

		// Strict priority ascending, then creation order within a priority.
		order := svc.Order()
		if assert.True(len(order) >= 1) {
			assert.Expect(order[0], "_holder")
		}
		assert.Expect(order[1:], []string{"p2a", "p2b", "p3a", "p5a", "p5b", "p9a"})
	})

	t.Run("starvation_by_design", func(t *testing.T) {
		assert := testarossa.For(t)
		svc.mu.Lock()
		svc.order = nil
		svc.mu.Unlock()

		holder := startFlow(assert, "_holder", 1, 1500)
		// A single low-priority flow plus a batch of higher-priority flows, all
		// pending before the worker frees. The low flow is starved by design
		// until the entire higher-priority batch is exhausted - no aging.
		low := startFlow(assert, "low", 9, 50)
		var highs []string
		for i := 0; i < 8; i++ {
			highs = append(highs, startFlow(assert, "h"+string(rune('0'+i)), 2, 50))
		}

		time.Sleep(300 * time.Millisecond)

		status, _, err := fm.Await(ctx, holder)
		assert.NoError(err)
		assert.Expect(status, foremanapi.StatusCompleted)
		for _, k := range append(highs, low) {
			status, _, err := fm.Await(ctx, k)
			assert.NoError(err)
			assert.Expect(status, foremanapi.StatusCompleted)
		}

		order := svc.Order()
		assert.Expect(order, []string{"_holder", "h0", "h1", "h2", "h3", "h4", "h5", "h6", "h7", "low"})
	})
}
