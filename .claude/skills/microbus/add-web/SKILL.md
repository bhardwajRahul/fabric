---
name: Adding a Web Handler Endpoint
description: Creates or modify a web handler endpoint of a microservice. Use when explicitly asked by the user to create or modify a web handler endpoint of a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a web endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine the method and route
- [ ] Step 3: Determine a description
- [ ] Step 4: Determine required claims
- [ ] Step 5: Extend the ToDo interface
- [ ] Step 6: Extend the clients
- [ ] Step 7: Implement the logic
- [ ] Step 8: Bind the handler to the microservice
- [ ] Step 9: Expose the endpoint via OpenAPI
- [ ] Step 10: Extend the mock
- [ ] Step 11: Test the handler
- [ ] Step 12: Update manifest
- [ ] Step 13: Document the microservice
- [ ] Step 14: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. Use `ANY` to accept requests with any method.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-web`.

To set a port other than the default 443, prefix the route with the port, e.g. `:123/my-web`.

Encase path arguments with `{}` , e.g. `/section/{section}/page/{page...}`.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:1234/on-something`

#### Step 3: Determine a Description

Describe the endpoint starting with its name, in Go doc style: `MyWeb does X`. Embed this description in followup steps where appropriate.

#### Step 4: Determine the Required Claims

Determine if the endpoint should be restricted to authorized actors only. Compose a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied. For example: `roles=~"manager" && level>2`. Leave empty if the endpoint should be accessible by all.

#### Step 5: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyWeb(w http.ResponseWriter, r *http.Request) (err error) // MARKER: MyWeb
}
```

#### Step 6: Extend the Clients

Extend the clients in `myserviceapi/client.go`.

Define the route and URL of the endpoint at the top of the file in the respective `const` and `var` blocks. 

```go
// Endpoint routes.
const (
	// ...
	RouteOfMyWeb = `/my-web` // MARKER: MyWeb
)

// Endpoint URLs.
var (
	// ...
	URLOfMyWeb = httpx.JoinHostAndPath(Hostname, RouteOfMyWeb) // MARKER: MyWeb
)
```

Append the stubs at the bottom of the file.

- In the comments, replace `MyWeb does X` with the description of the endpoint
- If the method of the endpoint is anything other than `ANY`, omit the `method` argument and instead hardcode it as the argument of `pub.Method`
- If the method is `GET`, `HEAD`, `OPTIONS`, or `TRACE` which don't support a body, omit the `body` argument and the `pub.Body` option and adjust the comment appropriately

```go
/*
MyWeb does X.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) MyWeb(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: MyWeb
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfMyWeb)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
MyWeb does X.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) MyWeb(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: MyWeb
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfMyWeb)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}
```

#### Step 7: Implement the Logic

Implement the web handler in `service.go`:
- In the comments, replace `MyWeb does X` with the description of the endpoint
- Use `r.PathValue("argName")` to obtain the values of path arguments by name, if needed

```go
/*
MyWeb does X.
*/
func (svc *Service) MyWeb(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyWeb
	// Implement logic here...
	return nil
}
```

#### Step 8: Bind the Handler to the Microservice

Bind the web handler to the microservice in the `NewIntermediate` constructor in `intermediate.go`.

- The first two arguments to `svc.Subscribe` are the method and route of the endpoint
- The queue option indicate how requests are distributed among replicas of the microservice
  - `sub.DefaultQueue()`: requests are load balanced among peers and processed by only one. This is the default option and may be omitted
  - `sub.NoQueue()`: requests are processed by all subscribers
  - `sub.Queue(queueName)`: requests are load balanced among peers associated with this queue name. Subscribers associated with other queue names receive the requests separately based on their own queue option
- The `sub.RequiredClaims(requiredClaims)` option defines the authorization requirements of the endpoint. This option can be omitted to allow all requests

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.Subscribe("ANY", myserviceapi.RouteOfMyWeb, svc.MyWeb, sub.LoadBalanced(), sub.RequiredClaims(requiredClaims)) // MARKER: MyWeb
	// ...
}
```

#### Step 9: Expose the Endpoint via OpenAPI

Register the web handler endpoint in `doOpenAPI` in `intermediate.go`.

- For a web handler endpoint, the `Type` field should be set to `web`
- Set the simplified signature of the endpoint, with no arguments, in the `Summary` field
- In the `Description` field, replace `MyWeb does X` with the description of the endpoint
- Set the `RequiredClaims` boolean expression, if relevant to this endpoint. Otherwise, omit the field or leave it empty

```go
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	// ...
	endpoints := []*openapi.Endpoint{
		// ...
		{ // MARKER: MyWeb
			Type:          "web",
			Name:          "MyWeb",
			Method:        "ANY",
			Route:         myserviceapi.RouteOfMyWeb,
			Summary:       "MyWeb()",
			Description:   `MyWeb does X.`,
			RequiredClaims: ``,
		},
	}
	// ...
}
```

#### Step 10: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyWeb func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: MyWeb
}
```

Add the stubs to the `Mock`:

```go
// MockMyWeb sets up a mock handler for MyWeb.
func (svc *Mock) MockMyWeb(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: MyWeb
	svc.mockMyWeb = handler
	return svc
}

// MyWeb executes the mock handler.
func (svc *Mock) MyWeb(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyWeb
	if svc.mockMyWeb == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockMyWeb(w, r)
	return errors.Trace(err)
}
```

Add a test case in `TestMyService_Mock`.

```go
t.Run("my_web", func(t *testing.T) { // MARKER: MyWeb
	assert := testarossa.For(t)

	w := httpx.NewResponseRecorder()
	r := httpx.MustNewRequest("GET", "/", nil)

	err := mock.MyWeb(w, r)
	assert.Contains(err.Error(), "not implemented")
	mock.MockMyWeb(func(w http.ResponseWriter, r *http.Request) (err error) {
		w.WriteHeader(http.StatusOK)
		return nil
	})
	err = mock.MyWeb(w, r)
	assert.NoError(err)
})
```

#### Step 11: Test the Handler

Skip this step if instructed to be "quick" or to skip tests.

Append the following code block to the end of `service_test.go`.

- Do not remove comments with `HINT`s. They are there to guide you in the future.
- Insert test cases at the bottom of the test function using the recommended pattern.
- There is no need to set the `pub.Actor` option if the web endpoint does not require claims.

```go
func TestMyService_MyWeb(t *testing.T) { // MARKER: MyWeb
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			actor := jwt.MapClaims{}
			res, err := client.WithOptions(pub.Actor(actor)).MyWeb(ctx, "GET", "", payload)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.HTMLMatch(body, "DIV.class > DIV#id", "substring")
					assert.Contains(body, "substring")
				}
			}
		})
	*/
}
```

In `TestMyService_OpenAPI` in `service_test.go`, add the endpoint's port, if not already tested.

#### Step 12: Update Manifest

Update the `webs` and `downstream` sections of `manifest.yaml`.

#### Step 13: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 14: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.
