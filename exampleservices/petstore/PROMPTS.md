## Create the petstore delegating microservice

Create a new Microbus microservice that delegates one-for-one to the public Swagger Petstore v3 API
(`https://petstore3.swagger.io/api/v3`). The microservice represents that remote API inside the mesh.

- Directory: `exampleservices/petstore`; Go package `petstore`; API package `petstoreapi`; module path
  `github.com/microbus-io/fabric/exampleservices/petstore`.
- Hostname: `petstore.example`.
- One-line description: `Petstore delegates to the Swagger Petstore API.`
- The OpenAPI document declares a relative server (`/api/v3`), so the specs were generated with base URL
  `https://petstore3.swagger.io/api/v3`. The remote document is stored in `openapispecs.json`.

This microservice is a regression fixture for the OpenAPI import pipeline. It imports only a curated
subset of the upstream API, chosen to cover both importer branches:

- AddPet - POST /pet - function with a JSON Pet request body and JSON Pet response.
- GetPetById - GET /pet/{petId} - function with an int64 path argument and JSON Pet response.
- UploadFile - POST /pet/{petId}/uploadImage - web handler relaying a binary octet-stream upload and
  returning a JSON ApiResponse.

Generate only the complex types these three endpoints reference: Pet (and its nested Category and Tag)
and ApiResponse.

The upstream API resolves to OAuth2, so add a secret `BearerToken` config plus a `RemoteBaseURL` config
(default `https://petstore3.swagger.io/api/v3`, validation `url`). The shared `authenticate` helper sets
a bearer Authorization header.

As a fixture inside the framework repo's `exampleservices/`, this microservice is intentionally not added to
`main/main.go` (sibling examples are standalone). Validate with `go vet ./exampleservices/petstore/...` and
`go build ./exampleservices/petstore/...`.
