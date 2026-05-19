package petstoreapi

// Tag is a pet tag. Imported from the Swagger Petstore API.
type Tag struct {
	ID   int64  `json:"id,omitzero" jsonschema:"description=ID is the tag identifier"`
	Name string `json:"name,omitzero" jsonschema:"description=Name is the tag name"`
}
