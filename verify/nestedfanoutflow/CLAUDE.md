# nestedfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **two-level nested fan-out** via a subgraph call. The outer graph is `A -> {NormalB, RunInner} -> J`, where `RunInner` is a task that calls `flow.Subgraph(Inner.URL())`. The inner subgraph itself has an internal fan-out: `X -> {Y, Z} -> W`. So the workflow exhibits fan-out at the outer level and fan-out inside one of the siblings.

The reason this fixture nests via a subgraph instead of pure graph nesting: the foreman's fan-out sibling constraint requires all siblings at a fan-out depth to share the same downstream targets. A pure-graph "inner fan-out per outer sibling" would violate that. A subgraph call sidesteps the constraint by isolating the inner pipeline behind its own graph.

## Patterns exercised

- Outer fan-out (2 heterogeneous siblings: NormalB + the RunInner subgraph caller)
- Inner fan-out inside the subgraph (X -> {Y, Z} -> W)
- Explicit `SetReducer("inner", ReducerAdd)` applied at the inner fan-in (Y and Z each contribute deltas)
- RunInner adopting the subgraph's `innerResult` output and returning it as its own output
- Outer fan-in at J combining NormalB's output with the subgraph caller's output
