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

package nestedfailfanoutflow

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

	"github.com/microbus-io/fabric/verify/nestedfailfanoutflow/nestedfailfanoutflowapi"
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
	_ nestedfailfanoutflowapi.Client
)

// TestNestedfailfanoutflow_Nested verifies that a single-cell failure in a 3x3 nested forEach
// does NOT preempt the other 8 cells, and that the flow transitions to failed only after every
// cell has terminated.
func TestNestedfailfanoutflow_Nested(t *testing.T) { // MARKER: Nested
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	// Start the workflow asynchronously so we can observe its mid-execution state.
	flowKey, err := foremanClient.Create(ctx, nestedfailfanoutflowapi.Nested.URL(), nil, nil)
	if !testarossa.For(t).NoError(err) {
		return
	}
	err = foremanClient.Start(ctx, flowKey)
	if !testarossa.For(t).NoError(err) {
		return
	}

	// Wait until every inner cell has started: one will fail synchronously, the other 8 block
	// on the test gate. At this point the failing branch's failStep has marked its step failed
	// and bumped cohort counters, but the cohort hasn't fully resolved yet.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		innerStarts, _, _, _ := svc.Counters()
		if innerStarts >= 9 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Run("flow_running_while_siblings_still_in_flight", func(t *testing.T) {
		assert := testarossa.For(t)
		innerStarts, innerCompleted, _, _ := svc.Counters()
		assert.Expect(
			innerStarts, 9, // all 9 cells started
			innerCompleted, 0, // none of the gated 8 has completed yet
		)
		outcome, err := foremanClient.Snapshot(ctx, flowKey)
		if assert.NoError(err) && assert.NotNil(outcome) {
			assert.Equal(workflow.StatusRunning, outcome.Status)
		}
	})

	// Release the gated inner cells. They'll all complete, their cohorts will resolve, the
	// outer cohort will see its failed branch via propagation, and the flow will fail.
	svc.ReleaseSlowInners()

	outcome, err := foremanClient.Await(ctx, flowKey)

	t.Run("flow_failed_after_full_resolution", func(t *testing.T) {
		assert := testarossa.For(t)
		assert.NoError(err)
		if !assert.NotNil(outcome) {
			return
		}
		assert.Equal(workflow.StatusFailed, outcome.Status)
	})

	t.Run("eight_inners_completed_and_two_joinI_fired", func(t *testing.T) {
		assert := testarossa.For(t)
		innerStarts, innerCompleted, joinI, joinO := svc.Counters()
		assert.Expect(
			innerStarts, 9,    // every inner cell ran (one failed, eight completed)
			innerCompleted, 8, // the 8 non-failing cells reached the end of TaskI
			joinI, 2,          // joinI fired for outer=0 and outer=2; outer=1's cohort had a failure and was never aggregated
			joinO, 0,          // joinO never ran because the outer cohort had a failed branch
		)
	})

	// Restart-from-the-failed-step path. Locate the one failed cell, restart it with overrides
	// that flip `currentOuter` so the failure condition (currentOuter == 1 && innerItem == 1) no
	// longer fires. The cohort counter undo walks up from outer=1's inner spawn to the root, so
	// the outer cohort no longer carries the propagated failure; once the rerun succeeds, joinI
	// fires for outer=1, joinO fires for the outer cohort, and the flow ends in completed.
	steps, herr := foremanClient.History(ctx, flowKey)
	if !testarossa.For(t).NoError(herr) {
		return
	}
	var failedStepKey string
	for _, s := range steps {
		if s.Status == workflow.StatusFailed {
			failedStepKey = s.StepKey
			break
		}
	}
	if !testarossa.For(t).NotEqual("", failedStepKey) {
		return
	}

	err = foremanClient.RestartFrom(ctx, failedStepKey, map[string]any{"currentOuter": 2})
	if !testarossa.For(t).NoError(err) {
		return
	}
	restartOutcome, err := foremanClient.Await(ctx, flowKey)

	t.Run("restart_flips_to_completed", func(t *testing.T) {
		assert := testarossa.For(t)
		assert.NoError(err)
		if !assert.NotNil(restartOutcome) {
			return
		}
		assert.Equal(workflow.StatusCompleted, restartOutcome.Status)
	})

	t.Run("only_failed_cell_re_executed", func(t *testing.T) {
		assert := testarossa.For(t)
		innerStarts, innerCompleted, joinI, joinO := svc.Counters()
		assert.Expect(
			innerStarts, 10,    // 9 from first pass + 1 from the restart
			innerCompleted, 9,  // 8 from first pass + 1 from the restart (the restarted cell succeeds with currentOuter=2)
			joinI, 3,           // outer=0 and outer=2 from first pass; outer=1 now fires after the restart
			joinO, 1,           // outer cohort had no failed branches after restart-undo, so joinO finally fires
		)
	})
}
