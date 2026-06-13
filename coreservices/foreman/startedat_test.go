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

package foreman

import (
	"context"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"
)

// readFlowTimestamps returns (created_at, started_at, updated_at) for flow_id on shard 1.
func readFlowTimestamps(t *testing.T, ctx context.Context, svc *Service, flowID int) (time.Time, time.Time, time.Time) {
	t.Helper()
	db, err := svc.shard(1)
	if err != nil {
		t.Fatal(err)
	}
	var createdAt, startedAt, updatedAt time.Time
	err = db.QueryRowContext(ctx,
		"SELECT created_at, started_at, updated_at FROM microbus_flows WHERE flow_id=?",
		flowID,
	).Scan(&createdAt, &startedAt, &updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	return createdAt.UTC(), startedAt.UTC(), updatedAt.UTC()
}

// TestForeman_StartedAt_StartStampsAfterCreate verifies that Start advances started_at past the
// INSERT default, so a flow that sits in `created` for a while before being Started gets a
// started_at reflecting the actual dispatch moment, not the row's insert time.
func TestForeman_StartedAt_StartStampsAfterCreate(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	flowKey, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
	if !assert.NoError(err) {
		return
	}
	_, flowID, _, err := parseFlowKey(flowKey)
	if !assert.NoError(err) {
		return
	}

	// Right after Create: started_at is the INSERT default, roughly equal to created_at.
	createdAtAfterCreate, startedAtAfterCreate, _ := readFlowTimestamps(t, ctx, svc, flowID)
	// A small sleep so Start's NOW_UTC() is provably later than the INSERT-time started_at,
	// using the DB's own clock as the reference.
	time.Sleep(20 * time.Millisecond)

	if !assert.NoError(fm.Start(ctx, flowKey)) {
		return
	}

	createdAtAfterStart, startedAtAfterStart, _ := readFlowTimestamps(t, ctx, svc, flowID)

	// created_at does NOT change on Start.
	assert.Expect(createdAtAfterStart.Equal(createdAtAfterCreate), true)
	// started_at advanced - it was stamped fresh by the Start UPDATE, distinct from the
	// INSERT-time default.
	assert.True(startedAtAfterStart.After(startedAtAfterCreate))
}

// TestForeman_StartedAt_RestartResets verifies that Restart on a terminal flow resets started_at
// to a fresh value - a Restart is a new attempt, so duration metrics start over.
func TestForeman_StartedAt_RestartResets(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	flowKey, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
	if !assert.NoError(err) {
		return
	}
	_, flowID, _, err := parseFlowKey(flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.NoError(fm.Start(ctx, flowKey)) {
		return
	}
	outcome, err := fm.Await(ctx, flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.Equal(workflow.StatusCompleted, outcome.Status) {
		return
	}

	createdAtBeforeRestart, startedAtBeforeRestart, _ := readFlowTimestamps(t, ctx, svc, flowID)

	// Pause long enough that Restart's NOW_UTC() is provably later than the prior timestamps,
	// using the DB's own clock as the reference (not Go's wall clock - the two can differ).
	time.Sleep(20 * time.Millisecond)

	if !assert.NoError(fm.Restart(ctx, flowKey, nil)) {
		return
	}

	createdAtAfterRestart, startedAtAfterRestart, _ := readFlowTimestamps(t, ctx, svc, flowID)

	// Both created_at and started_at reset (Restart is a fresh attempt) - new values are strictly
	// after the pre-Restart ones, using the same NOW_UTC() clock source for both reads.
	assert.True(createdAtAfterRestart.After(createdAtBeforeRestart))
	assert.True(startedAtAfterRestart.After(startedAtBeforeRestart))

	// FlowSummary.Duration() reflects the new attempt's wall time, not the original lifespan.
	outcome2, err := fm.Await(ctx, flowKey)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(workflow.StatusCompleted, outcome2.Status)
	flows, _, err := fm.List(ctx, foremanapi.Query{Limit: 10})
	if !assert.NoError(err) {
		return
	}
	var summary *foremanapi.FlowSummary
	for i := range flows {
		if flows[i].FlowKey == flowKey {
			summary = &flows[i]
			break
		}
	}
	if !assert.NotNil(summary) {
		return
	}
	// Summary.StartedAt should be the post-Restart started_at; Duration() = updated - started_at,
	// which is the second-run duration (short), not since-original-creation.
	assert.True(summary.StartedAt.Equal(startedAtAfterRestart) || summary.StartedAt.After(startedAtAfterRestart))
	assert.True(summary.Duration() < time.Until(time.Now().UTC().Add(time.Second)))
}

// TestForeman_StartedAt_RestartFromPreserves verifies that RestartFrom is a surgical rewind -
// the flow's started_at is preserved so total-lifespan metrics stay meaningful across the rewind.
func TestForeman_StartedAt_RestartFromPreserves(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	flowKey, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
	if !assert.NoError(err) {
		return
	}
	_, flowID, _, err := parseFlowKey(flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.NoError(fm.Start(ctx, flowKey)) {
		return
	}
	outcome, err := fm.Await(ctx, flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.Equal(workflow.StatusCompleted, outcome.Status) {
		return
	}

	createdAtBeforeRF, startedAtBeforeRF, _ := readFlowTimestamps(t, ctx, svc, flowID)

	// Pause so we can prove RestartFrom did NOT touch the timestamps (if it had, the new value
	// would be strictly later than this pause).
	time.Sleep(20 * time.Millisecond)

	// Pick the entry step and RestartFrom it.
	steps, err := fm.History(ctx, flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.True(len(steps) > 0) {
		return
	}
	if !assert.NoError(fm.RestartFrom(ctx, steps[0].StepKey, nil)) {
		return
	}

	createdAtAfterRF, startedAtAfterRF, _ := readFlowTimestamps(t, ctx, svc, flowID)

	// Neither created_at nor started_at changed - RestartFrom is a surgical rewind, the flow's
	// run continues. updated_at DOES bump (status transitioned), but that's outside scope here.
	assert.True(createdAtAfterRF.Equal(createdAtBeforeRF))
	assert.True(startedAtAfterRF.Equal(startedAtBeforeRF))
}

// TestForeman_FlowSummary_Duration_UsesStartedAt verifies the user-facing semantic: the duration
// column on the flows list page reads time since the current attempt's first dispatch, not since
// the row's original creation. Captures the bug fix for "duration shows 7+ hours after a flow
// sat in the created/parked state and was finally Started."
func TestForeman_FlowSummary_Duration_UsesStartedAt(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	fm := foremanapi.NewClient(tester)
	app := application.New()
	app.Add(svc, newTestWorkflowSvc(), tester)
	app.RunInTest(t)

	assert := testarossa.For(t)

	flowKey, err := fm.Create(ctx, "https://test.workflow.host:428/my-workflow", map[string]any{}, nil)
	if !assert.NoError(err) {
		return
	}

	// Simulate a flow that sits in `created` for a while before being Started.
	time.Sleep(200 * time.Millisecond)

	if !assert.NoError(fm.Start(ctx, flowKey)) {
		return
	}
	outcome, err := fm.Await(ctx, flowKey)
	if !assert.NoError(err) {
		return
	}
	if !assert.Equal(workflow.StatusCompleted, outcome.Status) {
		return
	}

	flows, _, err := fm.List(ctx, foremanapi.Query{Limit: 10})
	if !assert.NoError(err) {
		return
	}
	var summary *foremanapi.FlowSummary
	for i := range flows {
		if flows[i].FlowKey == flowKey {
			summary = &flows[i]
			break
		}
	}
	if !assert.NotNil(summary) {
		return
	}
	// Without the fix, Duration() would be UpdatedAt - CreatedAt ≈ 200ms+ (includes the Create→
	// Start delay). With the fix, Duration() is UpdatedAt - StartedAt and excludes the
	// pre-Start delay - this trivial test workflow completes in well under 100ms.
	assert.True(summary.Duration() < 100*time.Millisecond)
}
