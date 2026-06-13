# subgraphentryflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **a subgraph call at both the entry and the last position of a workflow graph**. The outer
graph is `runInner -> runTail -> END`; both `runInner` and `runTail` are regular tasks that call `flow.Subgraph`.
This is the minimal coordinator-shape graph, in which leaf workflows are chained directly via subgraph calls.

The fixture pins that a `flow.Subgraph`-calling task can sit at the entry-point position, and that state a subgraph
caller adopts crosses into the parent flow and feeds the next subgraph caller.

## Patterns exercised

- A `flow.Subgraph` caller task as the entry point of the outer graph
- A `flow.Subgraph` caller task as the last non-`END` node of the outer graph
- State flowing from one subgraph caller into the next (the first adopts `innerResult`, which crosses into the second's child)
- Sequential `caller -> caller -> END` chain with no fan-out
