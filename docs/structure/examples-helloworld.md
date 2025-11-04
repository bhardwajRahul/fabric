# Package `examples/helloworld`

The `helloworld.example` microservice demonstrates the classic minimalist example.

http://localhost:8080/helloworld.example/hello-world simply prints `Hello, World!`.

The code looks rather daunting but practically all of it is code generated. The manually-coded pieces are:

The definition of the microservice and its single endpoint `HelloWorld` in `service.yaml`:

```yaml
general:
  host: helloworld.example
  description: The HelloWorld microservice demonstrates the classic minimalist example.

webs:
  - signature: HelloWorld()
    description: HelloWorld prints the classic greeting.
```

The implementation of the `HelloWorld` endpoint in `service.go`:

```go
w.Write([]byte("Hello, World!"))
return nil
```

A test of `TestHelloworld_HelloWorld` in `service_test.go`:

```go
res, err := client.HelloWorld(ctx, "")
if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
	body, err := io.ReadAll(res.Body)
	assert.Expect(body, []string("Hello, World!"), err, nil)
}
```

And finally, the addition of the microservice to the app in `main/main.go`.

```go
app.Add(
	helloworld.NewService(),
)
```
