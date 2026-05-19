package petstoreapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "petstore.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	AddPet     = Def{Method: "POST", Route: "/pet"}                     // MARKER: AddPet
	GetPetById = Def{Method: "GET", Route: "/pet/{petId}"}              // MARKER: GetPetById
	UploadFile = Def{Method: "POST", Route: "/pet/{petId}/uploadImage"} // MARKER: UploadFile
)

// AddPetIn are the input arguments of AddPet.
type AddPetIn struct { // MARKER: AddPet
	HTTPRequestBody *Pet `json:"-"`
}

// AddPetOut are the output arguments of AddPet.
type AddPetOut struct { // MARKER: AddPet
	HTTPResponseBody *Pet `json:"-"`
	HTTPStatusCode   int  `json:"-"`
}

// GetPetByIdIn are the input arguments of GetPetById.
type GetPetByIdIn struct { // MARKER: GetPetById
	PetId int64 `json:"petId,omitzero"`
}

// GetPetByIdOut are the output arguments of GetPetById.
type GetPetByIdOut struct { // MARKER: GetPetById
	HTTPResponseBody *Pet `json:"-"`
	HTTPStatusCode   int  `json:"-"`
}
