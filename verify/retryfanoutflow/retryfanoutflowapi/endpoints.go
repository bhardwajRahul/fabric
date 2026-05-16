package retryfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "retryfanoutflow.verify"

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
	Enter       = Def{Method: "POST", Route: ":428/enter"}            // MARKER: Enter
	Increment   = Def{Method: "POST", Route: ":428/increment"}        // MARKER: Increment
	Join        = Def{Method: "POST", Route: ":428/join"}             // MARKER: Join
	RetryFanOut = Def{Method: "GET", Route: ":428/retry-fan-out"}     // MARKER: RetryFanOut
)

// EnterIn are the input arguments of Enter.
type EnterIn struct { // MARKER: Enter
	Elements []int `json:"elements,omitzero"`
}

// EnterOut are the output arguments of Enter.
type EnterOut struct { // MARKER: Enter
	ElementsOut []int `json:"elements,omitzero"`
}

// IncrementIn are the input arguments of Increment.
type IncrementIn struct { // MARKER: Increment
	Element int `json:"element,omitzero"`
}

// IncrementOut are the output arguments of Increment.
type IncrementOut struct { // MARKER: Increment
	ListResultOut []int `json:"listResult,omitzero"`
}

// JoinIn are the input arguments of Join.
type JoinIn struct { // MARKER: Join
	ListResult []int `json:"listResult,omitzero"`
}

// JoinOut are the output arguments of Join.
type JoinOut struct { // MARKER: Join
	ListResultOut []int `json:"listResult,omitzero"`
}

// RetryFanOutIn are the input arguments of RetryFanOut.
type RetryFanOutIn struct { // MARKER: RetryFanOut
	Elements []int `json:"elements,omitzero"`
}

// RetryFanOutOut are the output arguments of RetryFanOut.
type RetryFanOutOut struct { // MARKER: RetryFanOut
	ListResult []int `json:"listResult,omitzero"`
}
