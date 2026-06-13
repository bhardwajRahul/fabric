# subgraphfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **a subgraph call as one sibling of an outer fan-out**. The graph is `A -> {NormalB, RunSub, NormalD} -> E`, where `RunSub` is a task that calls `flow.Subgraph(Sub.URL())`. The subgraph is sequential (`X -> Y`, no internal fan-out) and runs alongside two normal task siblings, all converging at E.

This is distinct from `nestedfanoutflow.verify` (where the subgraph itself contains a fan-out). Here the subgraph is just a multi-step pipeline that happens to be one branch of the outer fan-out.

## Patterns exercised

- Outer static fan-out with mixed normal/subgraph-caller siblings
- A `flow.Subgraph` caller task as a fan-out "sibling" satisfying the validator's same-target constraint
- RunSub adopting the subgraph's `subResult` output and returning it as its own output
- Outer fan-in at E with state contributions from each sibling
