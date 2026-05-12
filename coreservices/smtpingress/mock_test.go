package smtpingress

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestSmtpingress_Mock(t *testing.T) {
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

	t.Run("on_changed_port", func(t *testing.T) { // MARKER: Port
		assert := testarossa.For(t)

		mock.MockOnChangedPort(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedPort(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_enabled", func(t *testing.T) { // MARKER: Enabled
		assert := testarossa.For(t)

		mock.MockOnChangedEnabled(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedEnabled(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_max_size", func(t *testing.T) { // MARKER: MaxSize
		assert := testarossa.For(t)

		mock.MockOnChangedMaxSize(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedMaxSize(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_max_clients", func(t *testing.T) { // MARKER: MaxClients
		assert := testarossa.For(t)

		mock.MockOnChangedMaxClients(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedMaxClients(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_workers", func(t *testing.T) { // MARKER: Workers
		assert := testarossa.For(t)

		mock.MockOnChangedWorkers(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedWorkers(ctx)
		assert.NoError(err)
	})

}
