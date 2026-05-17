# perelementpipelineflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **per-element pipeline** pattern. The graph is
`S -> forEach(items) -> H -> {A, B} -> M -> L`. Each forEach element runs its own independent
`H -> {A, B} -> M` pipeline producing one per-element result, and L consolidates the results across all
elements.

The lineage-based fan-in scopes each element's inner `{A, B} -> M` fan-in to that element's own spawn
cohort (`graph.SetFanIn("taskM")`), and the fan-in across elements is a second cohort
(`graph.SetFanIn("taskL")`). Because a cohort is identified by spawn lineage rather than `step_depth`, the
N inner pipelines stay independent. An earlier depth-based fan-in collapsed them - one shared H, one shared
M, one shared L - into a single fused result instead of N. The test runs 3 elements and asserts the flow
completes with `finalCount == 3`; a collapsed pipeline would yield 1, so this is the regression guard for
per-element pipeline independence.

## Patterns exercised

- Dynamic fan-out (`forEach`) followed by a per-element inner pipeline
- Inner fan-out per element (`{A, B}`) with inner lineage fan-in at `taskM`
- Outer lineage fan-in across elements at `taskL`
- `set*` union reducer consolidating per-element results
