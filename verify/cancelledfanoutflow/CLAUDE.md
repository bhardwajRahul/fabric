# cancelledfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for cancelling a flow while a fan-out branch is mid-execution. The graph is
`Source -> {A, B, C} -> J`. The foreman is pinned to a single worker in the test
(`foreman.NewService().Init(func(svc) error { return svc.SetWorkers(1) })`). Source fans out
instantly; A/B/C each record their entry in an atomic counter and then block for `branchSleep`
(2s). The test starts the flow, waits 1s, cancels it, then awaits.

With one worker the lone worker is blocked inside the first branch's sleep across the cancel, so
the other two branches are cancelled while still `pending` and never run; J never runs.

## Patterns exercised

- Static fan-out (`Source` -> three unconditional transitions) with explicit `SetFanIn`
- Cancelling a `running` flow mid-branch via `foremanapi.Client.Cancel`
- Foreman `Workers` config pinned to 1 via `.Init` (SetConfig is only permitted once the
  connector is in the TESTING deployment, which is why a bare post-creation `SetWorkers` fails
  and the `.Init` hook is used instead)
- Manual flow driving (`Create` -> `Start` -> sleep -> `Cancel` -> `Await`) rather than the
  synchronous Executor, so the cancel lands mid-flight

## Deterministic assertions

- final status is `cancelled`
- the atomic execution counter is exactly 1 (only the branch the single worker picked up ran)
- `sumExecuted` is 0/absent in the final state (the running branch's result was discarded and
  the fan-in never aggregated)

The 1s cancel vs 2s branch sleep gives a comfortable margin for the in-memory test harness.
