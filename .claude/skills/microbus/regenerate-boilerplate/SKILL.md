---
name: regenerate-boilerplate
description: Use when a microservice's generated files are corrupted, outdated, or need to be rebuilt from scratch.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

The generated boilerplate is a pure projection of `myserviceapi/definition.go`. Rebuilding it is a single deterministic command - there is nothing to reconstruct by hand. The hand-written source (`definition.go`, sibling type files, `service.go`, `service_test.go`, `resources/embed.go`) is the source of truth and is never overwritten.

## Workflow

Copy this checklist and track your progress:

```
Regenerating boilerplate:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Regenerate the boilerplate
- [ ] Step 3: Vet and test
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

Confirm the hand-written source is intact: `myserviceapi/definition.go` (the spec), `service.go` (the handlers), and `resources/embed.go`. If `resources/embed.go` is missing, recreate it per the `add-microservice` skill. If `definition.go` itself is corrupted or missing, this skill cannot help - the boilerplate is derived from it, not the other way around.

#### Step 2: Regenerate the Boilerplate

From the microservice's directory, run the generator. It overwrites `myserviceapi/client.go`, `intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml` from `definition.go`, regardless of their prior state.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

#### Step 3: Vet and Test

Run `go vet ./...` from the project root to verify the microservice compiles, then `go test ./...` (or the microservice's own package) to verify its tests pass. Fix any issues before finishing.
