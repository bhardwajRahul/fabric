# retryflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `flow.Retry`. The graph is `A -> Flaky -> B`. Flaky reads an `attempts` counter from state, increments it, and calls `flow.Retry(5, 0, 0, 0)` until `attempts` reaches `target`. If retries are exhausted (Retry returns false), Flaky returns an error and the flow fails.

The `attempts` field is read-modify-write across retries: on each retry, the foreman merges the previous attempt's `changes` (which includes `attemptsOut`) back into `state`, so the next attempt observes the updated counter via `ParseState`.

## Patterns exercised

- `flow.Retry(maxAttempts, initial, multiplier, maxDelay)` returning true/false
- State persistence across retries (the read-modify-write pattern via the `Out` argument suffix)
- Successful retry path: flow reaches completion
- Exhaustion path: task returns error, flow fails

## Hardcoded cap

The task uses `flow.Retry(5, 0, 0, 0)` — max 5 attempts, no delay. Tests choose `target` relative to that cap to exercise both successful retry and exhaustion.
