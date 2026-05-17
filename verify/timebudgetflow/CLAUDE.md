# timebudgetflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for endpoint-declared time budgets via `sub.TimeBudget`. The graph is `A -> Slow` and
carries no timing. The `Slow` task endpoint declares a 50ms budget on its own subscription
(`sub.TimeBudget(50*time.Millisecond)`); its handler sleeps 500ms. The connector shortens the inbound
context deadline to the declared budget, the handler observes `ctx.Done()` and returns `ctx.Err()`, and
the foreman surfaces that as a step failure with status 408. With no `onError` transition, the flow itself
fails.

## Patterns exercised

- `sub.TimeBudget(duration)` declaring a per-endpoint budget on a task subscription
- The connector shortening the inbound handler context deadline to the declared budget
- The foreman's `TimeBudget` config acting as the (looser) ceiling on the task dispatch call
- Step failure on timeout (408)
- Flow failure when no error transition catches the timeout
