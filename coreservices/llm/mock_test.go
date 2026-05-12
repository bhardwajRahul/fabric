package llm

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

func TestLlm_Mock(t *testing.T) {
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

	t.Run("chat", func(t *testing.T) { // MARKER: Chat
		assert := testarossa.For(t)

		mock.MockChat(func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) {
			return
		})
		var provider string
		var model string
		var messages []llmapi.Message
		var toolURLs []string
		var options *llmapi.ChatOptions
		_, _, err := mock.Chat(ctx, provider, model, messages, toolURLs, options)
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

	t.Run("init_chat", func(t *testing.T) { // MARKER: InitChat
		assert := testarossa.For(t)

		mock.MockInitChat(func(ctx context.Context, flow *workflow.Flow, listMessages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error) {
			return
		})
		var listMessages []llmapi.Message
		var tools []llmapi.Tool
		var options *llmapi.ChatOptions
		_, _, err := mock.InitChat(ctx, nil, listMessages, tools, options)
		assert.NoError(err)
	})

	t.Run("call_l_l_m", func(t *testing.T) { // MARKER: CallLLM
		assert := testarossa.For(t)

		mock.MockCallLLM(func(ctx context.Context, flow *workflow.Flow, provider string, model string, listMessages []llmapi.Message) (llmContent string, pendingToolCalls any, turnUsage llmapi.Usage, err error) {
			return
		})
		var provider string
		var model string
		var listMessages []llmapi.Message
		_, _, _, err := mock.CallLLM(ctx, nil, provider, model, listMessages)
		assert.NoError(err)
	})

	t.Run("process_response", func(t *testing.T) { // MARKER: ProcessResponse
		assert := testarossa.For(t)

		mock.MockProcessResponse(func(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (listMessagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) {
			return
		})
		var llmContent string
		var turnUsage llmapi.Usage
		var toolRounds int
		var maxToolRounds int
		_, _, _, _, err := mock.ProcessResponse(ctx, nil, llmContent, turnUsage, toolRounds, maxToolRounds)
		assert.NoError(err)
	})

	t.Run("execute_tool", func(t *testing.T) { // MARKER: ExecuteTool
		assert := testarossa.For(t)

		mock.MockExecuteTool(func(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error) {
			return
		})
		var toolExecuted bool
		_, err := mock.ExecuteTool(ctx, nil, toolExecuted)
		assert.NoError(err)
	})

	t.Run("chat_loop", func(t *testing.T) { // MARKER: ChatLoop
		assert := testarossa.For(t)

		mock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, listMessages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (listMessagesOut []llmapi.Message, usage llmapi.Usage, err error) {
			return
		})
		graph, err := mock.ChatLoop(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
