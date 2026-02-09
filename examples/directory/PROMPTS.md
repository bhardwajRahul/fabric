## Upgrade to v2

Upgrade the directory microservice from the v1 code-generated pattern (using `service.yaml`, `intermediate/` package, and `-gen.go` files) to the v2 agent-based pattern (using `intermediate.go`, `mock.go`, and `client.go` directly in the microservice package). The microservice exposes a RESTful CRUD API for personal records with Create, Load, Delete, Update, LoadByEmail, List functions, a WebUI web handler, and a SQL config property.
