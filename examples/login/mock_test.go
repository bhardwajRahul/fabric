package login

import (
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestLogin_Mock(t *testing.T) {
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

	t.Run("login", func(t *testing.T) { // MARKER: Login
		assert := testarossa.For(t)

		mock.MockLogin(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Login(w, r)
		assert.NoError(err)
	})

	t.Run("logout", func(t *testing.T) { // MARKER: Logout
		assert := testarossa.For(t)

		mock.MockLogout(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Logout(w, r)
		assert.NoError(err)
	})

	t.Run("welcome", func(t *testing.T) { // MARKER: Welcome
		assert := testarossa.For(t)

		mock.MockWelcome(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Welcome(w, r)
		assert.NoError(err)
	})

	t.Run("admin_only", func(t *testing.T) { // MARKER: AdminOnly
		assert := testarossa.For(t)

		mock.MockAdminOnly(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.AdminOnly(w, r)
		assert.NoError(err)
	})

	t.Run("manager_only", func(t *testing.T) { // MARKER: ManagerOnly
		assert := testarossa.For(t)

		mock.MockManagerOnly(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.ManagerOnly(w, r)
		assert.NoError(err)
	})

}
