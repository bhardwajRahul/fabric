# failedfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for a hard failure inside a static fan-out. The graph is
`Src -> {A, B, C} -> J`. Src fans out; A and C succeed (contributing 1 each to the sum*
reducer field sumExecuted); B always returns an error. There is **no** OnError transition,
so B's failure fails B's step and `failStep` cascades the whole flow to failed. The fan-in J
is never reached.

This is distinct from `fanouterrorflow`, which adds an OnError handler and recovers (the flow
completes). Here the flow is expected to terminate as failed.

## Patterns exercised

- Static fan-out (`Src` -> three unconditional transitions) with explicit `SetFanIn`
- A fan-out branch returning an error with no OnError route -> `failStep` cascade
- Flow-level failure as an outcome: the synchronous Executor returns `err == nil` with
  `status == foremanapi.StatusFailed` (failure is a flow status, not a transport error)

## Deterministic assertions

- `err` from the Executor is nil
- final status is `foremanapi.StatusFailed`

B always errors regardless of how A/C race, and the fan-in gate never reaches all three
arrivals, so the failed outcome is fully deterministic without worker pinning or sleeps.
