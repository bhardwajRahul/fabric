# dynamicfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for dynamic fan-out via `forEach`. The graph is `A -> forEach(items) -> B -> C`: TaskA emits a list of items, one TaskB instance runs per element, and TaskC sums the per-element results.

## Patterns exercised

- Dynamic fan-out (`graph.AddTransitionForEach`)
- `sum*` reducer convention (fan-in numeric add)
- forEach over a 0-element array (flow completes at the forEach source without firing the fan-in target)
- forEach over a 1-element array
