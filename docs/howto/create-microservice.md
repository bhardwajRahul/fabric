# Creating a Microservice

### Step 1: Create a Directory

Create a new directory for the new microservice. If you expect the solution to have a large number of microservice, you might want to create a nested structure. Lowercase directory names are recommended.

```shell
mkdir mydomain/myservice
```

### Step 2: Initialize the Code Generator

Create `mydomain/myservice/doc.go` with the `go:generate` instruction that will run the code generator. It is recommended to echo the name of the directory for the name of the package.

```go
//go:generate go run github.com/microbus-io/fabric/codegen

package myservice
```

### Step 3: Generate `service.yaml`

From within the directory, run `go generate` to create an empty `service.yaml` template.

```shell
cd mydomain/myservice
go generate
```

### Step 4: Declare the Functionality

Fill in the [characteristics of the microservice](../tech/service-yaml.md) in `service.yaml`, and `go generate` again to generate the [skeleton code](../blocks/skeleton-code.md) for the new microservice and its [client stubs](../blocks/client-stubs.md).

### Step 5: Implement the Business Logic

Implement the functionality of the microservice in `service.go` and [test](../blocks/integration-testing.md) it in `service_test.go`.

Run `go generate` a final time to update the version number of the microservice.

### Step 6: Add to the Application

Include the new microservice in the application in `main.go` if not already done so by the code generator.

```go
app.Add(
    // Add solution microservices here
    myservice.NewService(),
)
```
