# Creating a Microservice

The [code generator](../blocks/codegen.md) is the primary tool in `Microbus` for working with microservices, including the creation of new ones. You can interact with the code generator directly, or instruct a [coding agent](../blocks/coding-agents.md) to do it for you.

## Using the Code Generator

#### Step 1: Create a Directory

Create a new directory for the new microservice. If you expect the solution to have a large number of microservices, you might want to create a nested structure. Use only the lowercase letters `a` through `z` for the name of the directory.

```shell
mkdir -p mydomain/myservice
cd mydomain/myservice
```

#### Step 2: Initialize the Code Generator

Create `doc.go` with the `go:generate` directive that will run the code generator. Set the name of the package to be the same as the name of the directory.


```go
package myservice

//go:generate go run github.com/microbus-io/fabric/codegen

```

#### Step 3: Generate File Structure

Within the directory of the microservice, run `go generate` to generate the file structure of the microservice, including `service.yaml`.

```shell
go generate
```

#### Step 4: Declare the Functionality

Fill in the [features of the microservice](../tech/service-yaml.md) in `service.yaml`, and `go generate` again to generate the [skeleton code](../blocks/skeleton-code.md) for the new microservice and its [client stubs](../blocks/client-stubs.md).

#### Step 5: Implement the Business Logic

Implement the functionality of the microservice in `service.go` and [test](../blocks/integration-testing.md) it in `service_test.go`.

Run `go generate` a final time to update the version number of the microservice.

#### Step 6: Add to the Application

Include the new microservice in the application in `main.go` if not already done so by the code generator.

```go
app.Add(
    // Add solution microservices here
    myservice.NewService(),
)
```

## Using a Coding Agent

#### Step 1: Initialize the Coding Agent

Start your coding agent, e.g.:

```shell
claude
```

#### Step 2: Create a New Microservice

Use a prompt similar to the following to create a new microservice.

```
Create a new "stripe" microservice that will process credit card payments via Stripe. Place it under the @financial directory. Set its hostname to "stripe.financial.ex".
```

#### Step 3: Implement the Business Logic

Use prompts similar to the following to add one feature at a time to the microservice:

```
Add a config property for the Stripe API key.
```

```
Create a functional endpoint "CreateIntent" that accepts a credit card number, email, and a dollar amount as arguments, and calls the Stripe API to create an intent. If successful, it should return the transaction ID.
```
