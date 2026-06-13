# reducerflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the explicit `graph.SetReducer` wiring at fan-in. The graph is `A -> {B, C, D} -> E` where each fan-out sibling writes deltas to three reducer-managed state fields. The graph attaches the reducers at build time:

```go
graph.SetReducer("total", workflow.ReducerAdd)
graph.SetReducer("tags",  workflow.ReducerAppend)
graph.SetReducer("seen",  workflow.ReducerUnion)
```

- `total` (Add reducer): B contributes 10, C contributes 20, D contributes 30 -> total = 60
- `tags` (Append reducer): B contributes ["b"], C contributes ["c"], D contributes ["d"] -> tags = ["b","c","d"] in updated_at/step_id order
- `seen` (Union reducer): B contributes ["x"], C contributes ["y","x"], D contributes ["z"] -> seen = ["x","y","z"] deduplicated

TaskE reads the merged values and surfaces them.

## Patterns exercised

- `ReducerAdd` (numeric add)
- `ReducerAppend` (array append, duplicates kept)
- `ReducerUnion` (array union with dedupe)
- Delta-only writes from each fan-out sibling

## Important convention

Each fan-out branch sets only its *delta* (e.g. `totalOut=10` not the running total). The foreman applies the reducer at fan-in.
