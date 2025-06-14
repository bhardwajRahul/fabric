/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

// Point is a 2D (X,Y) coordinate.
type Point struct {
	X float64 `json:"x,omitzero" jsonschema:"description=X coordinate,example=6"`
	Y float64 `json:"y,omitzero" jsonschema:"description=Y coordinate,example=8"`
}
