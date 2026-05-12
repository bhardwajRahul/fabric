# forkflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `Fork`. The workflow is `A -> B -> C` where:

- TaskA passes `value` through
- TaskB doubles `value`
- TaskC adds 1 to `value`

The test runs the workflow once to completion, retrieves its step history, picks the `stepKey` of TaskB, and calls `Fork(stepKey, {"value": 100})` to create a new flow whose B step is re-created with `value=100`. Running the forked flow advances `value` through B (->200) and C (->201).

## Patterns exercised

- Running a flow to completion and reading `History`
- Identifying a specific step by `StepKey`
- `Fork(stepKey, stateOverrides)` creating a derived flow with overridden state at the fork point
- Forked flow runs independently to completion with the overridden value
- Demonstrating that both original and forked flows produce correct, distinct outputs
