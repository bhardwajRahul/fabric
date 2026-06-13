# interruptedfanoutflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for interrupt/resume of one branch within a static fan-out. The graph is
`Src -> {A, B, C} -> J`. A and C succeed (contributing 1 each to the Add-reduced `executed`
field). On its first run B calls `flow.Interrupt` and contributes nothing, so the flow parks
in `interrupted` (the fan-in gate never reaches all three arrivals). The test detects the
interrupted status, calls `Resume(flowKey, {"resumed": true})`, and B re-runs with
`resumed=true` in its state, falls through, and contributes 1. Once all three branches have
arrived, the fan-in fires, J sees `executed=3`, and the flow completes.

## Patterns exercised

- Static fan-out (`Src` -> three unconditional transitions) with explicit `SetFanIn`
- One fan-out branch parking the flow via `flow.Interrupt` while siblings proceed
- Resume injecting the re-entry signal into the parked branch's state (the `resumed` input,
  same idiom as `interruptflow`'s AwaitInput)
- Manual flow driving: `Create` -> `Start` -> `Await` (interrupted) -> `Resume` -> `Await`
  (completed), since the synchronous Executor cannot resume

## Deterministic assertions

- the first `Await` returns `foremanapi.StatusInterrupted`
- after `Resume`, the second `Await` returns `foremanapi.StatusCompleted`
- `executed == 3` (A + B + C, each delta 1, summed by ReducerAdd at fan-in)
- `totalExecuted == 3` (J surfaced the summed value, proving the fan-in saw 3)

B always interrupts on its first run and the fan-in gate strictly requires all three arrivals,
so the interrupted-then-completed outcome and the executed count of 3 are fully deterministic.
