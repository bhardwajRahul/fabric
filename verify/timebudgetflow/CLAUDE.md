# timebudgetflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for per-task time budgets via `graph.SetTimeBudget`. The graph is `A -> Slow`. TaskSlow `time.Sleep`s for longer than its configured budget; the foreman cancels the HTTP request when the budget elapses and the step fails. With no `onError` transition, the flow itself fails.

## Patterns exercised

- `graph.SetTimeBudget(taskURL, duration)` setting a per-task budget
- Foreman applying the budget as `pub.Timeout` on the task dispatch HTTP call
- Step failure on timeout
- Flow failure when no error transition catches the timeout
