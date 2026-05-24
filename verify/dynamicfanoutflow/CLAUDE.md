# dynamicfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for dynamic fan-out via `forEach`. The graph is `A -> forEach(items) -> B -> C`: TaskA emits a list of items, one TaskB instance runs per element, and TaskC sums the per-element results.

## Patterns exercised

- Dynamic fan-out (`graph.AddTransitionForEach`)
- `sum*` reducer convention (fan-in numeric add)
- `list*` reducer (per-branch `itemIndex` accumulated in spawn order via `listSeenIndices`)
- `set*` reducer (per-branch `itemCount` deduped via `setSeenCounts` to assert all branches saw the same cohort size)
- forEach branch state stripping: the source array (`items`) is removed from each branch's local state at spawn, then resurfaces in the fan-in step from the spawn step's immutable snapshot
- Explicit downstream suppression: a branch calling `flow.Set("items", nil)` writes null into its changes, and the replace reducer at fan-in folds it over the spawn-step base so `items` is absent past the fan-in
- forEach over a 0-element array (flow completes at the forEach source without firing the fan-in target)
- forEach over a 1-element array
