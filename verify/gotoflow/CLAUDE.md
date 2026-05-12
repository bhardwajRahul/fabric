# gotoflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for runtime `flow.Goto` and graph-time `AddTransitionGoto`. The graph is `A -> B -> C` with `B -> withGoto -> A`. TaskB inspects the `loops` counter and, while it is below the target, calls `flow.Goto(taskA)` to loop back. Once the counter reaches the target, B falls through to C.

## Patterns exercised

- `AddTransitionGoto` declaring a runtime-targeted edge
- `flow.Goto(taskURL)` at runtime
- Read-modify-write of a counter state field via the `Out` suffix
- Verifying the goto target is one of the registered `withGoto` transitions (the foreman rejects goto to an unregistered target)
