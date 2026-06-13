# subgraphflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for subgraph invocation via `flow.Subgraph`. The parent workflow is `A -> RunInner -> Z`. The
`RunInner` task calls `flow.Subgraph(Inner.URL())` to run the inner subgraph `X -> Y`, then adopts the child's
`innerResult` output and returns it; `Z` reads `innerResult` and surfaces the final result.

This is how the foreman expresses nested workflows: an inner pipeline is run as a child flow via `flow.Subgraph`, the
caller task scoping exactly which of the child's outputs cross back into the parent.

## Patterns exercised

- `flow.Subgraph(url, input)` invoked from a regular caller task (`RunInner`)
- The caller adopting a specific child output (`innerResult`) from the returned `out` rather than an auto-merge
- Park-on-first-call, re-run-with-result re-entry (the `if yield { return }` guard)
- The child output flowing into a downstream task (`Z`) via the caller's typed output

## Two workflows

This service hosts two workflows that reference each other:

- `Parent` (the outer workflow): `A -> RunInner -> Z`, where `RunInner` calls `flow.Subgraph(Inner.URL())`
- `Inner` (the inner workflow, used as a subgraph): `X -> Y`

The same service hosts both so the cross-workflow reference stays inside one process for testing.
