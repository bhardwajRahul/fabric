package docextractionflow

import (
	"context"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/docextractionflow/docextractionflowapi"
)

func TestDocextractionflow_Mock(t *testing.T) {
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

	t.Run("scan_p_d_f", func(t *testing.T) { // MARKER: ScanPDF
		assert := testarossa.For(t)

		mock.MockScanPDF(func(ctx context.Context, flow *workflow.Flow, pdf []byte) (pageImages [][]byte, pageCount int, err error) {
			return
		})
		var pdf []byte
		_, _, err := mock.ScanPDF(ctx, nil, pdf)
		assert.NoError(err)
	})

	t.Run("identify_chunks", func(t *testing.T) { // MARKER: IdentifyChunks
		assert := testarossa.For(t)

		mock.MockIdentifyChunks(func(ctx context.Context, flow *workflow.Flow, page []byte) (chunks []docextractionflowapi.Rectangle, err error) {
			return
		})
		var page []byte
		_, err := mock.IdentifyChunks(ctx, nil, page)
		assert.NoError(err)
	})

	t.Run("transcribe_chunk", func(t *testing.T) { // MARKER: TranscribeChunk
		assert := testarossa.For(t)

		mock.MockTranscribeChunk(func(ctx context.Context, flow *workflow.Flow, page []byte, chunk docextractionflowapi.Rectangle) (listTranscriptions []string, err error) {
			return
		})
		var page []byte
		var chunk docextractionflowapi.Rectangle
		_, err := mock.TranscribeChunk(ctx, nil, page, chunk)
		assert.NoError(err)
	})

	t.Run("join_page_transcriptions", func(t *testing.T) { // MARKER: JoinPageTranscriptions
		assert := testarossa.For(t)

		mock.MockJoinPageTranscriptions(func(ctx context.Context, flow *workflow.Flow, listTranscriptions []string) (listPageTexts []string, err error) {
			return
		})
		var listTranscriptions []string
		_, err := mock.JoinPageTranscriptions(ctx, nil, listTranscriptions)
		assert.NoError(err)
	})

	t.Run("join_doc_transcriptions", func(t *testing.T) { // MARKER: JoinDocTranscriptions
		assert := testarossa.For(t)

		mock.MockJoinDocTranscriptions(func(ctx context.Context, flow *workflow.Flow, listPageTexts []string) (docTranscription string, err error) {
			return
		})
		var listPageTexts []string
		_, err := mock.JoinDocTranscriptions(ctx, nil, listPageTexts)
		assert.NoError(err)
	})

	t.Run("doc_extraction", func(t *testing.T) { // MARKER: DocExtraction
		assert := testarossa.For(t)

		mock.MockDocExtraction(func(ctx context.Context, flow *workflow.Flow, pdf []byte) (docTranscription string, pageCount int, err error) {
			return
		})
		graph, err := mock.DocExtraction(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}
	})

}
