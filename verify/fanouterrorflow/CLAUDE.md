# fanouterrorflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the OnError + fan-out interaction. The graph is `A -> {B, C, D} -> E` with
`B onError -> Handler -> E` and `graph.SetFanIn("taskE")`. When TaskB errors, the foreman marks B
completed and creates Handler as B's successor inside the same cohort lineage; the fan-in at E
collects contributions from all four exit steps (Handler, C, D — plus B's now-completed row, which
the merge filter skips since it has no usable changes).

## Patterns exercised

- Fan-out cohort with one member routing via OnError
- Handler created in the same cohort as B (OnError edges inherit lineage rather than pushing a frame)
- Siblings C and D run to completion and contribute to the fan-in at E

## Historical context (regression guards)

This fixture predates the no-sibling-cancel rule and was originally written to verify two race fixes
that mattered when OnError used to cancel siblings:

1. A sibling finishing simultaneously with the sibling-cancel must not call `failStep` and fail the
   parent flow.
2. Handler must actually run and its recovery state must reach TaskE. Under the old depth-based
   fan-in a sibling completing before B's error routing could insert the fan-in target at the next
   depth first and block Handler's insert, so the flow completed but `recovered` was false. The
   lineage-based fan-in coordinates by spawn cohort rather than `step_depth`.

Both hazards are now structurally impossible: OnError no longer cancels siblings, so there is no
sibling-cancel race to win or lose. The assertions are kept as regression guards in case either
behavior is reintroduced.

## Assertions

Two subtests, both stressable via `-count=N`:

- `flow_does_not_fail` - the flow completes cleanly.
- `handler_runs_and_state_reaches_taskE` - `recovered == true`, i.e. Handler ran and its state
  reached TaskE.

For an explicit assertion that C and D are not cancelled (the no-sibling-cancel behavior itself),
see [`verify/onerrorsiblingsflow`](../onerrorsiblingsflow), which surfaces `siblingsRan` as a
typed output from TaskE.
