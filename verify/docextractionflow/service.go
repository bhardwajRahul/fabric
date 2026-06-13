/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docextractionflow

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/docextractionflow/docextractionflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ docextractionflowapi.Client
)

/*
Service implements docextractionflow.verify, a simulated document-extraction pipeline
that exercises nested forEach fan-out (pages, then chunks) with two levels of explicit
fan-in, per-chunk simulated latency, and a 5% failure rate with bounded retry.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

var vocabulary = strings.Fields(
	"the quick brown fox jumps over a lazy dog invoice total amount due date " +
		"customer address signature page section clause amount paid balance " +
		"remit payment terms net thirty account number reference item quantity")

// randomSentence returns a pseudo-random sentence of 8 to 20 words.
func randomSentence() string {
	n := 8 + rand.IntN(13) // 8..20
	words := make([]string, n)
	for i := range words {
		words[i] = vocabulary[rand.IntN(len(vocabulary))]
	}
	return strings.Join(words, " ")
}

/*
ScanPDF is the entry task. It simulates rasterizing a PDF: a ~100ms delay, then 5-22
pages of random image data (~50-150 KB each). It is the forEach source over pages.
*/
func (svc *Service) ScanPDF(ctx context.Context, flow *workflow.Flow, pdf []byte) (pageImages [][]byte, pageCount int, err error) { // MARKER: ScanPDF
	flow.Set("pdf", nil) // Don't copy down the chain
	err = svc.Sleep(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, 0, err
	}
	pageCount = 5 + rand.IntN(18) // 5..22
	pageImages = make([][]byte, pageCount)
	for p := range pageImages {
		size := 50_000 + rand.IntN(100_001) // ~50-150 KB
		img := make([]byte, size)
		for i := range img {
			img[i] = byte(rand.UintN(256))
		}
		pageImages[p] = img
	}
	return pageImages, pageCount, nil
}

/*
IdentifyChunks runs once per page. It simulates region detection, returning 2-5
chunk rectangles. It is the inner forEach source over chunks.
*/
func (svc *Service) IdentifyChunks(ctx context.Context, flow *workflow.Flow, page []byte) (chunks []docextractionflowapi.Rectangle, err error) { // MARKER: IdentifyChunks
	n := 2 + rand.IntN(4) // 2..5, always >=1 so the inner forEach never empties
	chunks = make([]docextractionflowapi.Rectangle, n)
	for i := range chunks {
		chunks[i] = docextractionflowapi.Rectangle{
			X: rand.IntN(1000),
			Y: rand.IntN(1400),
			W: 50 + rand.IntN(950),
			H: 20 + rand.IntN(400),
		}
	}
	return chunks, nil
}

/*
TranscribeChunk runs once per chunk. It simulates OCR latency (50-150ms) and a 5%
failure rate; on failure it retries via flow.Retry (100 attempts, constant 500ms, no
backoff). On success it contributes one transcription as the single-element
transcriptions delta (append reducer at fan-in).
*/
func (svc *Service) TranscribeChunk(ctx context.Context, flow *workflow.Flow, page []byte, chunk docextractionflowapi.Rectangle) (transcriptions []string, err error) { // MARKER: TranscribeChunk
	flow.Set("page", nil) // Don't copy down the chain
	err = svc.Sleep(ctx, 50*time.Millisecond+time.Duration(rand.IntN(101))*time.Millisecond) // 50-150ms
	if err != nil {
		return nil, err
	}
	if rand.Float64() < 0.05 {
		// Constant 500ms delay, 100 attempts: at a 5% failure rate this effectively
		// never exhausts and adds negligible, near-constant latency - so neither
		// retry exhaustion nor backoff inflation can explain a hang.
		if flow.Retry(100, 500*time.Millisecond, 1.0, 500*time.Millisecond) {
			return nil, nil
		}
		return nil, errors.New("transcription failed after exhausting retries")
	}
	return []string{randomSentence()}, nil
}

/*
JoinPageTranscriptions is the inner fan-in (over a page's chunks). It joins the
page's chunk transcriptions and contributes the page text as the single-element
pageTexts delta (append reducer at the outer fan-in).
*/
func (svc *Service) JoinPageTranscriptions(ctx context.Context, flow *workflow.Flow, transcriptions []string) (pageTexts []string, err error) { // MARKER: JoinPageTranscriptions
	return []string{strings.Join(transcriptions, " ")}, nil
}

/*
JoinDocTranscriptions is the outer fan-in (over all pages). It joins the page
texts into the final document transcription, one page per line.
*/
func (svc *Service) JoinDocTranscriptions(ctx context.Context, flow *workflow.Flow, pageTexts []string) (docTranscription string, err error) { // MARKER: JoinDocTranscriptions
	return strings.Join(pageTexts, "\n"), nil
}

/*
DocExtraction defines the graph:
ScanPDF -forEach(pageImages as page)-> IdentifyChunks -forEach(chunks as chunk)->
TranscribeChunk -[fan-in]-> JoinPageTranscriptions -[fan-in]-> JoinDocTranscriptions.
*/
func (svc *Service) DocExtraction(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: DocExtraction
	graph = workflow.NewGraph(docextractionflowapi.DocExtraction.URL())
	graph.AddTask("scanPDF", docextractionflowapi.ScanPDF.URL())
	graph.AddTask("identifyChunks", docextractionflowapi.IdentifyChunks.URL())
	graph.AddTask("transcribeChunk", docextractionflowapi.TranscribeChunk.URL())
	graph.AddTask("joinPageTranscriptions", docextractionflowapi.JoinPageTranscriptions.URL())
	graph.AddTask("joinDocTranscriptions", docextractionflowapi.JoinDocTranscriptions.URL())
	graph.SetFanIn("joinPageTranscriptions") // inner: chunks -> page
	graph.SetFanIn("joinDocTranscriptions")  // outer: pages -> doc
	graph.SetReducer("transcriptions", workflow.ReducerAppend)
	graph.SetReducer("pageTexts", workflow.ReducerAppend)
	graph.AddTransitionForEach("scanPDF", "identifyChunks", "pageImages", "page")
	graph.AddTransitionForEach("identifyChunks", "transcribeChunk", "chunks", "chunk")
	graph.AddTransition("transcribeChunk", "joinPageTranscriptions")
	graph.AddTransition("joinPageTranscriptions", "joinDocTranscriptions")
	graph.AddTransition("joinDocTranscriptions", workflow.END)
	return graph, nil
}
