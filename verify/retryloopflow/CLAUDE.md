# retryloopflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **OnError retry loop** pattern: `A -> B -> C` where `B onError -> Handler -> B` (cycle back). The handler increments an `attempts` counter and routes back to B. B succeeds once `attempts == target`. The flow then proceeds to C.

The graph is expressible today via `AddTransition` + `AddTransitionOnError`, but the depth-based foreman cannot execute it reliably: the retry path makes B's depth different on each iteration, so C ends up at a varying step_depth depending on how many retries happened. The graph also has a normal-edge cycle (Handler -> B), and the runtime needs lineage-based fan-in to handle the join correctly.

The test is `t.Skip`'d pending the lineage redesign described in `_DOMINATOR.md`.

## Patterns exercised when the redesign lands

- `AddTransitionOnError` with a back-edge to the originating task
- State persistence across retry iterations via the `Out` argument suffix (attempts read-modify-write)
- Sequential cycle that terminates when a condition is met (attempts reaches target)
- Validator allows normal-edge cycles (only `withGoto` cycles are special-cased)

## Why this service exists pre-redesign

Building the graph, tasks, and test scaffolding now means the lineage redesign only needs to remove the `t.Skip` to validate.
