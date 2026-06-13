# continueflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for multi-turn flows via `Continue`. A single-task workflow increments a counter. The first turn produces `counter=1` from a starting `counter=0`. `Continue` creates a new flow in the same thread with the prior final state merged in; the second turn produces `counter=2`.

## Patterns exercised

- `foremanapi.Client.Continue(threadKey, additionalState)` creating a new flow in an existing thread
- State persistence across turns via `Continue`'s unfiltered carryover of the prior turn's `final_state`
- The read-modify-write `Out`-suffix pattern (`Increment` reads `counter`, returns `counterOut`)
- Thread identity carried via the original flowKey acting as a threadKey

## Important convention

`Continue` carries the prior turn's `final_state` through to the next turn unfiltered, so any field present at the end
of one turn (here, `counter`) is visible at the start of the next. There is no per-field input/output declaration; a
workflow author who wants narrower turn-to-turn carryover puts an adapter task at the workflow's entry that scrubs
state with `flow.Delete`/`Transform`.
