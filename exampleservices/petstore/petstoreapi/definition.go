package petstoreapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "petstore.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Petstore"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 1

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Petstore delegates to the Swagger Petstore API.

This is a sample Pet Store Server based on the OpenAPI 3.0 specification.  You can find out more about
Swagger at [https://swagger.io](https://swagger.io). In the third iteration of the pet store, we've switched to the design first approach!
You can now help us improve the API whether it's by making changes to the definition itself or to the code.
That way, with time, we can improve the API in general, and expose some of the new features in OAS3.

Some useful links:
- [The Pet Store repository](https://github.com/swagger-api/swagger-petstore)
- [The source API definition for the Pet Store](https://github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml)`

// RemoteBaseURL is the base URL of the remote Swagger Petstore API.
var RemoteBaseURL = define.Config{ // MARKER: RemoteBaseURL
	Value:      string(""),
	Default:    "https://petstore3.swagger.io/api/v3",
	Validation: "url",
}

// BearerToken is the OAuth2 bearer token presented to the remote Swagger Petstore API.
var BearerToken = define.Config{ // MARKER: BearerToken
	Value:  string(""),
	Secret: true,
}

// AddPet adds a new pet to the store.
var AddPet = define.Function{ // MARKER: AddPet
	Host: Hostname, Method: "POST", Route: "/pet",
	In: AddPetIn{}, Out: AddPetOut{},
}

// AddPetIn are the input arguments of AddPet.
type AddPetIn struct { // MARKER: AddPet
	HTTPRequestBody *Pet `json:"-"`
}

// AddPetOut are the output arguments of AddPet.
type AddPetOut struct { // MARKER: AddPet
	HTTPResponseBody *Pet `json:"-"`
	HTTPStatusCode   int  `json:"-"`
}

// GetPetById returns a single pet.
var GetPetById = define.Function{ // MARKER: GetPetById
	Host: Hostname, Method: "GET", Route: "/pet/{petId}",
	In: GetPetByIdIn{}, Out: GetPetByIdOut{},
}

// GetPetByIdIn are the input arguments of GetPetById.
type GetPetByIdIn struct { // MARKER: GetPetById
	PetId int64 `json:"petId,omitzero" jsonschema_description:"petId is the ID of the pet to return"`
}

// GetPetByIdOut are the output arguments of GetPetById.
type GetPetByIdOut struct { // MARKER: GetPetById
	HTTPResponseBody *Pet `json:"-"`
	HTTPStatusCode   int  `json:"-"`
}

// UploadFile uploads an image of the pet.
var UploadFile = define.Web{ // MARKER: UploadFile
	Host: Hostname, Method: "POST", Route: "/pet/{petId}/uploadImage",
}
