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

package foremanapi

import "time"

// FlowStep represents a single step in the execution history of a flow.
type FlowStep struct {
	StepKey    string         `json:"stepKey,omitzero"`
	StepDepth  int            `json:"stepDepth,omitzero"`
	TaskName   string         `json:"taskName,omitzero"`
	Subgraph   bool           `json:"subgraph,omitzero"`
	SubHistory []FlowStep     `json:"subHistory,omitzero"`
	State      map[string]any `json:"state,omitzero"`
	Changes    map[string]any `json:"changes,omitzero"`
	Status     string         `json:"status,omitzero"`
	Error      string         `json:"error,omitzero"`
	UpdatedAt  time.Time      `json:"updatedAt,omitzero"`
}
