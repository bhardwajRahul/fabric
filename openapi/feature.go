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

package openapi

// The feature type constants identify the kind of a Microbus endpoint.
// They are assigned to [Endpoint.Type].
const (
	// FeatureFunction is a typed request/response endpoint (RPC).
	FeatureFunction = "function"
	// FeatureWeb is a raw HTTP handler endpoint.
	FeatureWeb = "web"
	// FeatureWorkflow is a workflow graph endpoint.
	FeatureWorkflow = "workflow"
	// FeatureTask is a workflow task endpoint.
	FeatureTask = "task"
	// FeatureOutboundEvent is an event published by the microservice. Its route is
	// what subscribers bind their inbound handlers to. Outbound events are not
	// rendered in the OpenAPI doc and are not exposed as LLM tools.
	FeatureOutboundEvent = "outboundevent"
)
