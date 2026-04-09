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

// FlowSummary is a summary of a flow for listing purposes.
type FlowSummary struct {
	FlowKey      string `json:"flowKey,omitzero"`
	ThreadKey    string `json:"threadKey,omitzero"`
	WorkflowName string `json:"workflowName,omitzero"`
	Status       string `json:"status,omitzero"`
	TaskName     string `json:"taskName,omitzero"`
}
