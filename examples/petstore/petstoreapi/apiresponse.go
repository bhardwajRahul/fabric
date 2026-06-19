package petstoreapi

// ApiResponse is a generic response from the Swagger Petstore API. Imported from the Swagger Petstore API.
type ApiResponse struct {
	Code    int32  `json:"code,omitzero" jsonschema_description:"Code is the response code"`
	Type    string `json:"type,omitzero" jsonschema_description:"Type is the response type"`
	Message string `json:"message,omitzero" jsonschema_description:"Message is the response message"`
}
