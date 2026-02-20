---
name: Renaming the object persisted by a SQL CRUD microservice
description: Renames the type of the object used by a SQL CRUD microservice. Use when explicitly asked by the user to rename the object or its type definition.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**IMPORTANT**: `MyNoun`, `MyNounKey`, `mynoun`, and `mynounapi` are placeholders for the actual object, its key, directory, and API package of the microservice.

## Workflow

Copy this checklist and track your progress:

```
Renaming the object:
- [ ] Step 1: Read Local AGENTS.md File
- [ ] Step 2: Update Type Definitions
- [ ] Step 3: Update References
- [ ] Step 4: Alias the Old Type Definitions
- [ ] Step 5: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Update Type Definitions

Find the struct definition of `MyNounKey` in `mynounapi/objectkey.go` and rename it to the new name `MyNewNounKey`.

Find the struct definition of `MyNoun` in `mynounapi/object.go` and rename it to the new name `MyNewNoun`.

#### Step 3: Update References

Search the files in `mynounapi/` for references to `MyNounKey` and `MyNoun`. Update to the new names `MyNewNounKey` and `MyNewNoun` respectively.

Search all files in the directory of the microservice for `mynounapi.MyNounKey` and `mynounapi.MyNoun`. Update to the new names `mynounapi.MyNewNounKey` and `mynounapi.MyNewNoun`.

Search all other project files for `mynounapi.MyNounKey` and `mynounapi.MyNoun`. Update to the new names `mynounapi.MyNewNounKey` and `mynounapi.MyNewNoun`. Include all `.go` and `manifest.yaml` files.

Verify that the project builds ok.

#### Step 4: Alias the Old Type Definitions

This step intentionally reintroduces the old type names as deprecated aliases for backwards compatibility.

Add a type alias of the old object key to the new object key in `mynounapi/objectkey.go`

```go
// MyNounKey is an alias to MyNewNounKey.
//
// Deprecated: Use [MyNewNounKey]
type MyNounKey = MyNewNounKey
```

Add a type alias of the old object to the new object in `mynounapi/object.go`

```go
// MyNoun is an alias to MyNewNoun.
//
// Deprecated: Use [MyNewNoun]
type MyNoun = MyNewNoun
```

#### Step 5: Housekeeping

Follow the `microbus/housekeeping` skill. Skip the topology step.
