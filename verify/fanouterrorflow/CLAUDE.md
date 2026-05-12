# fanouterrorflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the OnError + fan-out interaction. The graph is `A -> {B, C, D} -> E` with `B onError -> Handler -> E`. When TaskB errors, the foreman cancels its siblings (C and D) and routes through Handler instead. This is the path that historically had a race condition where a sibling finishing simultaneously with the sibling-cancel would mistakenly call `failStep` and fail the parent flow. The race was fixed in `coreservices/foreman/service.go:2596` (fan-in worker returns nil on failed/cancelled siblings rather than escalating).

## Patterns exercised

- Fan-out with one branch defining `onError`
- Sibling-cancel triggered by the errored task
- Convergence at E via the Handler path
- Stress test: repeated execution (via `-count=N`) exposes the race if it returns

## Important assertion

The test runs the workflow many times to surface the OnError race if it ever regresses. If foreman's fix is reverted or the design changes, this test will go flaky.
