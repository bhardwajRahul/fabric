package continueflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "continueflow.verify"

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
	Increment = Def{Method: "POST", Route: ":428/increment"} // MARKER: Increment
	Counting  = Def{Method: "GET", Route: ":428/counting"}   // MARKER: Counting
)

// IncrementIn are the input arguments of Increment.
type IncrementIn struct { // MARKER: Increment
	Counter int `json:"counter,omitzero"`
}

// IncrementOut are the output arguments of Increment.
type IncrementOut struct { // MARKER: Increment
	CounterOut int `json:"counter,omitzero"`
}

// CountingIn are the input arguments of Counting.
type CountingIn struct { // MARKER: Counting
	Counter int `json:"counter,omitzero"`
}

// CountingOut are the output arguments of Counting. Same JSON tag as the input
// so the field persists across Continue turns.
type CountingOut struct { // MARKER: Counting
	CounterOut int `json:"counter,omitzero"`
}
