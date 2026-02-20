---
name: Upgrade a Microservice to V1.22.0
description: Upgrades a single microservice to v1.22.0.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

**IMPORTANT**: This skill affects a single microservice and all file names are relative to the directory of the microservice. 

## Workflow

Copy this checklist and track your progress:

```
Upgrade a microservice to v1.22.0:
- [ ] Step 1: Verify version
- [ ] Step 2: Delete and rename files
- [ ] Step 3: Modify service.go
- [ ] Step 4: Follow add-microservice skill
- [ ] Step 5: Transfer version
- [ ] Step 6: Transfer functions
- [ ] Step 7: Transfer webs
- [ ] Step 8: Transfer metrics
- [ ] Step 9: Transfer configs
- [ ] Step 10: Transfer events
- [ ] Step 11: Transfer sinks
- [ ] Step 12: Delete service.yaml
- [ ] Step 13: Clients no longer accept contentType
```

#### Step 1: Verify Version

If there is no `service.yaml` in the current directory, the microservice is already upgraded to v1.22.0. Exit without doing anything.

#### Step 2: Delete and Rename Files

Delete `doc.go`, `service-gen.go`, `myserviceapi/clients-gen.go`, `version-gen_test.go` and the `intermediate` subdirectory.

Rename `resources/embed-gen.go` to `resources/embed.go`.

#### Step 3: Modify `service.go`

In `service.go`:

- Modify the `Service` struct definition and replace the anonymous field `*intermediate.Intermediate` with `*connector.Connector` instead.
- Remove the import statement of the intermediate `"github.com/mycompany/myproject/myservice/intermediate"`.
- Add an import statement for the connector `"github.com/microbus-io/fabric/connector"`

#### Step 4: Follow `microbus/add-microservice` Skill

Read the `general` properties from `service.yaml`. Keep in mind the hostname and description of the microservice as you follow the steps of the `microbus/add-microservice` skill next.

Follow only the following steps from the `microbus/add-microservice` skill, in this order:
- Prepare `client.go`
- Prepare `intermediate.go`
- Prepare `mock.go`
- Prepare `service_test.go`

#### Step 5: Transfer Version

Take the value of the `const Version` from `version-gen.go`, increment it by 1, and set the new value to the `const Version` in `intermediate.go`. Then, delete `version-gen.go`.

#### Step 6: Transfer Functions

For each definition in the `functions` section in `service.yaml`, perform the `microbus/add-function` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: MyFunction` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: MyFunction` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 7: Transfer Webs

For each definition in the `webs` section in `service.yaml`, perform the `microbus/add-web` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: MyWeb` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: MyWeb` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 8: Transfer Metrics

For each definition in the `metrics` section in `service.yaml`, perform the `microbus/add-metric` skill.

- If the metric is observable, it will already have `OnObserveMyMetric` implemented in `service.go`. Do not overwrite the original implementation. Bind the original implementation in `intermediate.go`. Do add the `MARKER: MyMetric` comment in `service.go` next to the original implementation.
- Skip the documentation and versioning steps.

The API to increment counter metrics may require a change from `svc.AddMetric` to `svc.IncrementMetric`. Make any necessary changes in `service.go`.

#### Step 9: Transfer Configs

For each definition in the `configs` section in `service.yaml`, perform the `microbus/add-config` skill.

- If the config has a callback, it will already have `OnChangedMyConfig` implemented in `service.go`. Skip the step to implement the callback. Do not overwrite the original implementation. Wire up the original implementation in `intermediate.go`. Do add the `MARKER: MyConfig` comment in `service.go` next to the original implementation
- Skip the documentation and versioning steps

#### Step 10: Transfer Events

For each definition in the `events` section in `service.yaml`, perform the `add-event` skill.

- Skip the step of triggering the event in `service.go` because the implementation is already there
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: OnMyEvent` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 11: Transfer Sinks

For each definition in the `sinks` section in `service.yaml`, perform the `add-sink` skill.

- Skip the step of implementation in `service.go` because the implementation is already there. Do add the `MARKER: OnMyEvent` comment in `service.go` next to the original implementation
- Skip the step of testing in `service_test.go` because a test is already there. Do add the `MARKER: OnMyEvent` comment in `service_test.go` next to the original test
- Skip the documentation and versioning steps
- If the route of the endpoint contains a greedy path argument ending with a `+` sign, e.g. `{suffix+}`, change the `+` to `...`

#### Step 12: Delete `service.yaml`

Finally, delete `service.yaml`.

#### Step 13: Clients No Longer Accept `contentType`

V1.22.0 clients no longer accept a `contentType` parameter for web endpoints. Remove the superfluous argument when calling web endpoints via a client. Use `WithOptions(pub.ContentType(...)))` if the content type was not `""`.
