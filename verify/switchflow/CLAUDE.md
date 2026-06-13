# switchflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for `AddTransitionSwitch` (first-match-wins routing). Two graph endpoints:

- **Switch** - `Router -> {HandleHigh (amount>=10000), HandleMid (amount>=1000), HandleLow (true)} -> END`. Exactly one branch fires per execution; the default arm uses `when="true"` so every input lands somewhere.
- **SwitchNoMatch** - same router and the same first two predicates, but the final arm is `when="false"` instead of `"true"`. Any input that does not satisfy `amount>=10000` or `amount>=1000` falls off the end of the switch ladder, so no successor is created and the flow completes at the router with no `branch` field set.

## Patterns exercised

- `AddTransitionSwitch` declares the routing primitive
- `when="true"` as the terminal default arm (Switch graph)
- No-match-ends-flow path (SwitchNoMatch graph) — the source step still completes and the flow transitions to `completed`, the side-channel `branch` field is just absent
- No `SetFanIn` anywhere; the validator accepts the graph because Switch transitions do not count as fan-out
