package petstoreapi

// ApiResponse is a generic response from the Swagger Petstore API. Imported from the Swagger Petstore API.
type ApiResponse struct {
	Code    int32  `json:"code,omitzero" jsonschema:"description=Code is the response code"`
	Type    string `json:"type,omitzero" jsonschema:"description=Type is the response type"`
	Message string `json:"message,omitzero" jsonschema:"description=Message is the response message"`
}
