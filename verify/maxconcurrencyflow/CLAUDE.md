# maxconcurrencyflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's adaptive per-task concurrency control. The graph is the single
task `bounded -> END`. The `Bounded` task self-emits HTTP 429 when its observed in-flight count
exceeds a configured cap, exercising the full backpressure path: dispatch returns 429, the foreman
cuts the per-task adaptive limit, gossips the new anchor, and bounces the step back to pending under
a short backoff. The bounced step is later re-selected under the reduced limit. Steps are never
failed by this path.

The test pins the foreman to one shard with a generous worker pool and creates many flows. It then
asserts that (a) every flow completes (a 429'd step is delayed, not failed), (b) the task observed
at least one in-flight count above the cap (proving the 429 path actually fired), and (c) the
backpressure backoff metric was incremented (proving the controller learned).

## Patterns exercised

- Task self-emitting `http.StatusTooManyRequests` to trigger the foreman's 429 path
- The foreman's `handleBackpressure` flow: count -> regulate (debounced) -> gossip -> bounce
- The runRefill saturated-set filter dropping a task with no headroom
- Recovery via the CUBIC curve as in-flight counts drop and the limit grows back

## Determinism caveat

The exact peak in-flight observed by the task depends on dispatch timing within one refill (up to
~workers steps can be admitted before any has a chance to 429). The exactly-assertable properties
are kept exact: no flow fails, at least one 429 fires, all flows eventually complete. The peak value
itself is bounded statistically by `workers + ~peerCount`; the test asserts the upper bound, not an
exact value.
