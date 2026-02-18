---
name: Upgrade a Microservice to V1.22.0
description: Upgrades a single microservice to v1.22.0.
---

**CRITICAL**: Do NOT explore or analyze existing code before starting. This skill is self-contained.

**IMPORTANT**: This skill affects a single microservice and all file names are relative to the directory of the microservice. 

## Workflow

Copy this checklist and track your progress:

```
Upgrade a microservice from v1 to v2:
- [ ] Step 1: Verify version
- [ ] Step 2: Delete and rename files
- [ ] Step 3: Extend AGENTS.md
- [ ] Step 4: Modify service.go
- [ ] Step 5: Follow add-microservice skill
- [ ] Step 6: Transfer version
- [ ] Step 7: Transfer functions
- [ ] Step 8: Transfer webs
- [ ] Step 9: Transfer metrics
- [ ] Step 10: Transfer configs
- [ ] Step 11: Transfer events
- [ ] Step 12: Transfer sinks
- [ ] Step 13: Delete service.yaml
- [ ] Step 14: Clients no longer accept contentType
```

#### Step 1: Verify Version

If there is no `service.yaml` in the current directory, the microservice is not built using Microbus v1. Exit without doing anything.

#### Step 2: Delete and Rename Files

Delete `doc.go`, `service-gen.go`, `myserviceapi/clients-gen.go`, `version-gen_test.go` and the `intermediate` subdirectory.

Rename `resources/embed-gen.go` to `resources/embed.go`.

#### Step 3: Extend `AGENTS.md`

In `AGENTS.md`, add the following right after the two CRITICAL instructions at the top of the file.

```md
**IMPORTANT**: Keep track of prompts affecting this microservices in `PROMPTS.md`.

**IMPORTANT**: Keep track of features of this microservices in `manifest.yaml`.
```

#### Step 4: Modify `service.go`

In `service.go`:

- Modify the `Service` struct definition and replace the anonymous field `*intermediate.Intermediate` with `*connector.Connector` instead.
- Remove the import statement of the intermediate `"github.com/mycompany/myproject/myservice/intermediate"`.
- Add an import statement for the connector `"github.com/microbus-io/fabric/connector"`

#### Step 5: Follow `add-microservice` Skill

Read the `general` properties from `service.yaml`. Keep in mind the hostname and description of the microservice as you follow the steps of the `add-microservice` skill next.

Follow only the following steps from the `add-microservice` skill, in this order:
- Prepare `client.go`
- Prepare `intermediate.go`
- Prepare `mock.go`
- Prepare `service_test.go`

#### Step 6: Transfer Version

Take the value of the `const Version` from `version-gen.go`, increment it by 1, and set the new value to the `const Version` in `intermediate.go`. Then, delete `version-gen.go`.

#### Step 7: Transfer Functions

For each definition in the `functions` section in `service.yaml`, perform the `add-function` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: MyFunction` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: MyFunction` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 8: Transfer Webs

For each definition in the `webs` section in `service.yaml`, perform the `add-web` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: MyWeb` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: MyWeb` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 9: Transfer Metrics

For each definition in the `metrics` section in `service.yaml`, perform the `add-metric` skill.

- If the metric is observable, it will already have `OnObserveMyMetric` implemented in `service.go`. Do not overwrite the original implementation. Bind the original implementation in `intermediate.go`. Do add the `MARKER: MyMetric` comment in `service.go` next to the original implementation.
- Skip the documentation and versioning steps.

The API to increment counter metrics may require a change from `svc.AddMetric` to `svc.IncrementMetric`. Make any necessary changes in `service.go`.

#### Step 10: Transfer Configs

For each definition in the `configs` section in `service.yaml`, perform the `add-config` skill.

- If the config has a callback, it will already have `OnChangedMyConfig` implemented in `service.go`. Skip the step to implement the callback. Do not overwrite the original implementation. Wire up the original implementation in `intermediate.go`. Do add the `MARKER: MyConfig` comment in `service.go` next to the original implementation
- Skip the documentation and versioning steps

#### Step 11: Transfer Events

For each definition in the `events` section in `service.yaml`, perform the `add-event` skill.

- Skip the step of triggering the event in `service.go` because the implementation is already there
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: OnMyEvent` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 12: Transfer Sinks

For each definition in the `sinks` section in `service.yaml`, perform the `add-sink` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: OnMyEvent` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: OnMyEvent` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 13: Delete `service.yaml`

Finally, delete `service.yaml`.

#### Step 14: Clients No Longer Accept `contentType`

Clients in v2 no longer accept a `contentType` parameter for web endpoints. Remove the superfluous argument when calling web endpoints via a client. Use `WithOptions(pub.ContentType(...)))` if the content type was not `""`.
