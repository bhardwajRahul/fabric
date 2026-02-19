---
name: Take the Agent-Guided Tour
description: Runs the agent-guided tour of Microbus using examples. Use when the user asks to take the tour.
---

## Workflow

Copy this checklist and track your progress:

```
Take the agent-guided tour:
- [ ] Step 1: Download examples
- [ ] Step 2: Extend main app
- [ ] Step 3: Docker Compose
- [ ] Step 4: Run the example app
- [ ] Step 5: Calculator
- [ ] Step 6: Hello
- [ ] Step 7: Messaging
- [ ] Step 8: Browser
- [ ] Step 9: Directory
- [ ] Step 10: Login
- [ ] Step 11: Telemetry
- [ ] Step 12: Stop example app
- [ ] Step 13: Stop Docker containers
- [ ] Step 14: What's next
```

#### Step 1: Download Examples

Download the latest examples from Github.

```shell
git clone --depth 1 https://github.com/microbus-io/fabric temp-clone
rm -rf examples
cp -r temp-clone/examples .
rm -rf temp-clone  
```

The example files reference `github.com/microbus-io/fabric/examples/...` internally. Replace all those references with the local module path.

#### Step 2: Extend Main App

Edit `main/main.go` and add the following block to the main `app` after the block of the core microservices.

```go
app.Add(
    // Example microservices
    helloworld.NewService(),
    hello.NewService(),
    messaging.NewService(),
    messaging.NewService(),
    messaging.NewService(),
    calculator.NewService(),
    eventsource.NewService(),
    eventsink.NewService(),
    directory.NewService(),
    browser.NewService(),
    login.NewService(),
)
```

Add the appropriate imports too, again, replace all references to `github.com/microbus-io/fabric/examples/...` with the local module path.

```go
import (
	"github.com/microbus-io/fabric/examples/browser"
	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/directory"
	"github.com/microbus-io/fabric/examples/eventsink"
	"github.com/microbus-io/fabric/examples/eventsource"
	"github.com/microbus-io/fabric/examples/hello"
	"github.com/microbus-io/fabric/examples/helloworld"
	"github.com/microbus-io/fabric/examples/login"
	"github.com/microbus-io/fabric/examples/messaging"
    "github.com/microbus-io/fabric/coreservices/httpingress/middleware"
)
```

Add the `LoginExample401Redirect` middleware to the HTTP ingress proxy using its `Init` method.

```go
httpingress.NewService().Init(func(svc *httpingress.Service) (err error) {
    svc.Middleware().Append("LoginExample401Redirect",
        middleware.OnRoute(
            func(path string) bool {
                return strings.HasPrefix(path, "/"+login.Hostname+"/")
            },
            middleware.ErrorPageRedirect(http.StatusUnauthorized, "/"+login.Hostname+"/login"),
        ),
    )
	return nil
}),
```

#### Step 3: Docker Compose

Ask the user if they'd like to install NATS and the LGTM stack using Docker Compose.

