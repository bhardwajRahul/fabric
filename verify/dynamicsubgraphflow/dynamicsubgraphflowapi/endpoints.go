package dynamicsubgraphflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "dynamicsubgraphflow.verify"

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
	Parent          = Def{Method: "POST", Route: ":428/parent"}            // MARKER: Parent
	InnerA          = Def{Method: "POST", Route: ":428/inner-a"}           // MARKER: InnerA
	InnerB          = Def{Method: "POST", Route: ":428/inner-b"}           // MARKER: InnerB
	Inner           = Def{Method: "GET", Route: ":428/inner"}              // MARKER: Inner
	DynamicSubgraph = Def{Method: "GET", Route: ":428/dynamic-subgraph"}   // MARKER: DynamicSubgraph
)

// ParentIn are the input arguments of Parent. Reads value (caller-supplied),
// and innerDone/innerResult (set by the child on first run completion).
type ParentIn struct { // MARKER: Parent
	Value       int  `json:"value,omitzero"`
	InnerDone   bool `json:"innerDone,omitzero"`
	InnerResult int  `json:"innerResult,omitzero"`
}

// ParentOut are the output arguments of Parent.
type ParentOut struct { // MARKER: Parent
	ParentResult string `json:"parentResult,omitzero"`
}

// InnerAIn are the input arguments of InnerA.
type InnerAIn struct { // MARKER: InnerA
	Value int `json:"value,omitzero"`
}

// InnerAOut are the output arguments of InnerA.
type InnerAOut struct { // MARKER: InnerA
	InnerStage int `json:"innerStage,omitzero"`
}

// InnerBIn are the input arguments of InnerB.
type InnerBIn struct { // MARKER: InnerB
	InnerStage int `json:"innerStage,omitzero"`
}

// InnerBOut are the output arguments of InnerB. The child sets innerDone=true
// so the parent task can detect re-entry.
type InnerBOut struct { // MARKER: InnerB
	InnerResult int  `json:"innerResult,omitzero"`
	InnerDone   bool `json:"innerDone,omitzero"`
}

// InnerIn are the input arguments of Inner.
type InnerIn struct { // MARKER: Inner
	Value int `json:"value,omitzero"`
}

// InnerOut are the output arguments of Inner. Both fields cross back into the parent.
type InnerOut struct { // MARKER: Inner
	InnerResult int  `json:"innerResult,omitzero"`
	InnerDone   bool `json:"innerDone,omitzero"`
}

// DynamicSubgraphIn are the input arguments of DynamicSubgraph.
type DynamicSubgraphIn struct { // MARKER: DynamicSubgraph
	Value int `json:"value,omitzero"`
}

// DynamicSubgraphOut are the output arguments of DynamicSubgraph.
type DynamicSubgraphOut struct { // MARKER: DynamicSubgraph
	ParentResult string `json:"parentResult,omitzero"`
}
