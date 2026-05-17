# fanouterrorflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the OnError + fan-out interaction. The graph is `A -> {B, C, D} -> E` with
`B onError -> Handler -> E` and `graph.SetFanIn("taskE")`. When TaskB errors, the foreman cancels its
siblings (C and D) and routes through Handler instead. Two distinct hazards live on this path:

1. A sibling finishing simultaneously with the sibling-cancel must not call `failStep` and fail the parent
   flow (the fan-in worker returns nil on failed/cancelled siblings rather than escalating).
2. Handler must actually run and its recovery state must reach TaskE. Under the old depth-based fan-in a
   sibling completing before B's error routing could insert the fan-in target at the next depth first and
   block Handler's insert, so the flow completed but `recovered` was false. The lineage-based fan-in
   coordinates by spawn cohort rather than `step_depth`, so Handler always runs and TaskE observes
   `recovered == true`.

## Patterns exercised

- Fan-out with one branch defining `onError`
- Sibling-cancel triggered by the errored task
- Convergence at E via the Handler path, coordinated by lineage fan-in
- Stress test: repeated execution (via `-count=N`) surfaces either race if it returns

## Assertions

Two subtests, both stressed via `-count=N`:

- `flow_does_not_fail` - the flow completes cleanly (hazard 1).
- `handler_runs_and_state_reaches_taskE` - `recovered == true`, i.e. Handler ran and its state reached
  TaskE (hazard 2). This was disabled until the lineage fan-in landed; it is now active and is the
  regression guard for the depth-collision class.
