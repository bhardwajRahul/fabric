# ackdroppedflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's per-task 404 ack-timeout breaker. The test deactivates the
`Park` task's subscription before any flows are created, so dispatch to Park returns
`404 "ack timeout: ..."` from the connector. The first such 404 trips the breaker, which thereafter
gates all dispatch for the `park` task name to one probe per refill cycle per replica. Meanwhile
the `Ping` task's subscription stays on-bus, exercising the property that the breaker is per
*task name* (not per microservice) - Ping flows must continue to drain unimpeded while Park is
gated.

Recovery is also exercised: the test reactivates Park's subscription, the next probe succeeds,
the breaker reopens locally, and the parked AckDropped flows drain through normal refill.

## Patterns exercised

- `connector.DeactivateSubscription` / `ActivateSubscription` to simulate a downstream that is
  intermittently unreachable, producing the canonical "ack timeout" 404 the breaker hooks on
- Trip-after-first: any single 404 trips the breaker for that task name
- Breaker is the gate; per-step `not_before` is irrelevant while tripped
- Unrelated-task isolation: a tripped breaker on `park` does not gate `ping`
- Probe success reopens the breaker locally (no gossip; opens are silent)

## Determinism caveat

The exact number of probe attempts that fire during the trip window depends on refill cadence and
the exponential schedule (100ms, 200ms, 400ms, ... up to 1m). The test asserts the strong shape
properties (breaker tripped while Park is off-bus, Echo unaffected, AckDropped drains after
reactivation) and does not pin probe counts.
