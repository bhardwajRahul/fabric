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

package docextractionflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "docextractionflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	ScanPDF                = Def{Method: "POST", Route: ":428/scan-pdf"}                  // MARKER: ScanPDF
	IdentifyChunks         = Def{Method: "POST", Route: ":428/identify-chunks"}           // MARKER: IdentifyChunks
	TranscribeChunk        = Def{Method: "POST", Route: ":428/transcribe-chunk"}          // MARKER: TranscribeChunk
	JoinPageTranscriptions = Def{Method: "POST", Route: ":428/join-page-transcriptions"} // MARKER: JoinPageTranscriptions
	JoinDocTranscriptions  = Def{Method: "POST", Route: ":428/join-doc-transcriptions"}  // MARKER: JoinDocTranscriptions
	DocExtraction          = Def{Method: "GET", Route: ":428/doc-extraction"}            // MARKER: DocExtraction
)

// ScanPDFIn are the input arguments of ScanPDF.
type ScanPDFIn struct { // MARKER: ScanPDF
	Pdf []byte `json:"pdf,omitzero"`
}

// ScanPDFOut are the output arguments of ScanPDF.
type ScanPDFOut struct { // MARKER: ScanPDF
	PageImages [][]byte `json:"pageImages,omitzero"`
	PageCount  int      `json:"pageCount,omitzero"`
}

// IdentifyChunksIn are the input arguments of IdentifyChunks.
type IdentifyChunksIn struct { // MARKER: IdentifyChunks
	Page []byte `json:"page,omitzero"`
}

// IdentifyChunksOut are the output arguments of IdentifyChunks.
type IdentifyChunksOut struct { // MARKER: IdentifyChunks
	Chunks []Rectangle `json:"chunks,omitzero"`
}

// TranscribeChunkIn are the input arguments of TranscribeChunk.
type TranscribeChunkIn struct { // MARKER: TranscribeChunk
	Page  []byte    `json:"page,omitzero"`
	Chunk Rectangle `json:"chunk,omitzero"`
}

// TranscribeChunkOut are the output arguments of TranscribeChunk. The `transcriptions`
// field is Append-reduced; each chunk contributes a single-element delta.
type TranscribeChunkOut struct { // MARKER: TranscribeChunk
	Transcriptions []string `json:"transcriptions,omitzero"`
}

// JoinPageTranscriptionsIn are the input arguments of JoinPageTranscriptions.
type JoinPageTranscriptionsIn struct { // MARKER: JoinPageTranscriptions
	Transcriptions []string `json:"transcriptions,omitzero"`
}

// JoinPageTranscriptionsOut are the output arguments of JoinPageTranscriptions.
type JoinPageTranscriptionsOut struct { // MARKER: JoinPageTranscriptions
	PageTexts []string `json:"pageTexts,omitzero"`
}

// JoinDocTranscriptionsIn are the input arguments of JoinDocTranscriptions.
type JoinDocTranscriptionsIn struct { // MARKER: JoinDocTranscriptions
	PageTexts []string `json:"pageTexts,omitzero"`
}

// JoinDocTranscriptionsOut are the output arguments of JoinDocTranscriptions.
type JoinDocTranscriptionsOut struct { // MARKER: JoinDocTranscriptions
	DocTranscription string `json:"docTranscription,omitzero"`
}

// DocExtractionIn are the input arguments of DocExtraction.
type DocExtractionIn struct { // MARKER: DocExtraction
	Pdf []byte `json:"pdf,omitzero"`
}

// DocExtractionOut are the output arguments of DocExtraction.
type DocExtractionOut struct { // MARKER: DocExtraction
	DocTranscription string `json:"docTranscription,omitzero"`
	PageCount        int    `json:"pageCount,omitzero"`
}
