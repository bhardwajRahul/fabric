---
name: Modifying a Feature of a Microservice
description: Modifies an existing functional endpoint, web handler endpoint, event source, event sink, configuration property, ticker or metric of a microservice. Use when explicitly asked by the user to modify a feature of a microservice.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Modifying a feature of a microservice:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Identify the feature
- [ ] Step 3: Consult the corresponding "add" skill
- [ ] Step 4: Locate the feature's code
- [ ] Step 5: Determine the scope of the change
- [ ] Step 6: Apply the modifications
- [ ] Step 7: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Identify the Feature

Determine the feature's name and type by reading the microservice's `manifest.yaml`. The feature type corresponds to its manifest section:

| Manifest section | Feature type | Corresponding skill |
|---|---|---|
| `functions` | Functional endpoint | `microbus/add-function` |
| `webs` | Web handler | `microbus/add-web` |
| `configs` | Configuration property | `microbus/add-config` |
| `tickers` | Ticker | `microbus/add-ticker` |
| `metrics` | Metric | `microbus/add-metric` |
| `outboundEvents` | Outbound event | `microbus/add-outbound-event` |
| `inboundEvents` | Inbound event | `microbus/add-inbound-event` |

#### Step 3: Consult the Corresponding "Add" Skill

Read the `SKILL.md` of the corresponding "add" skill identified in the previous step (e.g., `.claude/skills/microbus/add-function/SKILL.md`). The templates in that skill define what the code should look like after modification. Use it as your reference throughout the remaining steps.

#### Step 4: Locate the Feature's Code

Search for `MARKER: FeatureName` within the microservice's directory and its subdirectories to find all code locations related to the feature. This gives you the complete set of places that may need editing.

#### Step 5: Determine the Scope of the Change

Categorize the requested change to understand its blast radius:

- **Implementation only** — Changes to the handler body in `service.go`. Tests in `service_test.go` may also need to be updated to reflect the new behavior.
- **Property change** — Changes to the route, method, required claims, description, config default or validation, ticker interval, or metric buckets. Affects a subset of the marked locations, typically `intermediate.go`, `myserviceapi/client.go`, and `manifest.yaml`.
- **Signature change** — Arguments added, removed, renamed, or retyped. This has a wide blast radius: every file with the feature's marker must be updated. See the detailed list below.
- **Rename** — The feature's name changes. This has the widest blast radius: all markers, identifiers, struct names, and references must be renamed across every file.

These scopes can overlap. For example, a rename combined with a signature change should be treated as a single pass: rename all identifiers and markers while simultaneously updating the signature at each location.

#### Step 6: Apply the Modifications

Edit each affected location, using the "add" skill's templates as the reference for what the updated code should look like.

For **implementation-only** changes, edit the handler in `service.go` and update tests in `service_test.go` if the expected behavior changes.

For **property changes**, edit the relevant subset of marked locations. Common examples:
- Route change: update the route const in `myserviceapi/client.go`, the `Subscribe` call in `intermediate.go`, and `manifest.yaml`
- Method change: update the `Subscribe` call in `intermediate.go`, the `_method` value in client methods in `myserviceapi/client.go`, the OpenAPI registration in `intermediate.go`, and `manifest.yaml`
- Required claims change: update the `Subscribe` call in `intermediate.go`, the OpenAPI registration in `intermediate.go`, and `manifest.yaml`

For **signature changes**, update every marked location. All of the following must be updated to reflect the new signature:
- `intermediate.go` — `ToDo` interface method, marshaler function, OpenAPI registration (Summary, InputArgs, OutputArgs)
- `myserviceapi/client.go` — In/Out/Response structs and their methods, Client method, MulticastClient method
- `myserviceapi/*.go` — Add definitions for new complex types; remove definitions for types no longer used
- `service.go` — Handler function signature
- `mock.go` — Mock field type, MockX setter signature, X executor signature, mock test case
- `service_test.go` — Test cases that call the function with the old signature

For **renames**, update every marked location with the new feature name. This includes all identifiers derived from the feature name (e.g., `MyFunction` becomes `NewName`, `doMyFunction` becomes `doNewName`, `MockMyFunction` becomes `MockNewName`, `MyFunctionIn` becomes `NewNameIn`, `RouteOfMyFunction` becomes `RouteOfNewName`, etc.) and all `MARKER` comments. In `service_test.go`, also rename the test function (e.g., `TestMyService_MyFunction` becomes `TestMyService_NewName`) and the mock subtest name.

#### Step 7: Housekeeping

Follow the `microbus/housekeeping` skill.