- [NATS](https://nats.io) is not required for the tour, but it is worthwhile learning
- The [Grafana LGTM](https://grafana.com/blog/2024/03/13/an-opentelemetry-backend-in-a-docker-image-introducing-grafana/otel-lgtm/) stack is also optional and only required for one step of the tour

If the answer is yes, do so using the following command.

```shell
docker compose -f setup/microbus.yaml -p microbus up -d
```

**CRITICAL**: Stop the Docker containers when the user ends the tour or exists the session.

```shell
docker compose -f setup/microbus.yaml -p microbus down
```

#### Step 4: Run the Example App

Change into the `main` directory and run the example app in the backgroun.

```shell
cd main
go run main.go
```

**CRITICAL**: Interrupt or kill the example app when the user ends the tour or exists the session.

#### Step 5: Calculator

The `calculator.example` microservice demonstrates basic functional endpoints (RPCs) that perform mathematical operations. It shows how to define typed request/response functions, handle input validation errors, and work with custom types like points.

Explain the microservice to the user and present the following links for them to experiment with in their browser:

- http://localhost:8080/calculator.example/arithmetic?x=5&op=*&y=8 — computes 5 * 8
- http://localhost:8080/calculator.example/square?x=5 — computes 5 squared
- http://localhost:8080/calculator.example/square?x=not-a-number — demonstrates input validation error handling
- http://localhost:8080/calculator.example/distance?p1.x=0&p1.y=0&p2.x=3&p2.y=4 — calculates distance between two points using a custom Point type

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 6: Hello

The `hello.example` microservice demonstrates a variety of framework capabilities including configuration properties, tickers, embedded static resources, and inter-service communication. It calls into `calculator.example` to show how microservices interact with each other, and multicasts a ping to discover all running microservices.

Explain the microservice to the user and present the following links for them to experiment with in their browser:

- http://localhost:8080/hello.example/echo — echoes back the raw HTTP request in wire format
- http://localhost:8080/hello.example/ping — multicasts a ping to discover all running microservices
- http://localhost:8080/hello.example/hello?name=Bella — greets the user using a configurable greeting
- http://localhost:8080/hello.example/calculator — renders a calculator UI that calls the calculator microservice
- http://localhost:8080/hello.example/bus.png — serves an embedded static image resource

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 7: Messaging

The `messaging.example` microservice demonstrates service-to-service communication patterns — unicast (load-balanced), multicast (all peers respond), and direct addressing — as well as the distributed cache for sharing state across instances.

Explain the microservice to the user and present the following links for them to experiment with in their browser:

- http://localhost:8080/messaging.example/home — demonstrates unicast, multicast, and direct addressing patterns
- http://localhost:8080/messaging.example/cache-store?key=foo&value=bar — stores a key-value pair in the distributed cache
- http://localhost:8080/messaging.example/cache-load?key=foo — retrieves a value from the distributed cache by key

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 8: Browser

The `browser.example` microservice demonstrates how to make outbound HTTP requests using the HTTP egress proxy. It provides a simple web UI with an address bar that fetches and displays the HTML source of any URL.

Explain the microservice to the user and present the following link for them to experiment with in their browser:

- http://localhost:8080/browser.example/browse?url=example.com — fetches and displays the HTML source of example.com

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 9: Directory

The `directory.example` microservice demonstrates a RESTful CRUD API backed by a database. It includes a web UI for testing the endpoints. If a SQL database is not configured, it falls back to an in-memory store.

Explain the microservice to the user and present the following links for them to experiment with in their browser:

- http://localhost:8080/directory.example/web-ui?method=POST&path=/persons&body=%7B%22firstName%22:%22Harry%22,%22lastName%22:%22Potter%22,%22email%22:%22hp@hogwarts.edu%22%7D — opens the web UI pre-filled to create a person record (push submit)
- http://localhost:8080/directory.example/persons — lists all person keys in the directory
- http://localhost:8080/directory.example/persons/key/1 — loads the person record with key 1
- http://localhost:8080/directory.example/web-ui?method=DELETE&path=/persons/key/1 — opens the web UI pre-filled to delete person 1 (push submit)

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 10: Login

The `login.example` microservice demonstrates authentication and role-based access control using JWT tokens. It includes a login form, protected pages with different role requirements, and cookie-based session management. Try logging in with `admin@example.com`, `manager@example.com`, or `user@example.com` (any password works).

Explain the microservice to the user and present the following link for them to experiment with in their browser:

- http://localhost:8080/login.example/welcome

Present the user with these options:
- "Next step" - Proceed to the next step
- "Explore more" - Prepare and show an overview of the features of the microservice. Suggest to the user they can see the code of any individual feature of the microservice and offer them the opportunity to ask questions. If asked to see the code, be sure to display the full implementation code of the feature, not just its signature.
- "End the tour" - Skip to step 12

#### Step 11: Telemetry

Skip this step if the user elected not to install the LGTM stack with Docker.

Explain to the user that they can view the telemetry collected by Grafana at http://localhost:3000. Metrics and traces are visualized in [dashboards](http://localhost:3000/dashboards) and can also be viewed in the [metrics drill-down app](http://localhost:3000/a/grafana-metricsdrilldown-app) and the [traces drill-down app](http://localhost:3000/a/grafana-exploretraces-app).

Present the user with these options:
- "End the tour" - Proceed to the next step

#### Step 12: Stop Example App

Interrupt or kill the example app that was spun up earlier.

#### Step 13: Stop Docker Containers

Skip this step if the user elected not to install NATS and LGTM with Docker in step 3.

Stop the Docker containers that were started earlier.

```shell
docker compose -f setup/microbus.yaml -p microbus down
```

#### Step 14: What's Next

Tell the user this concludes the tour and suggest to them that they try creating their own microservice by prompting the agent. Offer a few example prompts to get them started:

1. "Create a microservice that converts between Fahrenheit and Celsius"
2. "Create a microservice that shortens URLs and redirects short links back to the original"
3. "Create a microservice that returns a random quote from a list stored in a resource file"
4. "Create a microservice that accepts a Markdown body and returns rendered HTML"
