# fanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for static fan-out + fan-in. The graph is `A -> {B, C, D} -> E`: TaskA fans out to TaskB/C/D in parallel, and they all converge at TaskE which checks that each branch executed.

## Patterns exercised

- Static fan-out (multiple `graph.AddTransition` calls from the same source)
- Static fan-in (multiple `graph.AddTransition` calls to the same target)
- Each branch writes to its own state field; TaskE reads all three to verify all branches ran
