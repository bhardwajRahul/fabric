/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

package openapi

import (
	"encoding/json"
	"mime"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/microbus-io/fabric/httpx"
)

// Service is populated with the microservice's specs in order to generate its OpenAPI document.
type Service struct {
	ServiceName string
	Description string
	Version     int
	Endpoints   []*Endpoint
	RemoteURI   string
}

// MarshalJSON produces the JSON representation of the OpenAPI document of the service.
func (s *Service) MarshalJSON() ([]byte, error) {
	doc := oapiDoc{
		OpenAPI: "3.1.0",
		Info: oapiInfo{
			Title:       s.ServiceName,
			Description: s.Description,
			Version:     strconv.Itoa(s.Version),
		},
		Paths: map[string]map[string]*oapiOperation{},
		Components: &oapiComponents{
			Schemas: map[string]*jsonschema.Schema{},
		},
		Servers: []*oapiServer{
			{
				URL: "https://localhost/",
			},
		},
	}
	if s.RemoteURI != "" {
		p := strings.Index(s.RemoteURI, "/"+s.ServiceName+"/")
		if p < 0 {
			p = strings.Index(s.RemoteURI, "/"+s.ServiceName+":")
		}
		if p >= 0 {
			doc.Servers[0].URL = s.RemoteURI[:p+1]
		}
	}

	resolveRefs := func(schema *jsonschema.Schema, endpoint string) {
		// Resolve all $def references to the components section of the document
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			if strings.HasPrefix(pair.Value.Ref, "#/$defs/") {
				// #/$defs/ABC ===> #/components/schemas/ENDPOINT_ABC
				pair.Value.Ref = "#/components/schemas/" + endpoint + "_" + pair.Value.Ref[8:]
			}
		}

		// Move $defs into the components section of the document
		// #/$defs/ABC ===> #/components/schemas/ENDPOINT_ABC
		for defKey, defSchema := range schema.Definitions {
			// Resolve all nested $def references to the components section of the document
			for pair := defSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
				if strings.HasPrefix(pair.Value.Ref, "#/$defs/") {
					// #/$defs/ABC ===> #/components/schemas/ENDPOINT_ABC
					pair.Value.Ref = "#/components/schemas/" + endpoint + "_" + pair.Value.Ref[8:]
				}
			}
			doc.Components.Schemas[endpoint+"_"+defKey] = defSchema
		}
		schema.Definitions = nil
		schema.Version = "" // Avoid rendering the $schema property
	}

	for _, ep := range s.Endpoints {
		var op *oapiOperation

		// Functions
		if ep.Type == "function" {
			if ep.Method == "" || ep.Method == "ANY" {
				ep.Method = "POST"
			}
			op = &oapiOperation{
				Summary:     cleanEndpointSummary(ep.Summary),
				Description: ep.Description,
				Responses: map[string]*oapiResponse{
					"200": {
						Description: "OK",
						Content: map[string]*oapiMediaType{
							"application/json": {
								Schema: &jsonschema.Schema{
									Ref: "#/components/schemas/" + ep.Name + "_OUT",
								},
							},
						},
					},
				},
			}

			// OUT is JSON in the response body
			schemaOut := jsonschema.Reflect(ep.OutputArgs)
			resolveRefs(schemaOut, ep.Name+"_OUT")
			doc.Components.Schemas[ep.Name+"_OUT"] = schemaOut

			if !methodHasBody(ep.Method) {
				// IN are explodable query arguments
				inType := reflect.TypeOf(ep.InputArgs)
				for i := 0; i < inType.NumField(); i++ {
					field := inType.Field(i)
					name := fieldName(field)
					if name == "" {
						continue
					}
					parameter := &oapiParameter{
						In:   "query",
						Name: name,
					}
					fieldSchema := jsonschema.ReflectFromType(field.Type)
					resolveRefs(fieldSchema, ep.Name+"_IN")
					if fieldSchema.Ref != "" {
						// Non-primitive type
						parameter.Schema = &jsonschema.Schema{
							Ref: "#/components/schemas/" + ep.Name + "_IN_" + field.Type.Name(),
						}
						parameter.Style = "deepObject"
						parameter.Explode = true
					} else {
						parameter.Schema = fieldSchema
					}
					op.Parameters = append(op.Parameters, parameter)
				}
			} else {
				// IN is JSON in the request body
				schemaIn := jsonschema.Reflect(ep.InputArgs)
				resolveRefs(schemaIn, ep.Name+"_IN")
				doc.Components.Schemas[ep.Name+"_IN"] = schemaIn

				op.RequestBody = &oapiRequestBody{
					Required: true,
					Content: map[string]*oapiMediaType{
						"application/json": {
							Schema: &jsonschema.Schema{
								Ref: "#/components/schemas/" + ep.Name + "_IN",
							},
						},
					},
				}
			}
		}

		if ep.Type == "web" {
			if ep.Method == "" || ep.Method == "ANY" {
				ep.Method = "GET"
			}
			op = &oapiOperation{
				Summary:     cleanEndpointSummary(ep.Summary),
				Description: ep.Description,
				Parameters:  []*oapiParameter{},
				Responses: map[string]*oapiResponse{
					"200": {
						Description: "OK",
					},
				},
			}
			p := strings.LastIndex(ep.Path, ".")
			if p >= 0 {
				contentType := mime.TypeByExtension(ep.Path[p:])
				if contentType != "" {
					op.Responses = map[string]*oapiResponse{
						"200": {
							Content: map[string]*oapiMediaType{
								contentType: {},
							},
						},
					}
				}
			}
		}

		path := httpx.JoinHostAndPath(s.ServiceName, ep.Path)
		_, path, _ = strings.Cut(path, "://")
		path = "/" + path
		// Catch all subscriptions
		if strings.HasSuffix(ep.Path, "/") {
			path += "{suffix}"

			op.Parameters = append(op.Parameters, &oapiParameter{
				In:   "path",
				Name: "suffix",
				Schema: &jsonschema.Schema{
					Type: "string",
				},
				Description: "Suffix of path",
				Required:    true,
			})
		}

		// Add to paths
		if doc.Paths[path] == nil {
			doc.Paths[path] = map[string]*oapiOperation{}
		}
		doc.Paths[path][strings.ToLower(ep.Method)] = op
	}
	return json.Marshal(doc)
}

