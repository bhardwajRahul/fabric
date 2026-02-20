---
name: Upgrade a Microservice to V1.23.0
description: Upgrades a single microservice to v1.23.0.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

**IMPORTANT**: This skill affects a single microservice and all file names are relative to the directory of the microservice. 

## Workflow

Copy this checklist and track your progress:

```
Upgrade a microservice to v1.23.0:
- [ ] Step 1: Verify framework version
- [ ] Step 2: Update manifest.yaml
- [ ] Step 3: Regenerate boilerplate
- [ ] Step 4: Update framework version
```

#### Step 1: Verify Framework Version

If the `frameworkVersion` in `manifest.yaml` is 1.23.0 or later, the microservice is already upgraded. Exit without doing anything.

#### Step 2: Update manifest.yaml

Update `manifest.yaml` to match the new spec:

- **Configs**: Replace the `type` property with a `signature` property that describes the getter, in the form `ConfigName() (returnName returnType)`. Look at the getter function in `intermediate.go` (search for `MARKER: ConfigName`) to determine the return argument name and type.
- **Configs**: Add `secret: true` for configs that use `cfg.Secret()` in their `DefineConfig` call in `intermediate.go`.
- **Configs**: Add `callback: true` for configs that have a corresponding `if changed("ConfigName")` entry in `doOnConfigChanged` in `intermediate.go`.
- **Metrics**: Add a `signature` property that describes the value type and labels, in the form `MetricName(value valueType, label1 string, label2 string)`. Look at the `RecordMetricName` or `IncrementMetricName` function in `intermediate.go` (search for `MARKER: MetricName`) to determine the arguments.
- **Tickers**: Add a `signature` property in the form `TickerName()`.

#### Step 3: Regenerate Boilerplate

Follow the `microbus/regenerate-boilerplate` skill to regenerate the boilerplate files of the microservice.

#### Step 4: Update Framework Version

In `manifest.yaml`, set `frameworkVersion` to `1.23.0`.
