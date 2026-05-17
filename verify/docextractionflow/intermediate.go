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
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/docextractionflow/docextractionflowapi"
	"github.com/microbus-io/fabric/verify/docextractionflow/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ docextractionflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = docextractionflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	ScanPDF(ctx context.Context, flow *workflow.Flow, pdf []byte) (pageImages [][]byte, pageCount int, err error)                                  // MARKER: ScanPDF
	IdentifyChunks(ctx context.Context, flow *workflow.Flow, page []byte) (chunks []docextractionflowapi.Rectangle, err error)                     // MARKER: IdentifyChunks
	TranscribeChunk(ctx context.Context, flow *workflow.Flow, page []byte, chunk docextractionflowapi.Rectangle) (listTranscriptions []string, err error) // MARKER: TranscribeChunk
	JoinPageTranscriptions(ctx context.Context, flow *workflow.Flow, listTranscriptions []string) (listPageTexts []string, err error)             // MARKER: JoinPageTranscriptions
	JoinDocTranscriptions(ctx context.Context, flow *workflow.Flow, listPageTexts []string) (docTranscription string, err error)                  // MARKER: JoinDocTranscriptions
	DocExtraction(ctx context.Context) (graph *workflow.Graph, err error)                                                                         // MARKER: DocExtraction
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`docextractionflow.verify simulates a nested forEach document-extraction pipeline with per-chunk latency, a 5% failure rate, and bounded retry.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: ScanPDF
		"ScanPDF", svc.doScanPDF,
		sub.At(docextractionflowapi.ScanPDF.Method, docextractionflowapi.ScanPDF.Route),
		sub.Description(`ScanPDF rasterizes a PDF into page images (forEach source over pages).`),
		sub.Task(docextractionflowapi.ScanPDFIn{}, docextractionflowapi.ScanPDFOut{}),
	)
	svc.Subscribe( // MARKER: IdentifyChunks
		"IdentifyChunks", svc.doIdentifyChunks,
		sub.At(docextractionflowapi.IdentifyChunks.Method, docextractionflowapi.IdentifyChunks.Route),
		sub.Description(`IdentifyChunks detects chunk rectangles on a page (inner forEach source).`),
		sub.Task(docextractionflowapi.IdentifyChunksIn{}, docextractionflowapi.IdentifyChunksOut{}),
	)
	svc.Subscribe( // MARKER: TranscribeChunk
		"TranscribeChunk", svc.doTranscribeChunk,
		sub.At(docextractionflowapi.TranscribeChunk.Method, docextractionflowapi.TranscribeChunk.Route),
		sub.Description(`TranscribeChunk OCRs one chunk, with simulated latency and a 5% failure rate retried with backoff.`),
		sub.Task(docextractionflowapi.TranscribeChunkIn{}, docextractionflowapi.TranscribeChunkOut{}),
	)
	svc.Subscribe( // MARKER: JoinPageTranscriptions
		"JoinPageTranscriptions", svc.doJoinPageTranscriptions,
		sub.At(docextractionflowapi.JoinPageTranscriptions.Method, docextractionflowapi.JoinPageTranscriptions.Route),
		sub.Description(`JoinPageTranscriptions is the inner fan-in over a page's chunks.`),
		sub.Task(docextractionflowapi.JoinPageTranscriptionsIn{}, docextractionflowapi.JoinPageTranscriptionsOut{}),
	)
	svc.Subscribe( // MARKER: JoinDocTranscriptions
		"JoinDocTranscriptions", svc.doJoinDocTranscriptions,
		sub.At(docextractionflowapi.JoinDocTranscriptions.Method, docextractionflowapi.JoinDocTranscriptions.Route),
		sub.Description(`JoinDocTranscriptions is the outer fan-in over all pages.`),
		sub.Task(docextractionflowapi.JoinDocTranscriptionsIn{}, docextractionflowapi.JoinDocTranscriptionsOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: DocExtraction
		"DocExtraction", svc.doDocExtraction,
		sub.At(docextractionflowapi.DocExtraction.Method, docextractionflowapi.DocExtraction.Route),
		sub.Description(`DocExtraction defines ScanPDF -forEach-> IdentifyChunks -forEach-> TranscribeChunk -> JoinPageTranscriptions -> JoinDocTranscriptions.`),
		sub.Workflow(docextractionflowapi.DocExtractionIn{}, docextractionflowapi.DocExtractionOut{}),
	)

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doScanPDF handles marshaling for ScanPDF.
func (svc *Intermediate) doScanPDF(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ScanPDF
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in docextractionflowapi.ScanPDFIn
	flow.ParseState(&in)
	var out docextractionflowapi.ScanPDFOut
	out.PageImages, out.PageCount, err = svc.ScanPDF(r.Context(), &flow, in.Pdf)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doIdentifyChunks handles marshaling for IdentifyChunks.
func (svc *Intermediate) doIdentifyChunks(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: IdentifyChunks
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in docextractionflowapi.IdentifyChunksIn
	flow.ParseState(&in)
	var out docextractionflowapi.IdentifyChunksOut
	out.Chunks, err = svc.IdentifyChunks(r.Context(), &flow, in.Page)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doTranscribeChunk handles marshaling for TranscribeChunk.
func (svc *Intermediate) doTranscribeChunk(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TranscribeChunk
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in docextractionflowapi.TranscribeChunkIn
	flow.ParseState(&in)
	var out docextractionflowapi.TranscribeChunkOut
	out.ListTranscriptions, err = svc.TranscribeChunk(r.Context(), &flow, in.Page, in.Chunk)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doJoinPageTranscriptions handles marshaling for JoinPageTranscriptions.
func (svc *Intermediate) doJoinPageTranscriptions(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JoinPageTranscriptions
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in docextractionflowapi.JoinPageTranscriptionsIn
	flow.ParseState(&in)
	var out docextractionflowapi.JoinPageTranscriptionsOut
	out.ListPageTexts, err = svc.JoinPageTranscriptions(r.Context(), &flow, in.ListTranscriptions)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doJoinDocTranscriptions handles marshaling for JoinDocTranscriptions.
func (svc *Intermediate) doJoinDocTranscriptions(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JoinDocTranscriptions
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in docextractionflowapi.JoinDocTranscriptionsIn
	flow.ParseState(&in)
	var out docextractionflowapi.JoinDocTranscriptionsOut
	out.DocTranscription, err = svc.JoinDocTranscriptions(r.Context(), &flow, in.ListPageTexts)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doDocExtraction handles marshaling for DocExtraction.
func (svc *Intermediate) doDocExtraction(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DocExtraction
	graph, err := svc.DocExtraction(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
