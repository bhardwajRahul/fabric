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

// Package openapi declares the registration types ([Service] and [Endpoint]) that describe
// a microservice's externally-visible features, along with [Render] which converts a
// [Service] into an OpenAPI 3.1 [Document].
package openapi

// Service describes the externally-visible features of a microservice. The [Endpoints] should
// be treated as immutable after registration.
type Service struct {
	ServiceName string
	Description string
	Version     int
	Endpoints   []*Endpoint
	RemoteURI   string
}
