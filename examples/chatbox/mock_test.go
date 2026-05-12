package chatbox

import (
	"context"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/testarossa"
)

func TestChatbox_Mock(t *testing.T) {
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

	t.Run("turn", func(t *testing.T) { // MARKER: Turn
		assert := testarossa.For(t)

		mock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
			return
		})
		var model string
		var messages []llmapi.Message
		var tools []llmapi.Tool
		var options *llmapi.TurnOptions
		_, _, _, err := mock.Turn(ctx, model, messages, tools, options)
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

}
