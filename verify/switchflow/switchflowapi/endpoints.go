package switchflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "switchflow.verify"

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
	Router         = Def{Method: "POST", Route: ":428/router"}          // MARKER: Router
	HandleHigh     = Def{Method: "POST", Route: ":428/handle-high"}     // MARKER: HandleHigh
	HandleMid      = Def{Method: "POST", Route: ":428/handle-mid"}      // MARKER: HandleMid
	HandleLow      = Def{Method: "POST", Route: ":428/handle-low"}      // MARKER: HandleLow
	Switch         = Def{Method: "GET", Route: ":428/switch"}           // MARKER: Switch
	SwitchNoMatch  = Def{Method: "GET", Route: ":428/switch-no-match"}  // MARKER: SwitchNoMatch
)

// RouterIn are the input arguments of Router.
type RouterIn struct { // MARKER: Router
	Amount int `json:"amount,omitzero"`
}

// RouterOut are the output arguments of Router.
type RouterOut struct { // MARKER: Router
	AmountOut int `json:"amount,omitzero"`
}

// HandleHighIn are the input arguments of HandleHigh.
type HandleHighIn struct { // MARKER: HandleHigh
	Amount int `json:"amount,omitzero"`
}

// HandleHighOut are the output arguments of HandleHigh.
type HandleHighOut struct { // MARKER: HandleHigh
	Branch string `json:"branch,omitzero"`
}

// HandleMidIn are the input arguments of HandleMid.
type HandleMidIn struct { // MARKER: HandleMid
	Amount int `json:"amount,omitzero"`
}

// HandleMidOut are the output arguments of HandleMid.
type HandleMidOut struct { // MARKER: HandleMid
	Branch string `json:"branch,omitzero"`
}

// HandleLowIn are the input arguments of HandleLow.
type HandleLowIn struct { // MARKER: HandleLow
	Amount int `json:"amount,omitzero"`
}

// HandleLowOut are the output arguments of HandleLow.
type HandleLowOut struct { // MARKER: HandleLow
	Branch string `json:"branch,omitzero"`
}

// SwitchIn are the input arguments of Switch.
type SwitchIn struct { // MARKER: Switch
	Amount int `json:"amount,omitzero"`
}

// SwitchOut are the output arguments of Switch.
type SwitchOut struct { // MARKER: Switch
	Branch string `json:"branch,omitzero"`
}

// SwitchNoMatchIn are the input arguments of SwitchNoMatch.
type SwitchNoMatchIn struct { // MARKER: SwitchNoMatch
	Amount int `json:"amount,omitzero"`
}

// SwitchNoMatchOut are the output arguments of SwitchNoMatch.
type SwitchNoMatchOut struct { // MARKER: SwitchNoMatch
	Branch string `json:"branch,omitzero"`
}
