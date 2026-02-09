# HTTP Magic Arguments

Functional endpoints support three specially named arguments to allow finer control over the marshaling to and unmarhsaling from the underlying HTTP protocol.

- An input argument named `httpRequestBody` receives the unmarshaled request body (JSON or URL encoded)
- An output argument `httpResponseBody` is marshaled as the response (JSON)
- An output argument `httpStatusCode` of type `int` sets the response's status code

These arguments are often required when implementing a [RESTful API](../tech/rpc-vs-rest.md).

### `httpRequestBody`

By default, the body of a request to a functional endpoint is a JSON object that contains a named field for each of the function's arguments. For an endpoint such as `Create(p Person) (id int)`, the `Person` is nested under the argument name `p` in the request body.

```http
POST /persons HTTP/1.1
Content-Type: application/json

{
    "p": {
        "first": "Harry",
        "last": "Potter",
        "muggle": false
    }
}
```

Often it is desirable to read the entire request directly into the object and avoid the extra nesting under the argument name `p`. This can be achieved by naming the input argument `httpRequestBody`, that is `Create(httpRequestBody Person) (id int)`. The expected payload is now this:

```http
POST /persons HTTP/1.1
Content-Type: application/json

{
    "first": "Harry",
    "last": "Potter",
    "muggle": false
}
```

Because the argument `httpRequestBody` takes over the entire request body, no additional arguments may be posted in the request body. Any other input arguments are unmarshaled from either the path or the query of the request. This limits their size and is best saved for simple types.

### `httpResponseBody`

`httpResponseBody` operates the same way but for output arguments. `Load(id int) (httpResponseBody Person)` produces the response:

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "first": "Harry",
    "last": "Potter",
    "muggle": false
}
```

Because the argument `httpResponseBody` takes over the entire response body, no additional arguments may be returned, except for `httpStatusCode`.

### `httpStatusCode`

`httpStatusCode` controls the HTTP status code returned by the function. For example, we might want the `Create` method discussed earlier to return HTTP status `201 Created` instead of the default `200 OK`. Making its signature `Create(httpRequestBody Person) (id int, httpStatusCode int)` enables that scenario.

The implementation may look similar to the following:

```go
func (svc *Service) Create(ctx context.Context, httpRequestBody Person) (id int, httpStatusCode int, err error) {
    id, err := svc.database.createPerson(ctx, httpRequestBody)
    if err != nil {
        return 0, http.StatusInternalServerError, errors.Trace(err)
    }
    return id, http.StatusCreated, nil
}
```
