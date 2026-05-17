# shardedflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

A dedicated, simple fixture for the foreman's **cross-shard global** priority+fairness aggregation, run
with `NumShards = 8` and a single worker. The graph is the single task `record -> END` (cloned from
`priorityflow`); `record` sleeps a small delay and appends its tag to a service-side order slice. Because
top-level `Create` picks a random shard, the test flows scatter across all 8 shards - so this fixture
verifies that the refiller aggregates *all shards* into one global population (global-min priority band,
fairness over `fairness_key` not over shards) rather than the old per-shard round-robin, which would
have broken priority ordering and skewed weighted fairness under shard scatter.

## Subtests

- `strict_priority_across_shards` (deterministic): a priority-1 holder occupies the lone worker while 8
  test flows with **distinct** priorities (2..9) are created and scattered across the 8 shards. The
  recorded order must be exactly priority-ascending. Distinct priorities mean each band has exactly one
  flow, so global-min selection is fully deterministic regardless of which shard each landed on or
  `created_at` millisecond ties - no intra-band tiebreak is exercised.
- `starvation_across_shards` (deterministic on the cross-band property): a priority-1 holder, one
  priority-9 low flow, and 8 priority-2 high flows, all scattered. Asserts every high precedes the low
  (cluster-wide strict priority across shards). Order *among* the same-priority highs is not asserted
  (it is `created_at` then `(shard, step_id)`, not creation order, under shard scatter).
- `weighted_fairness_over_keys_not_shards` (statistical): many flows across two weighted keys at one
  priority, scattered across 8 shards. Asserts the heavy:light dispatch share over the contended prefix
  approximates the weight ratio (~4:1) and the light key makes progress. This is the property the
  cross-shard aggregation delivers; the old per-shard round-robin would skew this toward ~1:1 because
  each shard got an equal turn regardless of key weight.
- `random_shard_distribution` (statistical): creates many flows (not started, so they stay inert) and
  parses the shard from each flowKey's `{shard}-...` prefix. Asserts every one of the 8 shards is used
  and the per-shard counts are approximately uniform (generous ~5-sigma bands around the mean), confirming
  `Create`'s `rand.IntN(NumShards)` actually scatters flows - which is what makes the cross-shard
  subtests above a real test rather than a single-shard one in disguise.
- `fifo_within_fairness_key` (deterministic): 4 fairness keys x 8 flows, all at one priority band, created
  in a randomly shuffled interleave and scattered over the 8 shards. The weighted key pick randomizes order
  *across* keys (the global recorded order is not creation order), but the refiller always takes a key's
  oldest pending step, so the recorded order **projected onto each key** must equal that key's creation
  order - FIFO per key. This is the realistic path (cross-key contention) and still proves cross-shard FIFO
  by per-shard `ageMs` (`DATE_DIFF_MILLIS(NOW_UTC(), created_at)`, offset-cancelling); a `(shard, step_id)`
  order would interleave a key's steps by shard and fail the per-key projection. Flows are created `>1`
  DATETIME(3) tick apart (12ms) so within every key `created_at` is strict (no same-ms tie, so the
  `(shard, step_id)` tiebreak never engages); a priority-1 holder occupies the lone worker through the
  whole creation window so nothing drains until all flows are pending.

## Determinism caveat

The deterministic subtests use **distinct priorities** (A), assert only the **cross-band** property (B), or
**space `created_at` by more than one DATETIME(3) tick** (C, `fifo_within_fairness_key`) so they never
depend on a same-millisecond intra-band tiebreak. Intra-band order across shards is `created_at` (per-shard
`ageMs`) then `(shard, step_id)`; only the `created_at` term is creation-ordered, so (A)/(B) avoid it
entirely and (C) makes the `created_at` term strict (no tie, so the `(shard, step_id)` fallback never
runs). The fairness subtest is statistical (like `fairnessflow`). `NumShards > 1` requires a
`%d`-templated `SQLDataSourceName`; in TESTING this still resolves to an isolated in-memory SQLite per
(shard, test). Priorities are >= 1 (0 means unset in `FlowOptions`); the holder uses priority 1.
