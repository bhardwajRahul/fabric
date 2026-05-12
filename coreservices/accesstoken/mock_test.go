package accesstoken

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
)

func TestAccesstoken_Mock(t *testing.T) {
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

	t.Run("rotate_key", func(t *testing.T) { // MARKER: RotateKey
		assert := testarossa.For(t)

		mock.MockRotateKey(func(ctx context.Context) (err error) {
			return
		})
		err := mock.RotateKey(ctx)
		assert.NoError(err)
	})

	t.Run("mint", func(t *testing.T) { // MARKER: Mint
		assert := testarossa.For(t)

		mock.MockMint(func(ctx context.Context, claims any) (token string, err error) {
			return
		})
		var claims any
		_, err := mock.Mint(ctx, claims)
		assert.NoError(err)
	})

	t.Run("j_w_k_s", func(t *testing.T) { // MARKER: JWKS
		assert := testarossa.For(t)

		mock.MockJWKS(func(ctx context.Context) (keys []accesstokenapi.JWK, err error) {
			return
		})
		_, err := mock.JWKS(ctx)
		assert.NoError(err)
	})

	t.Run("local_keys", func(t *testing.T) { // MARKER: LocalKeys
		assert := testarossa.For(t)

		mock.MockLocalKeys(func(ctx context.Context) (keys []accesstokenapi.JWK, err error) {
			return
		})
		_, err := mock.LocalKeys(ctx)
		assert.NoError(err)
	})

}
