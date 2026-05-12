# interruptflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `flow.Interrupt` + `Resume`. The graph is `A -> AwaitInput -> Compose`. AwaitInput checks whether `userInput` is in state — if absent, it calls `flow.Interrupt(payload)` to park the flow for external input; if present, it proceeds. On Resume the foreman merges the supplied data into the leaf step's state and re-executes AwaitInput, which then falls through.

## Patterns exercised

- `flow.Interrupt(payload)` parking the flow with status=interrupted
- `Resume(flowKey, data)` merging caller data into leaf step state
- Task re-execution after Resume (the interrupting task is invoked again with the merged state)
- The "check-and-interrupt" pattern: a task that idempotently interrupts until the required input has been provided

## Test approach

Uses `foremanapi.Client` directly (Create + Start + Await + Resume + Await) rather than the simpler `exec.Workflow()` helper, because the workflow goes through two stop-states (interrupted, then completed) that `Run` would not surface separately.
