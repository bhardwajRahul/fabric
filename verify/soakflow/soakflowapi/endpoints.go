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

package soakflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "soakflow.verify"

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
	Seed       = Def{Method: "POST", Route: ":428/seed"}        // MARKER: Seed
	FanA       = Def{Method: "POST", Route: ":428/fan-a"}       // MARKER: FanA
	Work       = Def{Method: "POST", Route: ":428/work"}        // MARKER: Work
	Collect    = Def{Method: "POST", Route: ":428/collect"}     // MARKER: Collect
	Loop       = Def{Method: "POST", Route: ":428/loop"}        // MARKER: Loop
	BoomR      = Def{Method: "POST", Route: ":428/boom-r"}      // MARKER: BoomR
	Recover    = Def{Method: "POST", Route: ":428/recover"}     // MARKER: Recover
	BoomF      = Def{Method: "POST", Route: ":428/boom-f"}      // MARKER: BoomF
	Join       = Def{Method: "POST", Route: ":428/join"}        // MARKER: Join
	InnerEntry = Def{Method: "POST", Route: ":428/inner-entry"} // MARKER: InnerEntry
	RunSub     = Def{Method: "POST", Route: ":428/run-sub"}     // MARKER: RunSub
	Soak       = Def{Method: "GET", Route: ":428/soak"}         // MARKER: Soak
	Inner      = Def{Method: "GET", Route: ":428/inner"}        // MARKER: Inner
)

// SeedIn are the input arguments of Seed.
type SeedIn struct { // MARKER: Seed
}

// SeedOut are the output arguments of Seed.
type SeedOut struct { // MARKER: Seed
	Done bool `json:"done,omitzero"`
}

// FanAIn are the input arguments of FanA.
type FanAIn struct { // MARKER: FanA
}

// FanAOut are the output arguments of FanA.
type FanAOut struct { // MARKER: FanA
	Done bool `json:"done,omitzero"`
}

// WorkIn are the input arguments of Work.
type WorkIn struct { // MARKER: Work
}

// WorkOut are the output arguments of Work.
type WorkOut struct { // MARKER: Work
	Done bool `json:"done,omitzero"`
}

// CollectIn are the input arguments of Collect.
type CollectIn struct { // MARKER: Collect
}

// CollectOut are the output arguments of Collect.
type CollectOut struct { // MARKER: Collect
	Done bool `json:"done,omitzero"`
}

// LoopIn are the input arguments of Loop.
type LoopIn struct { // MARKER: Loop
}

// LoopOut are the output arguments of Loop.
type LoopOut struct { // MARKER: Loop
	Done bool `json:"done,omitzero"`
}

// BoomRIn are the input arguments of BoomR.
type BoomRIn struct { // MARKER: BoomR
}

// BoomROut are the output arguments of BoomR.
type BoomROut struct { // MARKER: BoomR
	Done bool `json:"done,omitzero"`
}

// RecoverIn are the input arguments of Recover.
type RecoverIn struct { // MARKER: Recover
}

// RecoverOut are the output arguments of Recover.
type RecoverOut struct { // MARKER: Recover
	Done bool `json:"done,omitzero"`
}

// BoomFIn are the input arguments of BoomF.
type BoomFIn struct { // MARKER: BoomF
}

// BoomFOut are the output arguments of BoomF.
type BoomFOut struct { // MARKER: BoomF
	Done bool `json:"done,omitzero"`
}

// JoinIn are the input arguments of Join.
type JoinIn struct { // MARKER: Join
}

// JoinOut are the output arguments of Join.
type JoinOut struct { // MARKER: Join
	Done bool `json:"done,omitzero"`
}

// InnerEntryIn are the input arguments of InnerEntry.
type InnerEntryIn struct { // MARKER: InnerEntry
}

// InnerEntryOut are the output arguments of InnerEntry.
type InnerEntryOut struct { // MARKER: InnerEntry
	Done bool `json:"done,omitzero"`
}

// RunSubIn are the input arguments of RunSub.
type RunSubIn struct { // MARKER: RunSub
}

// RunSubOut are the output arguments of RunSub.
type RunSubOut struct { // MARKER: RunSub
	Done bool `json:"done,omitzero"`
}

// SoakIn are the input arguments of Soak.
type SoakIn struct { // MARKER: Soak
}

// SoakOut are the output arguments of Soak.
type SoakOut struct { // MARKER: Soak
	Done bool `json:"done,omitzero"`
}

// InnerIn are the input arguments of Inner.
type InnerIn struct { // MARKER: Inner
}

// InnerOut are the output arguments of Inner.
type InnerOut struct { // MARKER: Inner
	Done bool `json:"done,omitzero"`
}
