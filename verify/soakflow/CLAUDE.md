# soakflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

A high-volume **liveness soak** for the foreman dispatcher. Unlike the targeted feature fixtures (which run
one or a few flows with fixed inputs to assert a specific structural outcome), this fixture creates a large
number of flows with **random inputs** through a single **complex, input-driven** workflow and asserts only
that **every flow reaches a terminal status** - none stuck in `created`/`pending`/`running`. It is the
standing regression guard for dispatch-loop liveness bugs (e.g. a refiller that wedges under many concurrent
independent pending flows).

## The workflow

`Soak` clamps the random inputs in `seed`, then a mutually exclusive 5-way `when` split on `branch` selects
one route, all converging at `join -> END` except the unhandled failure:

- `branch==0`: dynamic `forEach` fan-out over an input-sized array, with an Add-reduced `work` field at the fan-in (`collect`).
- `branch==1`: a bounded `flow.Goto` self-loop (`loop`), iteration count from the input.
- `branch==2`: a `flow.Subgraph` caller task (`RunSub`) invoking the `Inner` subgraph.
- `branch==3`: a task that always errors, recovered via an `onError` transition (flow completes).
- `branch==4`: a task that always errors with no error transition (flow **fails** - still terminal).

Tasks are intentionally trivial (no sleeps): the graph's shape, not task work, is what stresses the
dispatcher, and fast tasks maximize the volume exercised in the soak window. All paths terminate by
construction (the loop and the fan-out width are bounded by `seed`'s clamps).

## Patterns exercised

- Conditional `AddTransitionWhen`, dynamic `AddTransitionForEach` + `SetFanIn` + explicit `SetReducer(work, Add)`, bounded
  `AddTransitionGoto`, a `flow.Subgraph` caller task, `AddTransitionOnError`, and unhandled-error termination - all under
  high concurrent volume.
- The priority/fairness dispatch path: candidate cache, single-slot refiller, doorbell/head-insert, the
  combined band+candidate query, and the deep priority/fairness subquery insert paths (via the subgraph and
  fan-out), with `NumShards > 1` to exercise the cross-shard global aggregation and weighted batch assembly.

## Soak / determinism caveat

The assertion is **liveness only**: every created flow ends `completed`, `failed`, or `cancelled` within a
generous deadline; ordering and outputs are not asserted (consistent with the soft, throughput-favoring
scheduler). Inputs are drawn from a seeded RNG and the seed is logged, so any failure is reproducible by
re-running with that seed. The test runs the foreman with `NumShards > 1` and a small worker pool (so the
candidate cache is the binding constraint - the regime where dispatch wedges surface) and creates flows for
a fixed wall-clock window, then drains and verifies. It is a stress/liveness test, not an exact-behavior
fixture, and must not assert dispatch order or per-flow output.
