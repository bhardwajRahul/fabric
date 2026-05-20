# distributedbackpressureflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's adaptive concurrency under multi-replica and multi-shard
deployment. Two foreman replicas share the same SQLite shards (via the cache=shared in-memory DSN
suffixed identically by OpenTesting because both replicas share the test's NATS plane), so they
observe each other's `microbus_steps.status='running'` writes across both shards. The Bounded task
self-emits 429 above its cap; the foreman replicas' `runRefill` paths must aggregate `SUM(running)`
across every shard each iteration to produce a coherent cluster-wide saturation verdict, and a 429
on one replica must propagate to the other via the `SyncValve` multicast gossip so both
replicas converge on the same `(wCong, tCong)` anchor.

The test asserts that all flows complete (no failure mode caused by cross-replica or cross-shard
state inconsistency), that rejections fired (the controller did exercise the 429 path), and that
both shards saw work (the foreman is not silently funneling every flow to one shard).

## Patterns exercised

- Multiple `foreman.NewService()` instances sharing in-memory SQLite per shard via OpenTesting
  uniqueness keyed by `(plane, shard)`; both replicas share the plane derived from the test name
- `NumShards > 1` driving cross-shard `SUM(running)` aggregation in `computeRunningByTask` and
  `countStepsByTask`
- `SyncValve` multicast: a 429 on one replica gossips the cut to the other
- `Sonar`'s synchronous OnStartup ping seeding `peerCount` at startup

## Determinism caveat

Exact dispatch ordering is non-deterministic across two replicas racing on shared shards. The
load-bearing assertions are end-to-end and shape-only: completion of every flow, the existence of
at least one 429, and the presence of work on each shard. The peak concurrent in-flight observed by
the task is bounded statistically by `workers * numReplicas` (the absolute hard cap before any cut
can take effect); the test asserts the upper bound, not an exact value.
