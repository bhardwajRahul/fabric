# nestedfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **two-level nested fan-out** via the subgraph escape hatch. The outer graph is `A -> {NormalB, Inner-subgraph} -> J`. The inner subgraph itself has an internal fan-out: `X -> {Y, Z} -> W`. So the workflow exhibits fan-out at the outer level and fan-out inside one of the siblings.

The reason this fixture nests via a subgraph instead of pure graph nesting: the foreman's fan-out sibling constraint requires all siblings at a fan-out depth to share the same downstream targets. A pure-graph "inner fan-out per outer sibling" would violate that. Subgraphs sidestep the constraint by isolating each inner pipeline behind its own graph.

## Patterns exercised

- Outer fan-out (2 siblings: NormalB + Inner subgraph)
- Inner fan-out inside the subgraph (X -> {Y, Z} -> W)
- `sum*` reducer applied at the inner fan-in (Y and Z each contribute deltas)
- Subgraph output (innerResult) merged back into the outer step's changes
- Outer fan-in at J combining NormalB's output with the subgraph's output
