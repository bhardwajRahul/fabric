# perelementpipelineflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **per-element pipeline** pattern, which is one of the headline cases the depth-based foreman cannot execute correctly. The graph is `S -> forEach(items) -> H -> {A, B} -> M -> L`. Each forEach element should run its own `H -> {A,B} -> M` pipeline producing one M_k per element, and L consolidates all M_k results.

Under the depth-based fan-in, the forEach instances collapse at the first fan-in (one shared H, one shared M, one shared L). So this graph completes but produces a single fused result rather than N independent results. The relevant test is `t.Skip`'d with reference to `_DOMINATOR.md` (lineage redesign).

## Patterns exercised when the redesign lands

- Dynamic fan-out (forEach) followed by per-thread inner pipeline
- Inner fan-out per thread (`{A, B}`)
- Inner fan-in per thread (M_k)
- Outer fan-in across threads at L (via `set*` reducer in this fixture, since list ordering would be racy)

## Why this service exists pre-redesign

Building the graph, tasks, and test scaffolding now means the lineage redesign only needs to remove the `t.Skip` to validate. No new scaffolding work at that time.
