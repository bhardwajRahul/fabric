package embedder

import (
	"context"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestEmbedder_Mock(t *testing.T) {
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

	t.Run("embed", func(t *testing.T) { // MARKER: Embed
		assert := testarossa.For(t)

		mock.MockEmbed(func(ctx context.Context, text string) (vector []float64, err error) {
			return
		})
		var text string
		_, err := mock.Embed(ctx, text)
		assert.NoError(err)
	})

	t.Run("similarity", func(t *testing.T) { // MARKER: Similarity
		assert := testarossa.For(t)

		mock.MockSimilarity(func(ctx context.Context, a string, b string) (score float64, err error) {
			return
		})
		var a string
		var b string
		_, err := mock.Similarity(ctx, a, b)
		assert.NoError(err)
	})

	t.Run("demo", func(t *testing.T) { // MARKER: Demo
		assert := testarossa.For(t)

		mock.MockDemo(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.Demo(w, r)
		assert.NoError(err)
	})

	t.Run("demo_init", func(t *testing.T) { // MARKER: DemoInit
		assert := testarossa.For(t)

		mock.MockDemoInit(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.DemoInit(w, r)
		assert.NoError(err)
	})

	t.Run("demo_status", func(t *testing.T) { // MARKER: DemoStatus
		assert := testarossa.For(t)

		mock.MockDemoStatus(func(w http.ResponseWriter, r *http.Request) (err error) {
			return nil
		})
		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)
		err := mock.DemoStatus(w, r)
		assert.NoError(err)
	})

}
