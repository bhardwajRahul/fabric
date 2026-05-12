package eventsource

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestEventsource_Mock(t *testing.T) {
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

	t.Run("register", func(t *testing.T) { // MARKER: Register
		assert := testarossa.For(t)

		mock.MockRegister(func(ctx context.Context, email string) (allowed bool, err error) {
			return
		})
		var email string
		_, err := mock.Register(ctx, email)
		assert.NoError(err)
	})

}
