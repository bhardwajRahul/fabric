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

package docextractionflowapi

// Rectangle is an axis-aligned region of a page image, in pixels.
type Rectangle struct {
	X int `json:"x,omitzero" jsonschema:"description=X is the left coordinate in pixels"`
	Y int `json:"y,omitzero" jsonschema:"description=Y is the top coordinate in pixels"`
	W int `json:"w,omitzero" jsonschema:"description=W is the width in pixels"`
	H int `json:"h,omitzero" jsonschema:"description=H is the height in pixels"`
}
