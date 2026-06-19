package petstoreapi

// Category is a pet category. Imported from the Swagger Petstore API.
type Category struct {
	ID   int64  `json:"id,omitzero" jsonschema_description:"ID is the category identifier"`
	Name string `json:"name,omitzero" jsonschema_description:"Name is the category name"`
}
