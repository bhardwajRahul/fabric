# onerrorsiblingsflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **OnError-does-not-cancel-siblings**. The graph is the same shape as `fanouterrorflow`:

```
taskA -> {taskB, taskC, taskD} -> taskE
taskB onError -> handler -> taskE
```

`taskB` always errors and routes via OnError to `handler`. `taskC` and `taskD` are normal siblings.

With the new failure model, when `taskB` errors:

- `taskB` is marked completed and `handler` is created as its successor in the same cohort lineage.
- `taskC` and `taskD` are NOT cancelled. They keep running and reach `taskE` on the success path.

This is the behavior the test asserts.

## Patterns exercised

- Cohort `{taskB, taskC, taskD}` where one member routes via OnError and the other two follow the normal path
- `handler` inherits `taskB`'s lineage_id (OnError edges don't push a frame), so its transition into `taskE` bumps the same cohort_arrivals as `taskC`/`taskD`'s arrivals
- Fan-in at `taskE` with `cohort_arrivals == cohort_size == 3` and `cohort_failures == 0` (because OnError converted the failure into a success)

## Assertions

Single subtest `flow_completes_with_handler_and_siblings`:

- Flow status is `completed`.
- `recovered == true`: `handler` ran and `taskB` never set `markB`.
- `siblingsRan == true`: both `taskC` and `taskD` reached `taskE` with their marks set (i.e. neither was cancelled).
