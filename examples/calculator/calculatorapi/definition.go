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
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "calculator.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Calculator"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 353

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Calculator microservice performs simple mathematical operations.`

// UsedOperators tracks the types of the arithmetic operators used.
var UsedOperators = define.Metric{ // MARKER: UsedOperators
	Kind: define.Counter, Value: int(0), Labels: []string{"op"},
	OTelName: "used_operators",
}

// SumOperations tracks the total sum of the results of all operators.
var SumOperations = define.Metric{ // MARKER: SumOperations
	Kind: define.Gauge, Value: int(0), Labels: []string{"op"},
	OTelName: "sum_operations", Observable: true,
}

// Arithmetic performs an arithmetic operation between two integers x and y given an operator op.
var Arithmetic = define.Function{ // MARKER: Arithmetic
	Host: Hostname, Method: "GET", Route: ":443/arithmetic",
	In: ArithmeticIn{}, Out: ArithmeticOut{},
}

// ArithmeticIn are the input arguments of Arithmetic.
type ArithmeticIn struct { // MARKER: Arithmetic
	X  int    `json:"x,omitzero" jsonschema:"description=X is the left operand"`
	Op string `json:"op,omitzero" jsonschema:"description=Op is the operator: + - * /"`
	Y  int    `json:"y,omitzero" jsonschema:"description=Y is the right operand"`
}

// ArithmeticOut are the output arguments of Arithmetic.
type ArithmeticOut struct { // MARKER: Arithmetic
	XEcho  int    `json:"xEcho,omitzero" jsonschema:"description=XEcho echoes the left operand"`
	OpEcho string `json:"opEcho,omitzero" jsonschema:"description=OpEcho echoes the operator"`
	YEcho  int    `json:"yEcho,omitzero" jsonschema:"description=YEcho echoes the right operand"`
	Result int    `json:"result,omitzero" jsonschema:"description=Result is the result of the operation"`
}

// Square prints the square of the integer x.
var Square = define.Function{ // MARKER: Square
	Host: Hostname, Method: "GET", Route: ":443/square",
	In: SquareIn{}, Out: SquareOut{},
}

// SquareIn are the input arguments of Square.
type SquareIn struct { // MARKER: Square
	X int `json:"x,omitzero" jsonschema:"description=X is the integer to square"`
}

// SquareOut are the output arguments of Square.
type SquareOut struct { // MARKER: Square
	XEcho  int `json:"xEcho,omitzero" jsonschema:"description=XEcho echoes the input integer"`
	Result int `json:"result,omitzero" jsonschema:"description=Result is X squared"`
}

// Distance calculates the distance between two points. It demonstrates the use of the defined type Point.
var Distance = define.Function{ // MARKER: Distance
	Host: Hostname, Method: "ANY", Route: ":443/distance",
	In: DistanceIn{}, Out: DistanceOut{},
}

// DistanceIn are the input arguments of Distance.
type DistanceIn struct { // MARKER: Distance
	P1 Point `json:"p1,omitzero" jsonschema:"description=P1 is the first point"`
	P2 Point `json:"p2,omitzero" jsonschema:"description=P2 is the second point"`
}

// DistanceOut are the output arguments of Distance.
type DistanceOut struct { // MARKER: Distance
	D float64 `json:"d,omitzero" jsonschema:"description=D is the Euclidean distance between P1 and P2"`
}
