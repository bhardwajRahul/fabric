# retryfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for ordered list fan-in under retries. The graph is
`Enter -> forEach(elements) -> Increment -> Join`. Enter echoes the input integer array; one
Increment instance runs per element with a random 10% failure that triggers infinite
`flow.RetryNow()`; on success it contributes `[element+1]` as its `listResult` delta. Join is
the explicit `SetFanIn` target; the `list*` reducer appends each branch's delta. The test asserts
`listResult == [1..100]` for input `[0..99]`.

## Patterns exercised

- Dynamic fan-out (`graph.AddTransitionForEach`) over an integer array
- Explicit lineage fan-in (`graph.SetFanIn`)
- `list*` append reducer with single-element deltas
- Task-initiated infinite retry (`flow.RetryNow`) inside a fan-out branch
- Deterministic fan-in ordering by `fan_out_ordinal`: despite random per-branch retry latency
  scrambling completion order, the appended `listResult` stays in input-array order. This is the
  regression guard for the foreman's `fan_out_ordinal` merge ordering and the retry path's
  `lineage_id`/`fan_out_ordinal` copy.
