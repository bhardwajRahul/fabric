# sleepflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `flow.Sleep`. The graph is `A -> B -> C`. TaskB calls `flow.Sleep(duration)` to push the next step's `not_before` into the future. The foreman's `pollPendingSteps` timer adapts and eventually dispatches TaskC after the delay elapses.

## Patterns exercised

- `flow.Sleep(duration)` setting `not_before` on the next step
- Foreman's adaptive poll interval waking when the sleep expires
- Workflow completes after the configured delay

## Test approach

The test measures wall-clock time across the workflow call and asserts the elapsed time is at least the configured sleep duration. Uses a short sleep (50ms) to keep tests fast.
