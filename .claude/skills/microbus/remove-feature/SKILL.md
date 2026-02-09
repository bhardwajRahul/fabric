---
name: Removing a Feature of a Microservice
description: Removes a configuration property, functional endpoint, event source, event sink, web handler endpoint, ticker or metric, from a microservice. Use when explicitly asked by the user to remove a feature of a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Removing a feature of a microservice:
- [ ] Step 1: Remove marked code
- [ ] Step 2: Remove unused custom types
- [ ] Step 3: Update manifest
- [ ] Step 4: Document the microservice
- [ ] Step 5: Versioning
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

If the deleted feature was using non-primitive custom types that are no longer used elsewhere by the microservice, remove the definition of the unused types from the `myserviceapi` directory.

#### Step 3: Update Manifest

Remove the relevant entry from `manifest.yaml`.

#### Step 4: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` to reflect the removal.

#### Step 5: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.
