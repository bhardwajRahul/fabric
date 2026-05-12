# reducerflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the prefix-driven reducer conventions at fan-in. The graph is `A -> {B, C, D} -> E` where each fan-out sibling writes deltas to three reducer-managed state fields:

- `sumTotal` (numeric add reducer): B contributes 10, C contributes 20, D contributes 30 -> sumTotal = 60
- `listTags` (array append reducer): B contributes ["b"], C contributes ["c"], D contributes ["d"] -> listTags = ["b","c","d"] in updated_at/step_id order
- `setSeen` (polymorphic union reducer): B contributes ["x"], C contributes ["y","x"], D contributes ["z"] -> setSeen = ["x","y","z"] deduplicated

TaskE reads the merged values and surfaces them.

## Patterns exercised

- `sum*` reducer (numeric add)
- `list*` reducer (array append, duplicates kept)
- `set*` reducer (array union with dedupe)
- Delta-only writes from each fan-out sibling

## Important convention

Each fan-out branch sets only its *delta* (e.g. `sumTotalOut=10` not the running total). The foreman applies the reducer at fan-in.
