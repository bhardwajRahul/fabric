# errorflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `onError` transitions in a sequential (non-fan-out) flow. The graph is `A -> B -> C` with `B onError -> Handler -> C`. When TaskB succeeds the normal path runs; when it errors the handler runs and the flow still reaches C.

## Patterns exercised

- `graph.AddTransitionOnError` routing
- Error handler receives the `onErr *errors.TracedError` argument
- Flow does not fail when the error is handled
- Both success path and error path converge at C
