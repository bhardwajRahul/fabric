---
name: add-microservice
description: TRIGGER when user asks to create, scaffold, or initialize a new microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: A microservice's hand-written source is `myserviceapi/definition.go` (the API spec) and `service.go` (the handler logic), plus the resource and test files. Run `cmd/genservice` to produce the rest.

## Workflow

Copy this checklist and track your progress:

```
Creating a new microservice:
- [ ] Step 1: Determine the name and description
- [ ] Step 2: Create a directory structure
- [ ] Step 3: Prepare coding agent files
- [ ] Step 4: Prepare definition.go
- [ ] Step 5: Prepare embed.go
- [ ] Step 6: Prepare service.go
- [ ] Step 7: Prepare service_test.go
- [ ] Step 8: Generate the boilerplate
- [ ] Step 9: Add to main app
- [ ] Step 10: Propose features
```

#### Step 1: Determine the Name and Description

Determine the name of the microservice. Use only letters `a` through `z` and `A` through `Z`.
The templates in this skill use `myservice`, `myserviceapi`, `MyService` and `TestMyService` as placeholders that are based on the name of the microservice.

Determine a Go-style description for the microservice in the form `MyService is X`.

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

The hand-written file preparation steps (4-7) can be performed in parallel; the generator step (8) must follow them.

#### Step 3: Prepare Coding Agent Files

Create `CLAUDE.md` with the hostname as an H1 heading:

```md
# my.service.hostname

**CRITICAL**: This directory is a Microbus microservice. Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.
```

Create `PROMPTS.md` with the prompt used to create this microservice. Rephrase the language to include context that was not made explicit in the original prompt. The intent is to maintain an auditable trail of the prompts, and to allow a future agent to reproduce the functionality of the microservice from these prompts.

Save the prompt as follows.

```md
## Prompt title

Prompt comes here...
```

#### Step 4: Prepare `definition.go`

Create `myserviceapi/definition.go` with the content of the template `definition.go` located in the directory of this skill. This is the single source of truth for the microservice's API: the add-feature skills append `define.*` vars (and their In/Out structs) here, and `cmd/genservice` projects everything else from it.

- The `Hostname` constant holds the hostname in which this microservice will be addressable. It must be unique across the application. Use reverse domain notation based on the module path, up to and including the name of the project. For example, if the module path is `github.com/mycompany/myproject/some/path/myservice`, set the hostname to `myservice.path.some.myproject`. Only letters `a-z`, numbers `0-9`, hyphens `-` and the dot `.` separator are allowed in the hostname
- `Name` is the decorative PascalCase name of the microservice (it cannot be derived from the lowercase directory)
- `Version` is a generation counter bumped on each regeneration (not a semantic version); leave it at `1` for a new microservice
- `Description` is the Go-style description from Step 1
- The `define` import is pre-added with a `var _ = define.None` guard so the file compiles before any feature exists and the add-feature skills can append a `define.*` var without managing imports. Leave the guard in place

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

#### Step 7: Prepare `service_test.go`

Create `service_test.go` with the content of the template `service_test.go` located in the directory of this skill.

- Match the package name to the directory name
- The imports are pre-declared for convenience now, before adding test code later

#### Step 8: Generate the Boilerplate

From the microservice's directory, run the generator. It reads `myserviceapi/definition.go` and writes `myserviceapi/client.go`, `intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml`.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then verify the microservice compiles with `go vet ./...` from the project root (not `go build`, which conflicts with the `main/` directory name).

#### Step 9: Add to Main App

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

#### Step 10: Propose Features

Ask the user if they'd like you to propose a design for the microservice. If the user declines, skip the remainder of this step.

Use the context provided by the user and propose a set of features for this microservice for each of the following categories. You may engage the user and ask for additional information if required.

- **Configuration properties** - runtime settings (strings, durations, booleans, etc.) read via the connector
- **Functional endpoints (RPCs)** - typed request/response functions with input/output structs, marshaling, and client stubs
- **Web handler endpoints** - raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Outbound events** - messages this microservice fires for others to consume
- **Inbound event sinks** - handlers that react to events emitted by other microservices
- **Tickers** - recurring operations on a schedule
- **Metrics** - counters, gauges, and histograms for observability

Append the proposed design to `CLAUDE.md` under the following heading, then show it to the user. Ask the user if they'd like you to implement any of the features. Do not implement without explicit approval from the user.

```md
## Agent Design Proposal

*This section is the original pre-development proposal. It reflects the intended design at the time of creation and may not match the current implementation.*
```
