---
name: Adding a Web Handler Endpoint
description: Creates or modify a web handler endpoint of a microservice. Use when explicitly asked by the user to create or modify a web handler endpoint of a microservice.
---

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a web endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Define in service.yaml
- [ ] Step 3: Generate boilerplate code
- [ ] Step 4: Move implementation and test if renamed
- [ ] Step 5: Implement the business logic
- [ ] Step 6: Test the web handler
- [ ] Step 7: Document the microservice
- [ ] Step 8: Versioning
```

#### Step 1: Read local `AGENTS.md` file

Check for and read a local `AGENTS.md` file in that microservice's directory. The local `AGENTS.md` file contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Define in `service.yaml`

Define the web endpoint in the `web` array in the `service.yaml` of the microservice.
- The `signature` of the web endpoint must follow Go function syntax exactly. Do not include any input arguments nor any output arguments.
- The `description` should explain what the web endpoint is doing. It should start with the name of the web handler.
- A `method` restricts requests to a specific HTTP method such as `GET`, `POST`, `DELETE`, `PUT`, `PATCH`, `OPTIONS` or `HEAD`. The default `ANY` accepts all requests regardless of the method.

```yaml
webs:
  - signature: MyNewWebHandler()
    description: MyNewWebHandler does X, Y and Z.
    method: ANY
```

#### Step 3: Generate boilerplate code

If you've made changes to `service.yaml`, run `go generate` to generate the boilerplate code.

#### Step 4: Move implementation and test if renamed

If you made a change to the name of the web handler in the `signature` field, you need to move over its implementation in `service.go` from under the old name to the new name. Similarly, you'll need to move over the implementation of the tests in `service_test.go`. 

#### Step 5: Implement the business logic

Look for the web handler declaration in `service.go` and implement or adjust its logic appropriately.
Note that Microbus web handlers extend on the standard Go web handler signature by also returning an error.
You do not need to worry about printing the error and status code to the `http.ResponseWriter`.

```go
func (svc *Service) MyNewWebHandler(w http.ResponseWriter, r *http.Request) (err error) {
	// Implement logic here
	return err
}
```

#### Step 6: Test the web handler

Skip this step if integration tests were skipped for this microservice, or if instructed to be "quick".

Look for the integration test created in `service_test.go` for the web handler and implement or adjust it appropriately.
- Follow the pattern recommendation in the code
- Add downstream microservices or their mocks to the testing app

```go
func TestMyservice_MyNewWebHandler(t *testing.T) {
	// Implement testing here
}
```

#### Step 7: Document the microservice

Skip this step if instructed to be "quick".

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 8: Versioning

Run `go generate` to version the code.
