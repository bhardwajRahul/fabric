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

// TranscribeChunkOut are the output arguments of TranscribeChunk. The list* prefix
// selects the append reducer; each chunk contributes a single-element delta.
type TranscribeChunkOut struct { // MARKER: TranscribeChunk
	ListTranscriptions []string `json:"listTranscriptions,omitzero"`
}

// JoinPageTranscriptionsIn are the input arguments of JoinPageTranscriptions.
type JoinPageTranscriptionsIn struct { // MARKER: JoinPageTranscriptions
	ListTranscriptions []string `json:"listTranscriptions,omitzero"`
}

// JoinPageTranscriptionsOut are the output arguments of JoinPageTranscriptions.
type JoinPageTranscriptionsOut struct { // MARKER: JoinPageTranscriptions
	ListPageTexts []string `json:"listPageTexts,omitzero"`
}

// JoinDocTranscriptionsIn are the input arguments of JoinDocTranscriptions.
type JoinDocTranscriptionsIn struct { // MARKER: JoinDocTranscriptions
	ListPageTexts []string `json:"listPageTexts,omitzero"`
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
