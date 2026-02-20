---
name: Removing a Feature of a Microservice
description: Removes a configuration property, functional endpoint, event source, event sink, web handler endpoint, ticker or metric, from a microservice. Use when explicitly asked by the user to remove a feature of a microservice.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Removing a feature of a microservice:
- [ ] Step 1: Remove marked code
- [ ] Step 2: Remove unused custom types
- [ ] Step 3: Housekeeping
```

#### Step 1: Remove Marked Code

Scan the files in the directory of the microservice and its subdirectories for `MARKER: FeatureName` to locate the code related to the feature.

A marker comment following `{` or `(` suggests that the entire code block should be removed.

```go
if example { // MARKER: FeatureName
	// ...
}
```

Otherwise, the marker suggests that a single line should be removed.

```go
var example // MARKER: FeatureName
```

#### Step 2: Remove Unused Custom Types

If the deleted feature was using non-primitive custom types defined in `myserviceapi` directory, and those types are no longer used elsewhere by the microservice, remove their definition.

#### Step 3: Housekeeping

Follow the `microbus/housekeeping` skill.
