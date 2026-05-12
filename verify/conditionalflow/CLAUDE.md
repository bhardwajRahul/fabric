# conditionalflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `AddTransitionWhen` (boolean-expression transitions). The graph is `A -> {B-high (when score>=50), B-low (when score<50)} -> C`. Based on input score, exactly one of the conditional branches is taken; the other transition's `when` evaluates false and is not followed.

## Patterns exercised

- `AddTransitionWhen` with mutually-exclusive `when` expressions
- Both branches converge at C (single fan-in target, satisfies the validator's "same outgoing target" constraint)
- Verification that only one branch runs per execution
