---
name: Adding a Web Handler Endpoint
description: Creates or modify a web handler endpoint of a microservice. Use when explicitly asked by the user to create or modify a web handler endpoint of a microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

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
- [ ] Step 12: Housekeeping
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

Describe the endpoint starting with its name, in Go doc style: `MyWeb does X`. Embed this description in followup steps wherever you see `MyWeb does X`.

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

Define the endpoint in the `var` block in `myserviceapi/client.go`, after the corresponding `HINT` comment.

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyWeb = Def{Method: "ANY", Route: "/my-web"} // MARKER: MyWeb
)
```

Append the following methods to `myserviceapi/client.go`.

- If the method of the endpoint is anything other than `ANY`, omit the `method` argument and instead hardcode it as the argument of `pub.Method`
- If the method is `GET`, `HEAD`, `OPTIONS`, or `TRACE` which don't support a body, omit the `body` argument and the `pub.Body` option and remove the lines about body serialization from the doc comment

```go
/*
MyWeb does X.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) MyWeb(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: MyWeb
	if method == "" {
		method = MyWeb.Method
	}
	if method == "ANY" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, MyWeb.Route)),
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
func (_c MulticastClient) MyWeb(ctx context.Context, method string, relativeURL string, body any) iter.Seq[*pub.Response] { // MARKER: MyWeb
	if method == "" {
		method = MyWeb.Method
	}
	if method == "ANY" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, MyWeb.Route)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}
```

#### Step 7: Implement the Logic

Implement the web handler in `service.go`:
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

Bind the web handler to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...
	svc.Subscribe(myserviceapi.MyWeb.Method, myserviceapi.MyWeb.Route, svc.MyWeb) // MARKER: MyWeb
	// ...
}
```

Add the following options to `svc.Subscribe` as needed:

- A queue option to control how requests are distributed among replicas of the microservice
  - `sub.DefaultQueue()`: requests are load balanced among peers and processed by only one. This is the default and may be omitted
  - `sub.NoQueue()`: requests are processed by all subscribers
  - `sub.Queue(queueName)`: requests are load balanced among peers associated with this queue name. Subscribers associated with other queue names receive the requests separately based on their own queue option
- `sub.RequiredClaims(requiredClaims)` to define the authorization requirements of the endpoint. Omit to allow all requests

#### Step 9: Expose the Endpoint via OpenAPI

Register the web handler endpoint in `doOpenAPI` in `intermediate.go`, after the corresponding `HINT` comment.

- For a web handler endpoint, the `Type` field should be set to `web`
- Set the simplified signature of the endpoint, with no arguments, in the `Summary` field
- Set the `RequiredClaims` boolean expression, if relevant to this endpoint. Otherwise, omit the field or leave it empty

```go
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	// ...
	endpoints := []*openapi.Endpoint{
		// ...
		{ // MARKER: MyWeb
			Type:          "web",
			Name:          "MyWeb",
			Method:        myserviceapi.MyWeb.Method,
			Route:         myserviceapi.MyWeb.Route,
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

Add the route of the web handler to the `routes` slice in `TestMyService_OpenAPI` in `service_test.go`.

```go
routes := []string{
	// HINT: Insert routes of functional and web endpoints here
	// ...
	myserviceapi.MyWeb.Route, // MARKER: MyWeb
}
```

Append the integration test to `service_test.go`.

```go
func TestMyService_MyWeb(t *testing.T) { // MARKER: MyWeb
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)
	_ = client

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

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

- You may omit the `pub.Actor` option if the functional endpoint does not require claims.

```go
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
```

Do not remove the `HINT` comments.

#### Step 12: Housekeeping

Follow the `microbus/housekeeping` skill.
