---
name: Removing a Feature of a Microservice
description: Removes a configuration property, functional endpoint, event source, event sink, web handler endpoint, ticker or metric, from a microservice. Use when explicitly asked by the user to remove a feature of a microservice.
---

## Workflow

Copy this checklist and track your progress:

```
Removing a feature of a microservice:
- [ ] Step 1: Remove definition from service.yaml
- [ ] Step 2: Remove implementation
- [ ] Step 3: Remove test
- [ ] Step 4: Remove unused custom types
- [ ] Step 5: Update boilerplate code
- [ ] Step 6: Document the microservice
```

#### Step 1: Remove definition from `service.yaml`

Remove the definition from `service.yaml`.

#### Step 2: Remove implementation

Remove any implementation code from `service.go`.

#### Step 3: Remove test

Skip this step if integration tests were skipped for this microservice.

Remove the corresponding test fom `service_test.go`.

#### Step 4: Remove unused custom types

If the deleted definition was using non-primitive custom types that are no longer used elsewhere, remove the definition of the unused types from the API directory.

#### Step 5: Update boilerplate code

Run `go generate` to update the boilerplate code.

#### Step 6: Document the microservice

Skip this step if instructed to be "quick".

Update the microservice's local `AGENTS.md` to reflect the removal.
