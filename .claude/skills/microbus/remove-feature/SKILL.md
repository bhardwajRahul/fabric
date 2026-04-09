---
name: remove-feature
description: TRIGGER when user asks to remove, delete, or drop an endpoint, config, metric, ticker, or event from a microservice. Safely removes code from service.go, intermediate.go, client.go, mock.go, service_test.go, and manifest.yaml.
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

A marker comment on a line that opens a `{` or `(` group suggests that the entire block - including everything up to the matching closing brace or paren - should be removed.

```go
if example { // MARKER: FeatureName
	// ...
}
```

```go
svc.Subscribe( // MARKER: FeatureName
	"FeatureName", svc.doFeatureName,
	sub.At(..., ...)
	...
)
```

Otherwise, the marker suggests that a single line should be removed.

```go
var example // MARKER: FeatureName
```

```go
FeatureName = Def{Method: "ANY", Route: "/feature-name"} // MARKER: FeatureName
```

#### Step 2: Remove Unused Custom Types

If the deleted feature was using non-primitive custom types defined in `myserviceapi` directory, and those types are no longer used elsewhere by the microservice, remove their definition.

#### Step 3: Housekeeping

Follow the `housekeeping` skill.
