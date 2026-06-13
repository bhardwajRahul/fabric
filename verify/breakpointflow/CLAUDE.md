# breakpointflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `BreakBefore` + `ResumeBreak`. The graph is `A -> B -> C`. The test sets a breakpoint on TaskB before starting the flow; the foreman parks the flow with status=interrupted before TaskB executes. `ResumeBreak` merges state overrides into the leaf step and allows execution to proceed without re-triggering the breakpoint (via the `breakpoint_hit` step column).

The fixture also pins the strict separation between the two resume entry points: `Resume` (the interrupt path) returns 409 on a breakpoint pause, and `ResumeBreak` is the only way to continue one.

## Patterns exercised

- `foremanapi.Client.BreakBefore(flowKey, taskURL, true)` setting a breakpoint
- Flow interrupts before the named task runs (not after)
- `Resume` rejects a breakpoint pause with 409 (it is for `flow.Interrupt` only)
- `ResumeBreak` continues past the breakpoint without re-triggering it (via the `breakpoint_hit` flag), injecting a state override that propagates to downstream tasks
- Subsequent tasks execute normally
