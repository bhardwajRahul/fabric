package petstoreapi

// Pet is a pet in the store. Imported from the Swagger Petstore API.
type Pet struct {
	ID        int64     `json:"id,omitzero" jsonschema_description:"ID is the pet identifier"`
	Name      string    `json:"name,omitzero" jsonschema_description:"Name is the pet name"`
	Category  *Category `json:"category,omitzero" jsonschema_description:"Category is the pet category"`
	PhotoURLs []string  `json:"photoUrls,omitzero" jsonschema_description:"PhotoURLs are the URLs of the pet photos"`
	Tags      []Tag     `json:"tags,omitzero" jsonschema_description:"Tags are the pet tags"`
	Status    string    `json:"status,omitzero" jsonschema_description:"Status is the pet status in the store"`
}
