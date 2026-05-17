# priorityflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's priority-based dispatch. The graph is the single task
`record -> END`. The test pins the foreman to one worker and to a single fairness key (no `FairnessKey`
option, no tenant claim, so every flow lands in the `""` bucket), then creates flows at several
priorities while a long-running holder flow occupies the lone worker. When the holder finishes, the full
set is pending and the worker drains it strictly by the foreman's selection.

## Patterns exercised

- `*workflow.FlowOptions{Priority: n}` supplied at `Create`
- Strict priority bands: lower priority number runs first, no aging
- Starvation by design: a continuously available higher-priority set delays a low-priority flow until
  the higher-priority work is exhausted (no safety valve)

## Determinism caveat

With a single fairness key the only probabilistic step of the foreman's two-level selection (the
weighted-random pick among candidate keys) degenerates to deterministic, and within a key selection is
FIFO by `step_id` (creation order). The holder flow guarantees the whole test set is pending before the
worker frees, so the recorded dispatch order is the exact selection order: strict priority ascending,
then creation order within a priority. These tests therefore make exact ordering assertions and must not
flake. Priority 0 in `FlowOptions` means "use `DefaultPriority`"; tests use priorities >= 1.
