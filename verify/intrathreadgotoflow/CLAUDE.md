# intrathreadgotoflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **intra-thread `flow.Goto` inside a fan-out region**. The outer graph is `A -> {LoopTask, NormalC} -> D`. LoopTask uses `flow.Goto(LoopTask)` to loop back to itself until a counter reaches its target; NormalC just produces a value. Both branches converge at D.

The intra-thread Goto in LoopTask's branch increments LoopTask's step_depth each iteration, while NormalC's branch stays at one depth. The fan-in at D needs to wait for both branches at the *same* effective convergence point, which the depth-based model can't represent. The lineage-based model handles this naturally: LoopTask's repeated runs all carry the same `fan_out_id` from the outer fan-out, and the fan-in checks the lineage cohort rather than the depth.

The test is `t.Skip`'d pending the lineage redesign described in `_DOMINATOR.md`.

## Patterns exercised when the redesign lands

- Outer fan-out with one sibling using `flow.Goto` to self-loop
- The other sibling proceeds normally and waits at the convergence
- Lineage-based fan-in coordinates across branches with different depth counts

## Why this service exists pre-redesign

Building the graph, tasks, and test scaffolding now means the lineage redesign only needs to remove the `t.Skip` to validate.
