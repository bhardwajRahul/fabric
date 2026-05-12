# aliasflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the case where the **same task URL** appears at two distinct positions in a workflow graph under two distinct node names. The graph is:

```
s -> a -> b -> c -> END
s -> bPrime -> d -> END   (taken via flow.Goto)
```

Nodes `b` and `bPrime` both dispatch to `TaskB.URL()` but are independently identified in the graph and recorded under their respective node names in the step history.

`s` chooses the path: with `branch == "alt"` it calls `flow.Goto("bPrime")` to take the alternate path; otherwise the default normal transition `s -> a` fires.

## Patterns exercised

- Two `AddTask` registrations sharing one task URL but using different node names
- `AddTransitionGoto` for the alternate-path edge (excluded from the fan-out sibling validator)
- `flow.Goto(name)` selecting the alternate path at runtime
- Step history surfacing the node name (not the URL) so downstream tooling can distinguish positions
- Round-trip through the foreman: graph load, transition evaluation, dispatch URL resolution via `URLOf(name)`
