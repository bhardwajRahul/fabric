# breakpointflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `BreakBefore` + Resume. The graph is `A -> B -> C`. The test sets a breakpoint on TaskB before starting the flow; the foreman parks the flow with status=interrupted before TaskB executes. Resume clears the breakpoint flag (via the `breakpoint_hit` step column) and allows execution to proceed.

## Patterns exercised

- `foremanapi.Client.BreakBefore(flowKey, taskURL, true)` setting a breakpoint
- Flow interrupts before the named task runs (not after)
- Resume continues past the breakpoint without re-triggering it (via the `breakpoint_hit` flag)
- Subsequent tasks execute normally
