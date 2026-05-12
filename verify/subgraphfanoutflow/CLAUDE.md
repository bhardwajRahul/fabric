# subgraphfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **a subgraph as one sibling of an outer fan-out**. The graph is `A -> {NormalB, Sequential subgraph, NormalD} -> E`. The subgraph is sequential (`X -> Y`, no internal fan-out) and runs alongside two normal task siblings, all converging at E.

This is distinct from `nestedfanoutflow.verify` (where the subgraph itself contains a fan-out). Here the subgraph is just a multi-step pipeline that happens to be one branch of the outer fan-out.

## Patterns exercised

- Outer static fan-out with mixed normal/subgraph siblings
- Subgraph as a "sibling" satisfying the fan-out validator's same-target constraint
- `DeclareOutputs` on the subgraph scoping which fields cross back into the parent
- Outer fan-in at E with state contributions from each sibling
