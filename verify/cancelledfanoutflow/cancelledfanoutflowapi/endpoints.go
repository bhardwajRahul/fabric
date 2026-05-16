package cancelledfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "cancelledfanoutflow.verify"

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
	Source         = Def{Method: "POST", Route: ":428/source"}              // MARKER: Source
	A              = Def{Method: "POST", Route: ":428/a"}                   // MARKER: A
	B              = Def{Method: "POST", Route: ":428/b"}                   // MARKER: B
	C              = Def{Method: "POST", Route: ":428/c"}                   // MARKER: C
	J              = Def{Method: "POST", Route: ":428/j"}                   // MARKER: J
	CancelledFanOut = Def{Method: "GET", Route: ":428/cancelled-fan-out"}   // MARKER: CancelledFanOut
)

// SourceIn are the input arguments of Source.
type SourceIn struct { // MARKER: Source
}

// SourceOut are the output arguments of Source.
type SourceOut struct { // MARKER: Source
	Started bool `json:"started,omitzero"`
}

// AIn are the input arguments of A.
type AIn struct { // MARKER: A
}

// AOut are the output arguments of A.
type AOut struct { // MARKER: A
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// BIn are the input arguments of B.
type BIn struct { // MARKER: B
}

// BOut are the output arguments of B.
type BOut struct { // MARKER: B
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// CIn are the input arguments of C.
type CIn struct { // MARKER: C
}

// COut are the output arguments of C.
type COut struct { // MARKER: C
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// JIn are the input arguments of J.
type JIn struct { // MARKER: J
	SumExecuted int `json:"sumExecuted,omitzero"`
}

// JOut are the output arguments of J.
type JOut struct { // MARKER: J
	TotalExecuted int `json:"totalExecuted,omitzero"`
}

// CancelledFanOutIn are the input arguments of CancelledFanOut.
type CancelledFanOutIn struct { // MARKER: CancelledFanOut
}

// CancelledFanOutOut are the output arguments of CancelledFanOut.
type CancelledFanOutOut struct { // MARKER: CancelledFanOut
	SumExecuted   int `json:"sumExecuted,omitzero"`
	TotalExecuted int `json:"totalExecuted,omitzero"`
}
