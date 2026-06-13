# superflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

A single unified verification fixture that exercises every workflow transition primitive in one graph. The
intent is to provide a parametric test surface for behaviors that are *shape-orthogonal*: sleep, retry,
interrupt, error routing, goto, conditional, subgraph, fan-out/fan-in. Tests inject per-task behavior
through the workflow state's `behaviors` map and assert on per-task visit counters carried on the service.

## Why not many small fixtures

The existing per-behavior fixtures (basicflow, fanoutflow, conditionalflow, etc.) isolate one foreman
behavior each and remain valuable as readable per-behavior documentation. `superflow` is *additive*: it
covers the cross-product cases those single-shape fixtures can't — sleep mid fan-out branch, goto crossing
a conditional, error inside a forEach element, retry through a subgraph child. The single-purpose fixtures
keep their narrative purpose; this one provides coverage breadth.

## Graph shape

The `Super` workflow:

```
TaskA -> TaskB                                  sequential
TaskB -> forEach items as item -> TaskC         dynamic fan-out
TaskC -> TaskD                                  fan-in cohort exit
TaskC -> OnError -> ErrorHandler -> TaskD       sibling-cancel + handler rejoin
SetFanIn(TaskD)                                 forEach cohort fan-in
TaskD -> when useSubgraph == true: SuperSubCall (flow.Subgraph caller)
TaskD -> when useSubgraph != true: TaskE        conditional alternate
SuperSubCall -> TaskE
SetFanIn(TaskE)                                 conditional fan-in
TaskE -> withGoto "taskZ": TaskZ                goto target
TaskE -> END                                    default
TaskZ -> END
```

The nested `SuperSub` workflow is a minimal `SubTaskA -> SubTaskB -> END` to keep subgraph traversal cheap.

## Behavior injection

Every task body shares a common preamble (`svc.step`):

1. Increment the per-task atomic visit counter.
2. Parse `superflowapi.SuperflowState` out of flow state.
3. Look up `state.Behaviors[<TaskName>]`, a `TaskBehavior` struct, and apply any of:
   - `SleepMs` — `flow.Sleep(d)`
   - `ErrorStatus` — `return errors.New(..., status)` (drives 409, 429, 5xx paths)
   - `Interrupt` — `flow.Interrupt(payload)` then return nil
   - `Goto` — `flow.Goto(target)` then continue/return per the other knobs
   - `Retry` — `flow.Retry(math.MaxInt32, 0, 0, 0)` and return nil

The behaviors map keys are the PascalCase task names (`TaskA`, `SubTaskB`, `ErrorHandler`, ...) so a test
declares behavior per node without coordinating with the graph topology.

## What is NOT covered here

- **404 ack-timeout.** Needs many flows hitting an inactive subscription in parallel to observe drop
  semantics; `verify/ackdroppedflow` keeps that role. Behavior injection via task state can't simulate a
  task that *never runs*.
- **Foreman scheduling fixtures** (priority, fairness, sharding-distribution, saturated band, max
  concurrency, adaptive concurrency, soak, backpressure). Those depend on side-channel recording
  (`svc.Order()`, `svc.Bands()`, etc.) that is intrinsic to each fixture's purpose. The trivial graph in
  those fixtures is incidental to what they test; running them against superflow's shape would not add
  coverage.

## Why two SetFanIn nodes

The graph has two genuine fan-outs (the `forEach` from TaskB, and the conditional from TaskD), each of
which converges on a different node (TaskD and TaskE respectively). Both are marked. The validator counts
goto and onError transitions as non-fan-out, so `taskC -> errorHandler` and `taskE -> taskZ` do not need
their own fan-in markers.

## Counters

`svc.Visits("TaskA")` returns the atomic visit count for one task; `svc.AllVisits()` snapshots all of
them. `svc.ResetVisitCounters()` zeros them between subtests sharing one service instance. The counter
map is initialized in `OnStartup`, populated from a package-level `taskNames` list.
