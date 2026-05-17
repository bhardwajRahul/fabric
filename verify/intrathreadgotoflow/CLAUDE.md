# intrathreadgotoflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for **intra-thread `flow.Goto` inside a fan-out region**. The outer graph is
`A -> {LoopTask, NormalC} -> D`. LoopTask uses `flow.Goto(LoopTask)` to loop back to itself until a counter
reaches its target; NormalC just produces a value. Both branches converge at D.

The Goto in LoopTask's branch advances its `step_depth` on every iteration while NormalC's branch stays at
one depth, so the two branches reach D at different depths. The lineage-based fan-in coordinates the
convergence by spawn cohort, not `step_depth`: LoopTask's repeated runs all carry the same lineage from the
outer fan-out, so the fan-in fires when the cohort is complete regardless of per-branch depth. This is also
the exact shape that broke the old `MAX(step_depth)` terminal-state heuristic - the looping branch outran
the real terminal step in depth - which the execution-DAG tail merge (`successor_id = 0`) now handles
depth-agnostically. The test runs target 3 and asserts the flow completes with `final == "stamped/3"`,
encoding that LoopTask iterated to the target and converged with NormalC at D.

## Patterns exercised

- Outer fan-out with one sibling using `flow.Goto` to self-loop
- The other sibling proceeds normally and waits at the convergence
- Lineage-based fan-in coordinating branches with different depth counts
- Depth-agnostic terminal-state merge across an intra-fan-out Goto loop
