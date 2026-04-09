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

package calculatorapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "calculator.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// ArithmeticIn are the input arguments of Arithmetic.
type ArithmeticIn struct { // MARKER: Arithmetic
	X  int    `json:"x,omitzero"`
	Op string `json:"op,omitzero"`
	Y  int    `json:"y,omitzero"`
}

// ArithmeticOut are the output arguments of Arithmetic.
type ArithmeticOut struct { // MARKER: Arithmetic
	XEcho  int    `json:"xEcho,omitzero"`
	OpEcho string `json:"opEcho,omitzero"`
	YEcho  int    `json:"yEcho,omitzero"`
	Result int    `json:"result,omitzero"`
}

// SquareIn are the input arguments of Square.
type SquareIn struct { // MARKER: Square
	X int `json:"x,omitzero"`
}

// SquareOut are the output arguments of Square.
type SquareOut struct { // MARKER: Square
	XEcho  int `json:"xEcho,omitzero"`
	Result int `json:"result,omitzero"`
}

// DistanceIn are the input arguments of Distance.
type DistanceIn struct { // MARKER: Distance
	P1 Point `json:"p1,omitzero"`
	P2 Point `json:"p2,omitzero"`
}

// DistanceOut are the output arguments of Distance.
type DistanceOut struct { // MARKER: Distance
	D float64 `json:"d,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Arithmetic = Def{Method: "GET", Route: ":443/arithmetic"} // MARKER: Arithmetic
	Square     = Def{Method: "GET", Route: ":443/square"}     // MARKER: Square
	Distance   = Def{Method: "ANY", Route: ":443/distance"}   // MARKER: Distance
)
