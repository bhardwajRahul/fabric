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

package svcapi

import (
	"github.com/microbus-io/fabric/cmd/genservice/testdata/pressuretest/srcapi"
)

// Pet is a domain type referenced by the magic-HTTP-arg endpoints.
type Pet struct {
	Name string `json:"name,omitzero" jsonschema_description:"Name is the pet's name"`
	Tag  string `json:"tag,omitzero"`
}

// Item is a domain type referenced by task and workflow In/Out structs.
type Item struct {
	ID    string `json:"id,omitzero"`
	Qty   int    `json:"qty,omitzero"`
	Flags []bool `json:"flags,omitzero"`
}

// SrcThing is a type alias re-exporting the upstream domain type, exercising the alias sibling-file
// convention (type X = otherapi.X).
type SrcThing = srcapi.SrcThing

// Policy is a structured configuration value, referenced by a Config's struct type carrier.
type Policy struct {
	Mode string `json:"mode,omitzero"`
}
