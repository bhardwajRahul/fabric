package docextractionflow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"

	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/verify/docextractionflow/docextractionflowapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ httpx.BodyReader
	_ *workflow.Flow
	_ testarossa.Asserter
	_ docextractionflowapi.Client
)

func TestDocextractionflow_DocExtraction(t *testing.T) { // MARKER: DocExtraction
	// Not parallel: this fixture is intentionally heavy (16 foreman workers driving a
	// doubly-nested forEach with 1s + 2-5s task sleeps). Running it alongside other
	// parallel tests starves CPU and pushes per-task dispatches past their time budget.
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	// The synchronous Run blocks on a single request whose context bounds Await.
	// This workflow legitimately runs long (2-5s per chunk across many pages, plus
	// the 5% retry's exponential backoff on the critical path), so the default
	// request timeout is too short - give it a generous one.
	foremanClient := foremanapi.NewClient(tester).WithOptions(pub.Timeout(5 * time.Minute))
	exec := docextractionflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	app := application.New()
	app.Add(
		svc,
		// 4 workers. The simulated OCR latency is small (50-150ms/chunk), so wall time
		// stays well within the test budget at this worker count even on a contended
		// runner; the count is not load-bearing for correctness.
		foreman.NewService().Init(func(f *foreman.Service) error { return f.SetWorkers(4) }),
		tester,
	)
	app.RunInTest(t)

	t.Run("extracts_every_page", func(t *testing.T) {
		assert := testarossa.For(t)

		// A sizable mock PDF input (its bytes are not interpreted; ScanPDF generates
		// random page images regardless).
		pdf := make([]byte, 512*1024)
		for i := range pdf {
			pdf[i] = byte(i)
		}

		docTranscription, pageCount, status, err := exec.DocExtraction(ctx, pdf)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
		)
		// 5-22 pages, every one transcribed; nested fan-in preserves one line per page.
		assert.Expect(pageCount >= 5 && pageCount <= 22, true)
		assert.Expect(docTranscription != "", true)
		lines := strings.Split(docTranscription, "\n")
		assert.Expect(len(lines), pageCount)
		for _, ln := range lines {
			assert.Expect(strings.TrimSpace(ln) != "", true)
		}
	})
}
