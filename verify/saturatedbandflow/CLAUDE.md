# saturatedbandflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's band-fallthrough behavior in `runRefill`. When a strict
priority band consists entirely of tasks at their adaptive concurrency limit, the refiller advances
past it to the next band (up to `maxSaturatedBandFallthrough`) rather than idling the cluster while
the saturated band drains.

The graph defines two task endpoints with distinct names so each gets its own per-task adaptive
limit: `Bounded` (self-emits 429 above a small cap) and `Open` (always succeeds). Two workflows pair
each task with a priority: `SaturatedBand` runs Bounded at high priority, `OpenBand` runs Open at low
priority. The test starts many flows in each, asserts that all complete, and asserts that at least
one OpenBand flow completes before the last SaturatedBand flow does. Without band fallthrough that
ordering is impossible (strict priority would force every SaturatedBand step ahead of every OpenBand
step).

## Patterns exercised

- Per-task adaptive limit driven by task self-emitting 429 (same mechanism as `maxconcurrencyflow`)
- `runRefill`'s `priority > prevBand` loop advancing past a band whose every task is saturated
- The `StepsSkippedSaturated` counter incrementing as saturated rows are dropped pre-fairness draw
- Cross-band ordering: high priority does NOT starve low priority when high is downstream-throttled

## Determinism caveat

Exact completion ordering is timing-dependent (the Bounded task's cap drives how many concurrent
runs it admits before a 429, which in turn drives how fast the SaturatedBand backlog drains). The
load-bearing assertion is the ordering invariant - some OpenBand flow finishes before all
SaturatedBand flows - not exact counts or exact wall-clock timings.
