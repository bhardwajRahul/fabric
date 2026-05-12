---
name: add-python-microservice
description: TRIGGER when user asks to create a microservice that calls Python, runs ML inference, uses PyTorch/pandas/sentence-transformers/numpy, or otherwise needs Python libraries for its core compute. Scaffolds a Microbus microservice that owns its own in-process Python virtual environment via github.com/microbus-io/pyvenv, with manual-subscription lifecycle, an embedded Python source tree, and a pip requirements file.
---

**CRITICAL**: Read `.claude/rules/python.txt` before proceeding.

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so.

## Workflow

This skill works in two modes:

- **New microservice**: starts at Step 1, runs the standard `add-microservice` scaffold first.
- **Extend existing microservice**: skips Step 1; the target package already exists and may have business logic. Every subsequent step is additive (it appends, never overwrites).

Copy this checklist and track your progress:

```
Adding Python support to a microservice:
- [ ] Step 1: Run add-microservice (skip when extending an existing service)
- [ ] Step 2: Add Python source files
- [ ] Step 3: Add python.go
- [ ] Step 4: Extend the Service fields
- [ ] Step 5: Add MaxWorkers config
- [ ] Step 6: Wire OnStartup and OnShutdown
- [ ] Step 7: Housekeeping
```

#### Step 1: Run `add-microservice`

Skip this step when extending an existing microservice.

Otherwise, run the `add-microservice` skill, skipping its housekeeping step (housekeeping runs once at the end of this skill instead). When it finishes, return here. The remaining steps assume `myservice/` is the microservice directory.

#### Step 2: Add Python Source Files

Refuse to overwrite if either file already exists. Copy the template files to the microservice's root:

```shell
test -e myservice/service.py        || cp .claude/skills/python/add-python-microservice/service.py myservice/service.py
test -e myservice/requirements.txt  || cp .claude/skills/python/add-python-microservice/requirements.txt myservice/requirements.txt
```

#### Step 3: Add `python.go`

Refuse to overwrite if the file already exists:

```shell
test -e myservice/python.go || cp .claude/skills/python/add-python-microservice/python.go myservice/python.go
```

Change `package myservice` to match the microservice's package name. Leave the rest unchanged.

#### Step 4: Extend the Service Fields

In `service.go`, append the `venv` field to the existing `Service` struct, preserving any fields already there:

```go
type Service struct {
    *Intermediate // IMPORTANT: Do not remove

    // ...existing fields...
    venv *pyvenv.Venv
}
```

Add these imports to `service.go` if they're not already there:

```go
import (
    // ...
    "github.com/microbus-io/fabric/connector"
    "github.com/microbus-io/pyvenv"
)
```

#### Step 5: Add `MaxWorkers` Config

Run `add-config`:

- **Name**: `MaxWorkers`
- **Type**: `int`
- **Validation**: `int [1,]`
- **Default**: match the workload (e.g. `1` for LLM inference, `4` for I/O, `8` for parallel numpy/pandas)
- **Description**: `MaxWorkers caps how many calls into the Python venv may run concurrently.`

#### Step 6: Wire `OnStartup` and `OnShutdown`

Add the Python lifecycle wiring to the existing `OnStartup` and `OnShutdown` bodies. Do **not** delete any existing code; append the new lines around it.

In `OnStartup`, add this block (anywhere in the body, but conventionally at the end so any earlier setup is in place before the venv goroutine launches):

```go
sources, err := readPythonSources()
if err != nil {
    return errors.Trace(err)
}
svc.venv, err = pyvenv.New(pyvenv.Config{
    Sources:          sources,
    Requirements:     parseRequirements(pythonRequirements),
    MaxWorkers:       svc.MaxWorkers(),
    Logger:           svc,
    LivenessCallback: svc.onVenvLiveness,
})
if err != nil {
    return errors.Trace(err)
}
// Start the venv in the background. Auto-start is gated on deployment; in TESTING the
// venv is left dormant (parallels how tickers don't run in TESTING).
if svc.Deployment() != connector.TESTING {
    // Tests opt-in by calling svc.StartPyVenv(ctx)
    svc.Go(ctx, svc.venv.Start)
}
```

In `OnShutdown`, add this block (conventionally at the start, so the venv is released before any downstream resources the service shuts down later):

```go
if svc.venv != nil {
    err := svc.venv.Close(ctx)
    if err != nil {
        svc.LogError(ctx, "Closing python venv failed", "error", err)
    }
}
```

#### Step 7: Housekeeping

Follow the `housekeeping` skill.
