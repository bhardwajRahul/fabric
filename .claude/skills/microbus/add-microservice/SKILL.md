---
name: Adding a new microservice
description: Creates and initializes a new microservice. Use when explicitly asked by the user to create a new microservice.
---

## Workflow

Copy this checklist and track your progress:

```
Creating a new microservice:
- [ ] Step 1: Create a directory for the new microservice
- [ ] Step 2: Create doc.go with code generation directive
- [ ] Step 3: Generate service.yaml
- [ ] Step 4: Update service.yaml
- [ ] Step 5: Generate the microservice file structure
- [ ] Step 6: Propose microservice features
```

#### Step 1: Create a directory for the new microservice

Each microservice must be placed in a separate directory. Create a new directory for the new microservice.
Use only lowercase letters `a` through `z` for the name of the directory.

In smaller projects, place the new directory under the root directory of the project.

```bash
mkdir -p myservice
cd myservice
```

In larger projects, consider using a nested directory structure to group similar microservices together.

```bash
mkdir -p mydomain/myservice
cd mydomain/myservice
```

#### Step 2: Create `doc.go` with the code generation directive

Create `doc.go` with the `go:generate` directive to trigger the code generator. Name the package the same as the directory.

```go
package myservice

//go:generate go run github.com/microbus-io/fabric/codegen

```

#### Step 3: Generate `service.yaml`

Run `go generate` to generate `service.yaml`.

**Important**: Do not create `service.yaml` from scratch. Always let the code generator create it.

#### Step 4: Update `service.yaml`

The `service.yaml` file is the blueprint of the microservice. Update the `general` section as needed:
- The `host` defines the host name under which this microservice will be addressable. It must be unique across the application. Use reverse domain notation, e.g. `myservice.myproject.mycompany`.
- The `description` should explain what this microservice is about.
- Set `integrationTests` to `false` if instructed to skip integration tests, or if instructed to be "quick".

```yaml
general:
  host: myservice.myproject.mycompany
  description: My microservice does X, Y, and Z
  integrationTests: true
```

**Important**: Do not fill in any other sections of `service.yaml` unless explicitly asked to do so by the user.

#### Step 5: Generate the microservice file structure

Run `go generate` to generate the file structure of the microservice.

#### Step 6: Propose microservice features

Skip this step if instructed to be "quick".

If you have ideas of features to add to the microservice, prepare a proposal categorizing them by type:
- Configuration properties
- Functional endpoints (RPCs)
- Outbound events
- Inbound event sinks
- Web handler endpoints
- Tickers (recurring operations)
- Metrics

Save the proposal in `AGENTS.md`, then show it to the user and seek additional instructions.
Do not implement any of the proposals without explicit approval from the user.
