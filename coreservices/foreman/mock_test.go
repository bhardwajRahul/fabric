package foreman

import (
	"context"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

func TestForeman_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("create", func(t *testing.T) { // MARKER: Create
		assert := testarossa.For(t)

		mock.MockCreate(func(ctx context.Context, workflowName string, initialState any) (flowKey string, err error) {
			return
		})
		var workflowName string
		var initialState any
		_, err := mock.Create(ctx, workflowName, initialState)
		assert.NoError(err)
	})

	t.Run("start", func(t *testing.T) { // MARKER: Start
		assert := testarossa.For(t)

		mock.MockStart(func(ctx context.Context, flowKey string) (err error) {
			return
		})
		var flowKey string
		err := mock.Start(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("start_notify", func(t *testing.T) { // MARKER: StartNotify
		assert := testarossa.For(t)

		mock.MockStartNotify(func(ctx context.Context, flowKey string, notifyHostname string) (err error) {
			return
		})
		var flowKey string
		var notifyHostname string
		err := mock.StartNotify(ctx, flowKey, notifyHostname)
		assert.NoError(err)
	})

	t.Run("snapshot", func(t *testing.T) { // MARKER: Snapshot
		assert := testarossa.For(t)

		mock.MockSnapshot(func(ctx context.Context, flowKey string) (status string, state map[string]any, err error) {
			return
		})
		var flowKey string
		_, _, err := mock.Snapshot(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("resume", func(t *testing.T) { // MARKER: Resume
		assert := testarossa.For(t)

		mock.MockResume(func(ctx context.Context, flowKey string, resumeData any) (err error) {
			return
		})
		var flowKey string
		var resumeData any
		err := mock.Resume(ctx, flowKey, resumeData)
		assert.NoError(err)
	})

	t.Run("fork", func(t *testing.T) { // MARKER: Fork
		assert := testarossa.For(t)

		mock.MockFork(func(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error) {
			return
		})
		var stepKey string
		var stateOverrides any
		_, err := mock.Fork(ctx, stepKey, stateOverrides)
		assert.NoError(err)
	})

	t.Run("cancel", func(t *testing.T) { // MARKER: Cancel
		assert := testarossa.For(t)

		mock.MockCancel(func(ctx context.Context, flowKey string) (err error) {
			return
		})
		var flowKey string
		err := mock.Cancel(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("history", func(t *testing.T) { // MARKER: History
		assert := testarossa.For(t)

		mock.MockHistory(func(ctx context.Context, flowKey string) (steps []foremanapi.FlowStep, err error) {
			return
		})
		var flowKey string
		_, err := mock.History(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("retry", func(t *testing.T) { // MARKER: Retry
		assert := testarossa.For(t)

		mock.MockRetry(func(ctx context.Context, flowKey string) (err error) {
			return
		})
		var flowKey string
		err := mock.Retry(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("list", func(t *testing.T) { // MARKER: List
		assert := testarossa.For(t)

		mock.MockList(func(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error) {
			return
		})
		var query foremanapi.Query
		_, err := mock.List(ctx, query)
		assert.NoError(err)
	})

	t.Run("create_task", func(t *testing.T) { // MARKER: CreateTask
		assert := testarossa.For(t)

		mock.MockCreateTask(func(ctx context.Context, taskName string, initialState any) (flowKey string, err error) {
			return
		})
		var taskName string
		var initialState any
		_, err := mock.CreateTask(ctx, taskName, initialState)
		assert.NoError(err)
	})

	t.Run("enqueue", func(t *testing.T) { // MARKER: Enqueue
		assert := testarossa.For(t)

		mock.MockEnqueue(func(ctx context.Context, shard int, stepID int) (err error) {
			return
		})
		var shard int
		var stepID int
		err := mock.Enqueue(ctx, shard, stepID)
		assert.NoError(err)
	})

	t.Run("await", func(t *testing.T) { // MARKER: Await
		assert := testarossa.For(t)

		mock.MockAwait(func(ctx context.Context, flowKey string) (status string, state map[string]any, err error) {
			return
		})
		var flowKey string
		_, _, err := mock.Await(ctx, flowKey)
		assert.NoError(err)
	})

	t.Run("notify_status_change", func(t *testing.T) { // MARKER: NotifyStatusChange
		assert := testarossa.For(t)

		mock.MockNotifyStatusChange(func(ctx context.Context, flowKey string, status string) (err error) {
			return
		})
		var flowKey string
		var status string
		err := mock.NotifyStatusChange(ctx, flowKey, status)
		assert.NoError(err)
	})

	t.Run("break_before", func(t *testing.T) { // MARKER: BreakBefore
		assert := testarossa.For(t)

		mock.MockBreakBefore(func(ctx context.Context, flowKey string, taskName string, enabled bool) (err error) {
			return
		})
		var flowKey string
		var taskName string
		var enabled bool
		err := mock.BreakBefore(ctx, flowKey, taskName, enabled)
		assert.NoError(err)
	})

	t.Run("run", func(t *testing.T) { // MARKER: Run
		assert := testarossa.For(t)

		mock.MockRun(func(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) {
			return
		})
		var workflowName string
		var initialState any
		_, _, err := mock.Run(ctx, workflowName, initialState)
		assert.NoError(err)
	})

	t.Run("continue", func(t *testing.T) { // MARKER: Continue
		assert := testarossa.For(t)

		mock.MockContinue(func(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error) {
			return
		})
		var threadKey string
		var additionalState any
		_, err := mock.Continue(ctx, threadKey, additionalState)
		assert.NoError(err)
	})

	t.Run("history_mermaid", func(t *testing.T) { // MARKER: HistoryMermaid
		assert := testarossa.For(t)

		mock.MockHistoryMermaid(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.HistoryMermaid(w, r)
		assert.NoError(err)
	})

	t.Run("purge_expired_flows", func(t *testing.T) { // MARKER: PurgeExpiredFlows
		assert := testarossa.For(t)

		mock.MockPurgeExpiredFlows(func(ctx context.Context) (err error) {
			return
		})
		err := mock.PurgeExpiredFlows(ctx)
		assert.NoError(err)
	})

	t.Run("on_observe_queue_depth", func(t *testing.T) { // MARKER: QueueDepth
		assert := testarossa.For(t)

		mock.MockOnObserveQueueDepth(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnObserveQueueDepth(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_num_shards", func(t *testing.T) { // MARKER: NumShards
		assert := testarossa.For(t)

		mock.MockOnChangedNumShards(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedNumShards(ctx)
		assert.NoError(err)
	})

}
