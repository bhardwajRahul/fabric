package configurator

import (
	"context"
	"testing"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestConfigurator_Mock(t *testing.T) {
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

	t.Run("values", func(t *testing.T) { // MARKER: Values
		assert := testarossa.For(t)

		mock.MockValues(func(ctx context.Context, names []string) (values map[string]string, err error) {
			return
		})
		var names []string
		_, err := mock.Values(ctx, names)
		assert.NoError(err)
	})

	t.Run("refresh", func(t *testing.T) { // MARKER: Refresh
		assert := testarossa.For(t)

		mock.MockRefresh(func(ctx context.Context) (err error) {
			return
		})
		err := mock.Refresh(ctx)
		assert.NoError(err)
	})

	t.Run("sync_repo", func(t *testing.T) { // MARKER: SyncRepo
		assert := testarossa.For(t)

		mock.MockSyncRepo(func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) {
			return
		})
		var timestamp time.Time
		var values map[string]map[string]string
		err := mock.SyncRepo(ctx, timestamp, values)
		assert.NoError(err)
	})

	t.Run("values443", func(t *testing.T) { // MARKER: Values443
		assert := testarossa.For(t)

		mock.MockValues443(func(ctx context.Context, names []string) (values map[string]string, err error) {
			return
		})
		var names []string
		_, err := mock.Values443(ctx, names)
		assert.NoError(err)
	})

	t.Run("refresh443", func(t *testing.T) { // MARKER: Refresh443
		assert := testarossa.For(t)

		mock.MockRefresh443(func(ctx context.Context) (err error) {
			return
		})
		err := mock.Refresh443(ctx)
		assert.NoError(err)
	})

	t.Run("sync443", func(t *testing.T) { // MARKER: Sync443
		assert := testarossa.For(t)

		mock.MockSync443(func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) {
			return
		})
		var timestamp time.Time
		var values map[string]map[string]string
		err := mock.Sync443(ctx, timestamp, values)
		assert.NoError(err)
	})

	t.Run("periodic_refresh", func(t *testing.T) { // MARKER: PeriodicRefresh
		assert := testarossa.For(t)

		mock.MockPeriodicRefresh(func(ctx context.Context) (err error) {
			return
		})
		err := mock.PeriodicRefresh(ctx)
		assert.NoError(err)
	})

}
