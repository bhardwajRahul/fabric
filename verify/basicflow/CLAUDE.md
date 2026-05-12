# basicflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the simplest workflow shape: a sequential chain `A -> B -> C`. Each task reads a string `path` field, appends its own letter, and writes it back. The final state's `path` is `"ABC"`.

## Patterns exercised

- Sequential task execution (`graph.AddTransition` chain)
- Read-modify-write of a single state field via the `Out` argument suffix
- Workflow termination at `workflow.END`
