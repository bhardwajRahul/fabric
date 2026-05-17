# retryloopflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **OnError retry loop** pattern: `A -> B -> C` where `B onError -> Handler -> B`
(cycle back). Handler increments an `attempts` counter and routes back to B; B succeeds once
`attempts == target`, then the flow proceeds to C.

B's `step_depth` differs on every iteration, so the position of C is depth-varying - the runtime drives the
join and termination by execution-DAG edges and lineage, not `step_depth`, so the cycle terminates and C
runs exactly once at the right point regardless of retry count. The graph validator permits this normal-edge
cycle (`Handler -> B`); only `withGoto` cycles are special-cased. The test runs target 3 and asserts the
flow completes with `finalAttempts == 3`.

## Patterns exercised

- `AddTransitionOnError` with a back-edge to the originating task
- State persistence across iterations via the `Out` argument suffix (attempts read-modify-write)
- Sequential cycle that terminates when a condition is met (attempts reaches target)
- Validator allows normal-edge cycles (only `withGoto` cycles are special-cased)
