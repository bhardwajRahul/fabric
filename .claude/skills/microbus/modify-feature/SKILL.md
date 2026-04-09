---
name: modify-feature
description: TRIGGER when user asks to change the signature, route, method, arguments, or return type of an existing endpoint, config, metric, or ticker. Coordinates changes across service.go, intermediate.go, client.go, mock.go, and manifest.yaml.
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
| `functions` | Functional endpoint | `add-function` |
| `webs` | Web handler | `add-web` |
| `configs` | Configuration property | `add-config` |
| `tickers` | Ticker | `add-ticker` |
| `metrics` | Metric | `add-metric` |
| `outboundEvents` | Outbound event | `add-outbound-event` |
| `inboundEvents` | Inbound event | `add-inbound-event` |
| `tasks` | Task endpoint | `add-task` |
| `workflows` | Workflow graph | `add-workflow` |

#### Step 3: Consult the Corresponding "Add" Skill

Read the `SKILL.md` of the corresponding "add" skill identified in the previous step (e.g., `.claude/skills/microbus/add-function/SKILL.md`). The templates in that skill define what the code should look like after modification. Use it as your reference throughout the remaining steps.

#### Step 4: Locate the Feature's Code

Search for `MARKER: FeatureName` within the microservice's directory and its subdirectories to find all code locations related to the feature. This gives you the complete set of places that may need editing.

#### Step 5: Determine the Scope of the Change

Categorize the requested change to understand its blast radius:

- **Implementation only** - Changes to the handler body in `service.go`. Tests in `service_test.go` may also need to be updated to reflect the new behavior.
- **Property change** - Changes to the route, method, required claims, description, config default or validation, ticker interval, or metric buckets. Affects a subset of the marked locations, typically `intermediate.go`, `myserviceapi/client.go`, and `manifest.yaml`.
- **Signature change** - Arguments added, removed, renamed, or retyped. This has a wide blast radius: every file with the feature's marker must be updated. See the detailed list below.
- **Rename** - The feature's name changes. This has the widest blast radius: all markers, identifiers, struct names, and references must be renamed across every file.

These scopes can overlap. For example, a rename combined with a signature change should be treated as a single pass: rename all identifiers and markers while simultaneously updating the signature at each location.

#### Step 6: Apply the Modifications

Edit each affected location, using the "add" skill's templates as the reference for what the updated code should look like.

For **implementation-only** changes, edit the handler in `service.go` and update tests in `service_test.go` if the expected behavior changes.

For **property changes**, edit the relevant subset of marked locations. Common examples:
- Route change: update the `Route` field of the `Def{...}` literal in `myserviceapi/endpoints.go` and the route in `manifest.yaml`. The `svc.Subscribe(...)` call in `intermediate.go` reads the route via `myserviceapi.Foo.Route` and needs no edit
- Method change: update the `Method` field of the `Def{...}` literal in `myserviceapi/endpoints.go` and the method in `manifest.yaml`. The `svc.Subscribe(...)` call reads it via `myserviceapi.Foo.Method`
- Description change: update the godoc on the handler in `service.go`, the `sub.Description(...)` argument inside the `svc.Subscribe("FeatureName", ...)` block in `intermediate.go`, the godoc on the Client/MulticastClient methods in `myserviceapi/client.go`, and the `description` in `manifest.yaml`
- Required claims change: add or update the `sub.RequiredClaims(...)` option inside the `svc.Subscribe("FeatureName", ...)` block in `intermediate.go`, and update the `requiredClaims` field in `manifest.yaml`

For **signature changes**, update every marked location. All of the following must be updated to reflect the new signature:
- `intermediate.go` - `ToDo` interface method, marshaler function, the `sub.Function/Web/Task/Workflow(In{}, Out{})` argument in the `svc.Subscribe` block (it carries the input/output struct types used for OpenAPI schema reflection)
- `myserviceapi/endpoints.go` - In/Out structs (the `Def{...}` literal itself only carries Method/Route and is unchanged unless they too change)
- `myserviceapi/client.go` - Response struct, Client method, MulticastClient method (and Hook for outbound events / Executor for tasks/workflows)
- `myserviceapi/*.go` - Add definitions for new complex types; remove definitions for types no longer used
- `service.go` - Handler function signature
- `mock.go` - Mock field type, MockX setter signature, X executor signature, mock test case
- `service_test.go` - Test cases that call the function with the old signature

For **renames**, update every marked location with the new feature name. This includes all identifiers derived from the feature name (e.g., `MyFunction` becomes `NewName`, `doMyFunction` becomes `doNewName`, `MockMyFunction` becomes `MockNewName`, `MyFunctionIn` becomes `NewNameIn`, etc.), the first argument to `svc.Subscribe("MyFunction", ...)` in `intermediate.go`, and all `MARKER` comments. In `service_test.go`, also rename the test function (e.g., `TestMyService_MyFunction` becomes `TestMyService_NewName`) and the mock subtest name.

#### Step 7: Housekeeping

Follow the `housekeeping` skill.
