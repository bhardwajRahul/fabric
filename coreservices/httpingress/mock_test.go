package httpingress

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestHttpingress_Mock(t *testing.T) {
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

	t.Run("on_changed_ports", func(t *testing.T) { // MARKER: Ports
		assert := testarossa.For(t)

		mock.MockOnChangedPorts(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedPorts(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_allowed_origins", func(t *testing.T) { // MARKER: AllowedOrigins
		assert := testarossa.For(t)

		mock.MockOnChangedAllowedOrigins(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedAllowedOrigins(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_port_mappings", func(t *testing.T) { // MARKER: PortMappings
		assert := testarossa.For(t)

		mock.MockOnChangedPortMappings(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedPortMappings(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_read_timeout", func(t *testing.T) { // MARKER: ReadTimeout
		assert := testarossa.For(t)

		mock.MockOnChangedReadTimeout(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedReadTimeout(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_write_timeout", func(t *testing.T) { // MARKER: WriteTimeout
		assert := testarossa.For(t)

		mock.MockOnChangedWriteTimeout(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedWriteTimeout(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_read_header_timeout", func(t *testing.T) { // MARKER: ReadHeaderTimeout
		assert := testarossa.For(t)

		mock.MockOnChangedReadHeaderTimeout(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedReadHeaderTimeout(ctx)
		assert.NoError(err)
	})

	t.Run("on_changed_blocked_paths", func(t *testing.T) { // MARKER: BlockedPaths
		assert := testarossa.For(t)

		mock.MockOnChangedBlockedPaths(func(ctx context.Context) (err error) {
			return
		})
		err := mock.OnChangedBlockedPaths(ctx)
		assert.NoError(err)
	})

}
