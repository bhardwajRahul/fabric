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

// oapiDoc is the root object of the OpenAPI document.
// https://spec.openapis.org/oas/v3.1.0#openapi-object
type oapiDoc struct {
	OpenAPI    string                               `json:"openapi"`
	Info       oapiInfo                             `json:"info"`
	Servers    []*oapiServer                        `json:"servers,omitzero"`
	Paths      map[string]map[string]*oapiOperation `json:"paths,omitzero"`
	Components *oapiComponents                      `json:"components,omitzero"`
}

// oapiInfo provides metadata about the API.
// https://spec.openapis.org/oas/v3.1.0#info-object
type oapiInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitzero"`
	Version     string `json:"version,omitzero"`
}

// oapiServer represents a server.
// https://spec.openapis.org/oas/v3.1.0#server-object
type oapiServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitzero"`
}

// oapiOperation describes a single API operation on a path.
// https://spec.openapis.org/oas/v3.1.0#operation-object
type oapiOperation struct {
	Summary     string                     `json:"summary"`
	Description string                     `json:"description,omitzero"`
	Parameters  []*oapiParameter           `json:"parameters,omitzero"`
	RequestBody *oapiRequestBody           `json:"requestBody,omitzero"`
	Responses   map[string]*oapiResponse   `json:"responses,omitzero"`
	Security    []*oapiSecurityRequirement `json:"security,omitzero"`
}

// oapiComponents holds a set of reusable objects for different aspects of the OpenAPI schema.
// https://spec.openapis.org/oas/v3.1.0#components-object
type oapiComponents struct {
	Schemas         map[string]*jsonschema.Schema  `json:"schemas,omitzero"`
	SecuritySchemes map[string]*oapiSecurityScheme `json:"securitySchemes,omitzero"`
}

// oapiParameter describes a single operation parameter.
// https://spec.openapis.org/oas/v3.1.0#parameter-object
type oapiParameter struct {
	Name        string             `json:"name"`
	In          string             `json:"in"`
	Description string             `json:"description,omitzero"`
	Schema      *jsonschema.Schema `json:"schema,omitzero"`
	Style       string             `json:"style,omitzero"`
	Explode     bool               `json:"explode,omitzero"`
	Required    bool               `json:"required,omitzero"`
}

// oapiRequestBody describes a single request body.
// https://spec.openapis.org/oas/v3.1.0#request-body-object
type oapiRequestBody struct {
	Description string                    `json:"description,omitzero"`
	Required    bool                      `json:"required,omitzero"`
	Content     map[string]*oapiMediaType `json:"content,omitzero"`
}

// oapiMediaType provides schema and examples for the media type identified by its key.
// https://spec.openapis.org/oas/v3.1.0#media-type-object
type oapiMediaType struct {
	Description string             `json:"description,omitzero"`
	Schema      *jsonschema.Schema `json:"schema,omitzero"`
}

// oapiResponse describes a single response from an API Operation.
// https://spec.openapis.org/oas/v3.1.0#response-object
type oapiResponse struct {
	Description string                    `json:"description,omitzero"`
	Content     map[string]*oapiMediaType `json:"content,omitzero"`
}

// oapiSecurityScheme describes means of authentication.
// https://spec.openapis.org/oas/v3.1.0#security-scheme-object
type oapiSecurityScheme struct {
	Type         string `json:"type,omitzero"`
	Description  string `json:"description,omitzero"`
	Scheme       string `json:"scheme,omitzero"`
	BearerFormat string `json:"bearerFormat,omitzero"`
}

// oapiSecurityRequirement specifies a security scheme required to access an API Operation.
// https://spec.openapis.org/oas/v3.1.0#security-requirement-object
type oapiSecurityRequirement map[string][]string