func cleanEndpointSummary(sig string) string {
	// Remove request/response
	sig = strings.Replace(sig, "(w http.ResponseWriter, r *http.Request)", "()", -1)
	sig = strings.Replace(sig, "(w http.ResponseWriter, r *http.Request, ", "(", -1)
	// Remove ctx argument
	sig = strings.Replace(sig, "(ctx context.Context)", "()", -1)
	sig = strings.Replace(sig, "(ctx context.Context, ", "(", -1)
	// Remove error return value
	sig = strings.Replace(sig, " (err error)", "", -1)
	sig = strings.Replace(sig, ", err error)", ")", -1)
	// Remove pointers
	sig = strings.Replace(sig, "*", "", -1)
	// Remove package identifiers
	sig = regexp.MustCompile(`\w+\.`).ReplaceAllString(sig, "")
	return sig
}

// methodHasBody indicates if the HTTP method has a body.
func methodHasBody(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "DELETE", "TRACE", "OPTIONS", "HEAD":
		return false
	default:
		return true
	}
}

func fieldName(field reflect.StructField) string {
	if field.Name[:1] != strings.ToUpper(field.Name[:1]) {
		// Not a public field
		return ""
	}
	name := field.Tag.Get("json")
	if comma := strings.Index(name, ","); comma >= 0 {
		name = name[:comma]
	}
	if name == "" {
		// No JSON tag, use field name
		name = field.Name
	}
	if name == "-" {
		// Omitted
		name = ""
	}
	return name
}
