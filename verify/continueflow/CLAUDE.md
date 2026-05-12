# continueflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for multi-turn flows via `Continue`. A single-task workflow increments a counter. The first turn produces `counter=1` from a starting `counter=0`. `Continue` creates a new flow in the same thread with the prior final state merged in; the second turn produces `counter=2`.

## Patterns exercised

- `foremanapi.Client.Continue(threadKey, additionalState)` creating a new flow in an existing thread
- State persistence across turns via `DeclareInputs(counter)` + `DeclareOutputs(counter)` on the same field
- Thread identity carried via the original flowKey acting as a threadKey

## Important convention

Fields that must persist across turns are declared as **both** inputs and outputs of the workflow. Other state fields are dropped when a turn completes.
