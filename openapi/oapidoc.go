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

package openapi

import "github.com/invopop/jsonschema"

// Doc is the root object of the OpenAPI document.
// https://spec.openapis.org/oas/v3.1.0#openapi-object
type Doc struct {
	OpenAPI    string                           `json:"openapi"`
	Info       Info                             `json:"info"`
	Servers    []*Server                        `json:"servers,omitzero"`
	Paths      map[string]map[string]*Operation `json:"paths,omitzero"`
	Components *Components                      `json:"components,omitzero"`
}

// Info provides metadata about the API.
// https://spec.openapis.org/oas/v3.1.0#info-object
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitzero"`
	Version     string `json:"version,omitzero"`
}

// Server represents a server.
// https://spec.openapis.org/oas/v3.1.0#server-object
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitzero"`
}

// Operation describes a single API operation on a path.
// https://spec.openapis.org/oas/v3.1.0#operation-object
type Operation struct {
	Summary      string                 `json:"summary"`
	Description  string                 `json:"description,omitzero"`
	XFeatureType string                 `json:"x-feature-type,omitzero"`
	Parameters   []*Parameter           `json:"parameters,omitzero"`
	RequestBody  *RequestBody           `json:"requestBody,omitzero"`
	Responses    map[string]*Response   `json:"responses,omitzero"`
	Security     []*SecurityRequirement `json:"security,omitzero"`
}

// Components holds a set of reusable objects for different aspects of the OpenAPI schema.
// https://spec.openapis.org/oas/v3.1.0#components-object
type Components struct {
	Schemas         map[string]*jsonschema.Schema `json:"schemas,omitzero"`
	SecuritySchemes map[string]*SecurityScheme    `json:"securitySchemes,omitzero"`
}

// Parameter describes a single operation parameter.
// https://spec.openapis.org/oas/v3.1.0#parameter-object
type Parameter struct {
	Name        string             `json:"name"`
	In          string             `json:"in"`
	Description string             `json:"description,omitzero"`
	Schema      *jsonschema.Schema `json:"schema,omitzero"`
	Style       string             `json:"style,omitzero"`
	Explode     bool               `json:"explode,omitzero"`
	Required    bool               `json:"required,omitzero"`
}

// RequestBody describes a single request body.
// https://spec.openapis.org/oas/v3.1.0#request-body-object
type RequestBody struct {
	Description string                `json:"description,omitzero"`
	Required    bool                  `json:"required,omitzero"`
	Content     map[string]*MediaType `json:"content,omitzero"`
}

// MediaType provides schema and examples for the media type identified by its key.
// https://spec.openapis.org/oas/v3.1.0#media-type-object
type MediaType struct {
	Description string             `json:"description,omitzero"`
	Schema      *jsonschema.Schema `json:"schema,omitzero"`
}

// Response describes a single response from an API Operation.
// https://spec.openapis.org/oas/v3.1.0#response-object
type Response struct {
	Description string                `json:"description,omitzero"`
	Content     map[string]*MediaType `json:"content,omitzero"`
}

// SecurityScheme describes means of authentication.
// https://spec.openapis.org/oas/v3.1.0#security-scheme-object
type SecurityScheme struct {
	Type         string `json:"type,omitzero"`
	Description  string `json:"description,omitzero"`
	Scheme       string `json:"scheme,omitzero"`
	BearerFormat string `json:"bearerFormat,omitzero"`
}

// SecurityRequirement specifies a security scheme required to access an API Operation.
// https://spec.openapis.org/oas/v3.1.0#security-requirement-object
type SecurityRequirement map[string][]string
