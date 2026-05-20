# adaptiveconcurrencyflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's CUBIC recovery curve. The other backpressure fixtures
(`maxconcurrencyflow`, `saturatedbandflow`, `distributedbackpressureflow`) verify that 429s are
observed, cut, gossiped, bounced, and skipped correctly. This one verifies the missing temporal
property: that the per-task valve's limit actually *grows back* over time after a cut, so a
downstream that gains capacity (autoscale, scale-up) is eventually re-utilized without operator
intervention.

The task exposes a runtime-mutable cap. The test runs two phases:

1. **Phase 1: low cap.** The cluster oversubscribes the cap, 429s fire, the regulator cuts the
   valve, the cluster settles to admit at or near the low cap. Most flows complete via the
   bounce-and-retry path.
2. **Recovery sleep.** The test waits long enough for the CUBIC curve to grow the limit well above
   the phase 1 cap (a few seconds is enough at `cubicC = 0.05`).
3. **Phase 2: high cap.** The cap is raised mid-test. The same task instance now admits up to the
   higher number of concurrent in-flight. The test asserts the cluster's *actually-observed* peak
   in phase 2 exceeds phase 1's cap - which is only possible if the regulator's limit recovered
   above that cap during the sleep.

The fixture deliberately does not assert exact recovery times or limit values; those are sensitive
to `cubicBeta`, `cubicC`, and timing jitter. The load-bearing assertion is the qualitative one:
"the controller noticed the new capacity."

## Patterns exercised

- The TCP CUBIC recovery curve `w(t) = cubicC*(t-K)^3 + wMax` actually running over wall-clock time
- The valve's `tCong` anchor persisting between phases (it doesn't reset just because cuts stopped)
- The regulator successfully re-cutting when phase 2's cap is later hit (verified indirectly via
  `TaskValveCount() >= 1` post-test)

## Determinism caveat

This fixture is fundamentally temporal and statistical. Exact peak counts in either phase depend on
worker-pool size, dispatch timing, and how aggressively CUBIC's convex region grows. The
assertions are *shape* assertions: phase 1 saw cuts, phase 2 admitted more than phase 1's cap, all
flows completed. Tight numeric assertions on recovery time would flake.
