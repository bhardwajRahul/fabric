# petstore.example

This microservice is a regression fixture for the import-openapi-microservice pipeline. It is a faithful
one-for-one delegator generated from the public Swagger Petstore v3 OpenAPI document
(`https://petstore3.swagger.io/api/v3/openapi.json`), but it imports only a curated 3-endpoint subset of
that API rather than the full surface. The subset is deliberately small and chosen to exercise both code
paths of the importer: function endpoints (JSON request/response, path-arg input) and web endpoints
(raw binary body relay).

Unlike sibling `examples/` microservices, this one is intentionally not wired into the framework's
`main/main.go`. It exists purely as a generated artifact for validating the import pipeline and its
housekeeping (genservice, gofmt). The full upstream Petstore API has 19 endpoints; only the
three below were imported. The durable record of the remote API is `openapispecs.json`, which is tracked
in source control so additional endpoints can be imported later without re-fetching.

## Imported Endpoints

- AddPet - POST /pet - function: JSON Pet request body, JSON Pet response.
- GetPetById - GET /pet/{petId} - function: int64 path argument, JSON Pet response.
- UploadFile - POST /pet/{petId}/uploadImage - web: binary octet-stream upload, JSON ApiResponse relay.
