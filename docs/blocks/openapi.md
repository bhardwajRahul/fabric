# OpenAPI

The [OpenAPI](https://www.openapis.org) specification is a formal standard for describing HTTP APIs in a YAML or JSON document. It is the world's most widely used API description standard.

Microbus leverages the knowledge it has about the structure of a microservice to automatically generate an OpenAPI document for each of its public web and functional endpoints. A separate OpenAPI document is created for each port of each microservice. Here's an (abbreviated) example of an OpenAPI document generated for the `:443` endpoints of the [calculator microservice](../structure/examples-calculator.md):

```yaml
openapi: 3.0.0
info:
    title: calculator.example
    description: The Calculator microservice performs simple mathematical operations.
    version: "141"
servers:
    - url: http://localhost:8080/
paths:
    /calculator.example:443/arithmetic:
        post:
            summary: Arithmetic(x int, op string, y int) (result int)
            description: Arithmetic perform an arithmetic operation between two integers x and y given an operator op.
            requestBody:
                required: true
                content:
                    application/json:
                        schema:
                            $ref: '#/components/schemas/Arithmetic_in'
            responses:
                "200":
                    description: OK
                    content:
                        application/json:
                            schema:
                                $ref: '#/components/schemas/Arithmetic_out'
components:
    schemas:
        Arithmetic_in:
            type: object
            properties:
                op:
                    type: string
                x:
                    type: integer
                    format: int64
                "y":
                    type: integer
                    format: int64
        Arithmetic_out:
            type: object
            properties:
                result:
                    type: integer
                    format: int64
```

Every microservice publishes an OpenAPI document describing its endpoints. The document is filtered by what the caller is authorized to see - operations whose claims the caller can't satisfy are simply absent.

The [OpenAPI portal](../structure/coreservices-openapiportal.md) brings these per-service documents together. It serves an aggregated OpenAPI document covering every microservice on the bus, and a human-friendly browser for exploring them.

[Swagger](https://swagger.io) is a set of popular tools for working with APIs in general and OpenAPI in particular. The [OpenAPI editor](https://editor-next.swagger.io) is an especially useful one that allows editing and exploring OpenAPI documents online.
