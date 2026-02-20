---
name: Adding a new microservice
description: Creates and initializes a new microservice. Use when explicitly asked by the user to create a new microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Creating a new microservice:
- [ ] Step 1: Determine the name and description
- [ ] Step 2: Create a directory structure
- [ ] Step 3: Prepare coding agent files
- [ ] Step 4: Prepare client.go
- [ ] Step 5: Prepare embed.go
- [ ] Step 6: Prepare service.go
- [ ] Step 7: Prepare intermediate.go
- [ ] Step 8: Prepare mock.go
- [ ] Step 9: Prepare service_test.go
- [ ] Step 10: Prepare manifest.yaml
- [ ] Step 11: Add to main app
- [ ] Step 12: Propose features
```

#### Step 1: Determine the Name and Description

Determine the name of the microservice. Use only letters `a` through `z` and `A` through `Z`.
The templates in this skill use `myservice`, `myserviceapi`, `MyService` and `TestMyService` as placeholders that are based on the name of the microservice.

Determine a Go-style description for the microserice in the form `MyService is X`.

#### Step 2: Create a Directory Structure

Each microservice must be placed in a separate directory. Create a new directory for the new microservice.
Use the name of the microservice in lowercase as the name of the new directory name.

In smaller projects, place the new directory under the root directory of the project.
In larger projects, consider using a nested directory structure to group similar microservices together.

```shell
mkdir -p myservice
```

Create two subdirectories.

- The first should be a concatenation of the microservice directory and the suffix `api`
- The second subdirectory should be named `resources`

```shell
mkdir -p myservice/myserviceapi
mkdir -p myservice/resources
```

The directory structure should look like this.

```
myproject/
└── myservice/
    ├── myserviceapi/
    └── resources/
```

**IMPORTANT**: File names in the following steps are relative to the new `myservice` directory, unless indicated otherwise.

File preparation steps can be performed in parallel.

#### Step 3: Prepare Coding Agent Files

Create `AGENTS.md` with the following content verbatim.

```md
**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.
```

Create `CLAUDE.md` with the following content verbatim.

```md
**CRITICAL**: Read `AGENTS.md` immediately.
```

Create `PROMPTS.md` with the prompt to create this microservice. Save the prompt under a `## Title`.

```md
## Prompt title

Prompt comes here...
```

#### Step 4: Prepare `client.go`

Create `myserviceapi/client.go` with the content of the template `client.go` located in the directory of this skill.

- The `Hostname` constant holds the hostname in which this microservice will be addressable. It must be unique across the application. Use reverse domain notation based on the module path, up to and including the name of the project. For example, if the module path is `github.com/my-company/myproject/some/path/myservice`, set the hostname to `myservice.path.some.myproject`. Only letters `a-z`, numbers `0-9`, hyphens `-` and the dot `.` separator are allowed in the hostname

#### Step 5: Prepare `embed.go`

Create `resources/embed.go` with the following content verbatim.

```go
package resources

import "embed"

//go:embed *
var FS embed.FS
```

#### Step 6: Prepare `service.go`

Create `service.go` with the content of the template `service.go` located in the directory of this skill.

- Match the package name to the directory name
- Set the comment of the type definition of `Service` to describe this particular microservice. The provided value is a template. Do not copy it verbatim

#### Step 7: Prepare `intermediate.go`

Create `intermediate.go` with the content of the template `intermediate.go` located in the directory of this skill.

- Set the description of the microservice in `svc.SetDescription`

#### Step 8: Prepare `mock.go`

Create `mock.go` with the content of the template `mock.go` located in the directory of this skill.

Note that it is the intention of `Mock` to shadow the functions of `Service`.

#### Step 9: Prepare `service_test.go`

Create `service_test.go` with the content of the template `service_test.go` located in the directory of this skill.

- Match the package name to the directory name
- The imports are pre-declared for convenience now, before adding test code later
- Be sure to include `TestMyService_OpenAPI` even though initially it is a no op

#### Step 10: Prepare `manifest.yaml`

Look in `go.mod` and identify the current version of the `github.com/microbus-io/fabric` dependency. This is the framework version. Set it in `manifest.yaml` next. When working inside the fabric repository itself, there is no such dependency. Use instead the latest version you can find in any other `manifest.yaml` in the project.

Create `manifest.yaml` with the following content.

```yaml
general:
  hostname: myservice.myproject.mycompany
  description: MyService does X.
  package: github.com/mycompany/myproject/myservice
  frameworkVersion: 1.23.0 
```

#### Step 11: Add to Main App

Find `main/main.go` relative to the project root. Add the new microservice to the app in the `main` function. Add the appropriate import statement at the top of the file.

```go
import (
	// ...
	"github.com/mycompany/myproject/myservice"
)

func main() {
	// ...
	app.Add(
		// HINT: Add solution microservices here
		myservice.NewService(),
	)
	// ...
}
```

#### Step 12: Propose Features

Ask the user if they'd like you to propose a design for the microservice. If the user declines, skip the remainder of this step.

Use the context provided by the user and propose a set of features for this microservice for each of the following categories. You may engage the user and ask for additional information if required.

- **Configuration properties** — runtime settings (strings, durations, booleans, etc.) read via the connector
- **Functional endpoints (RPCs)** — typed request/response functions with input/output structs, marshaling, and client stubs
- **Web handler endpoints** — raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Outbound events** — messages this microservice fires for others to consume
- **Inbound event sinks** — handlers that react to events emitted by other microservices
- **Tickers** — recurring operations on a schedule
- **Metrics** — counters, gauges, and histograms for observability

Save the proposed design to `DESIGN.md`, then show it to the user. Ask the user if they'd like you to implement any of the features. Do not implement without explicit approval from the user.
