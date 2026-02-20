---
name: Regenerate Boilerplate
description: Regenerates the boilerplate files of a microservice from its manifest and service code. Use when boilerplate files are corrupted, outdated, or need to be rebuilt from scratch.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Regenerating boilerplate:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Read manifest.yaml and service.go
- [ ] Step 3: Read Version const from intermediate.go
- [ ] Step 4: Delete existing boilerplate files
- [ ] Step 5: Initialize boilerplate files
- [ ] Step 6: Regenerate configs
- [ ] Step 7: Regenerate functional endpoints
- [ ] Step 8: Regenerate web handler endpoints
- [ ] Step 9: Regenerate outbound events
- [ ] Step 10: Regenerate inbound event sinks
- [ ] Step 11: Regenerate tickers
- [ ] Step 12: Regenerate metrics
- [ ] Step 13: Verify unchanged files
- [ ] Step 14: Increment Version const
- [ ] Step 15: Build, vet and test
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Read `manifest.yaml` and `service.go`

Read `manifest.yaml` to identify all features of the microservice. Read `service.go` to understand the implementations. Read `service_test.go` to understand the tests. These files must not be modified.

#### Step 3: Read `Version` Const from `intermediate.go`

Before deleting the existing boilerplate, read `intermediate.go` and note the current value of the `Version` const (e.g. `Version = 270`). Remember this value for a later step.

#### Step 4: Delete Existing Boilerplate Files

Delete the following files.

- `myserviceapi/client.go`
- `resources/embed.go`
- `intermediate.go`
- `mock.go`

Do NOT delete `service.go`, `service_test.go`, `manifest.yaml`, `AGENTS.md`, `CLAUDE.md`, `PROMPTS.md`, or any other files.

#### Step 5: Initialize Boilerplate Files

Follow these steps from the `microbus/add-microservice` skill to recreate the boilerplate files from scratch:

- **Prepare `client.go`**: Use the hostname and package from `manifest.yaml`
- **Prepare `embed.go`**
- **Prepare `intermediate.go`**: Use the description from `manifest.yaml`
- **Prepare `mock.go`**

Do NOT follow the steps that create `service.go`, `service_test.go`, `manifest.yaml`, or the directory structure — these already exist.

#### Step 6: Regenerate Configs

For each config listed under `configs` in `manifest.yaml`, follow the `microbus/add-config` skill but ONLY these steps:

- **Extend the `ToDo` interface** (only if the config has `callback: true`)
- **Define the config**
- **Implement the getter and setter**
- **Wire up the config change dispatcher** (only if the config has `callback: true`)
- **Extend the mock** (only if the config has `callback: true`)

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 7: Regenerate Functional Endpoints

For each function listed under `functions` in `manifest.yaml`, follow the `microbus/add-function` skill but ONLY these steps:

- **Extend the `ToDo` interface**
- **Define the payload structs**
- **Extend the clients**
- **Define the marshaler function**
- **Bind the marshaler function to the microservice**
- **Expose the endpoint via OpenAPI**
- **Extend the mock**

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 8: Regenerate Web Handler Endpoints

For each web handler listed under `webs` in `manifest.yaml`, follow the `microbus/add-web` skill but ONLY these steps:

- **Extend the `ToDo` interface**
- **Extend the clients**
- **Bind the handler to the microservice**
- **Expose the endpoint via OpenAPI**
- **Extend the mock**

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 9: Regenerate Outbound Events

For each event listed under `outboundEvents` in `manifest.yaml`, follow the `microbus/add-outbound-event` skill but ONLY these steps:

- **Define the payload structs**
- **Extend the trigger and hook**

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 10: Regenerate Inbound Event Sinks

For each event listed under `inboundEvents` in `manifest.yaml`, follow the `microbus/add-inbound-event` skill but ONLY these steps:

- **Extend the `ToDo` interface**
- **Bind the inbound event sink to the microservice**
- **Extend the mock**

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 11: Regenerate Tickers

For each ticker listed under `tickers` in `manifest.yaml`, follow the `microbus/add-ticker` skill but ONLY these steps:

- **Extend the `ToDo` interface**
- **Bind the handler to the microservice**
- **Extend the mock**

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 12: Regenerate Metrics

For each metric listed under `metrics` in `manifest.yaml`, follow the `microbus/add-metric` skill but ONLY these steps:

- **Extend the `ToDo` interface** (only if the metric has `observable: true`)
- **Describe the metric**
- **Implement the recorders**
- **Observe with callback** (only if the metric has `observable: true`)
- **Extend the mock** (only if the metric has `observable: true`)

Skip the steps that affect `service.go` and `service_test.go`. Skip the housekeeping step.

#### Step 13: Verify Unchanged Files

Verify that `service.go`, `service_test.go`, and `manifest.yaml` have not been modified. If any of these files were changed, revert them immediately.

#### Step 14: Increment `Version` Const

In the regenerated `intermediate.go`, find the `Version` const and set its value to the value remembered in Step 3, plus 1. For example, if the old value was `Version = 270`, set it to `Version = 271`.

#### Step 15: Vet and Test

Run `go vet` on the microservice's package to verify it compiles without errors. Then run `go test` on the package to verify all tests pass. Fix any issues before finishing.
