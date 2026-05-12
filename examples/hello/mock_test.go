package hello

import (
	"context"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestHello_Mock(t *testing.T) {
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

	t.Run("hello", func(t *testing.T) { // MARKER: Hello
		assert := testarossa.For(t)

		mock.MockHello(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Hello(w, r)
		assert.NoError(err)
	})

	t.Run("echo", func(t *testing.T) { // MARKER: Echo
		assert := testarossa.For(t)

		mock.MockEcho(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Echo(w, r)
		assert.NoError(err)
	})

	t.Run("ping", func(t *testing.T) { // MARKER: Ping
		assert := testarossa.For(t)

		mock.MockPing(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Ping(w, r)
		assert.NoError(err)
	})

	t.Run("calculator", func(t *testing.T) { // MARKER: Calculator
		assert := testarossa.For(t)

		mock.MockCalculator(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Calculator(w, r)
		assert.NoError(err)
	})

	t.Run("bus_p_n_g", func(t *testing.T) { // MARKER: BusPNG
		assert := testarossa.For(t)

		mock.MockBusPNG(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.BusPNG(w, r)
		assert.NoError(err)
	})

	t.Run("localization", func(t *testing.T) { // MARKER: Localization
		assert := testarossa.For(t)

		mock.MockLocalization(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Localization(w, r)
		assert.NoError(err)
	})

	t.Run("root", func(t *testing.T) { // MARKER: Root
		assert := testarossa.For(t)

		mock.MockRoot(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Root(w, r)
		assert.NoError(err)
	})

	t.Run("tick_tock", func(t *testing.T) { // MARKER: TickTock
		assert := testarossa.For(t)

		mock.MockTickTock(func(ctx context.Context) (err error) {
			return
		})
		err := mock.TickTock(ctx)
		assert.NoError(err)
	})

}
