package eventsink

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestEventsink_Mock(t *testing.T) {
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

	t.Run("registered", func(t *testing.T) { // MARKER: Registered
		assert := testarossa.For(t)

		mock.MockRegistered(func(ctx context.Context) (emails []string, err error) {
			return
		})
		_, err := mock.Registered(ctx)
		assert.NoError(err)
	})

	t.Run("on_allow_register", func(t *testing.T) { // MARKER: OnAllowRegister
		assert := testarossa.For(t)

		mock.MockOnAllowRegister(func(ctx context.Context, email string) (allow bool, err error) {
			return
		})
		var email string
		_, err := mock.OnAllowRegister(ctx, email)
		assert.NoError(err)
	})

	t.Run("on_registered", func(t *testing.T) { // MARKER: OnRegistered
		assert := testarossa.For(t)

		mock.MockOnRegistered(func(ctx context.Context, email string) (err error) {
			return
		})
		var email string
		err := mock.OnRegistered(ctx, email)
		assert.NoError(err)
	})

}
