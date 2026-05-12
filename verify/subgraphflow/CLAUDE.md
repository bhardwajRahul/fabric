# subgraphflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for subgraph invocation. The parent workflow is `A -> [Inner subgraph] -> Z`. The inner subgraph is `X -> Y` and produces a result that is merged back into the parent's state. The parent then surfaces the merged result.

This is how the depth-based foreman expresses nested workflows: an inner fan-out (or any multi-step internal pipeline) is wrapped in its own subgraph, isolating its internal depth from the parent's.

## Patterns exercised

- `graph.AddSubgraph` registration
- `graph.AddTransition(parent, subgraph)` for subgraph as a step
- `DeclareInputs` and `DeclareOutputs` on the inner subgraph to scope which state crosses the boundary
- Subgraph child flow's `final_state` filtered through `DeclareOutputs` and merged into the parent step's `changes`
- Parent flow's lifecycle waits for the subgraph to complete

## Two workflows

This service hosts two workflows that reference each other:

- `Parent` (the outer workflow): `A -> Inner subgraph -> Z`
- `Inner` (the inner workflow, used as a subgraph): `X -> Y`

The same service hosts both so the cross-workflow reference stays inside one process for testing.
