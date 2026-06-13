# nestedfailfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **deferred-cascade failure handling in a nested fan-out**. The graph is a 3x3 nested forEach:

```
taskA --forEach(outers)--> taskO --forEach(inners)--> taskI --> joinI --> joinO --> END
```

The single cell `(outer=1, inner=1)` fails synchronously. The other 8 cells block on a test-controlled gate so all 9 cells are demonstrably in flight at the same time before any of the successful 8 are allowed to complete.

## Patterns exercised

- Two-level pure-graph nested fan-out via `forEach` (no subgraph escape hatch)
- Single-cell failure inside an inner cohort
- `cohort_failures` propagation from the inner spawn up to the outer spawn via `lineage_id`
- `joinI` firing for outer branches whose inner cohort succeeded; not firing for the failed cohort
- Flow row staying in `running` while siblings are still alive, then transitioning to `failed` after the outer cohort fully resolves

## Assertions

Three subtests:

- `flow_running_while_siblings_still_in_flight` - after observing all 9 inner cells have started, the flow is still in `running` status (the failing cell's failStep did not preempt the cohort).
- `flow_failed_after_full_resolution` - after the gate is released and every cell terminates, the flow has transitioned to `failed`.
- `eight_inners_completed_and_two_joinI_fired` - all 9 inner cells ran (`innerStarts==9`), exactly 8 of them completed (`innerCompleted==8`), `joinI` ran exactly twice (for outer=0 and outer=2, whose inner cohorts had no failures), and `joinO` never ran (the outer cohort had a failed branch via propagation from outer=1).

When `RestartFrom` is added, this fixture can be extended with a follow-up subtest that restarts the failing cell with overrides that succeed, asserts only the restarted cell re-executes, and asserts the flow ends in `completed`.
