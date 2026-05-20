# cancelbackpressureflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the status guard in the foreman's 429 bounce path. `handleBackpressure`
writes the offending step back to `pending` with an UPDATE gated by `WHERE step_id=? AND
status='running'`. If a `Cancel` commits between the dispatch starting and the 429 returning, the
step is already `cancelled` by the time the bounce UPDATE runs, the predicate must match zero
rows, and the step must stay `cancelled` rather than getting resurrected into `pending`.

## How the race is forced

The task exposes a `Ready()` channel that closes when it enters its parked wait, and a `Release()`
method that wakes it. The test:

1. Creates and starts a flow. The foreman dispatches the single task; the task parks.
2. Waits on `Ready()` so the step is known to be in `running` status under a lease.
3. Calls `Cancel`. Its transaction commits, moving the step (and flow) to `cancelled`.
4. Calls `Release`. The task wakes and returns 429.
5. The foreman observes the 429 and runs `handleBackpressure`. Its UPDATE finds zero rows because
   the status guard matches only `running`. No resurrection.
6. `Await` returns `cancelled`. `History` shows the step is still `cancelled`.

This is the deterministic version of the race; without the channel coordination the timing window
between `Cancel` commit and the task's 429 return is microseconds and the test would be flaky.

## Patterns exercised

- Cancel's transactional flip from `running` to `cancelled` racing the dispatch error path.
- `handleBackpressure`'s `WHERE status='running'` guard: the only protection against a 429 bounce
  reviving a cancelled step.
- The regulator's cut + gossip running even when the bounce UPDATE matches zero rows. That side
  effect is fine - a cancelled step still legitimately observed a 429, so the per-task valve
  should still tighten.

## Determinism caveat

The Cancel-then-release sequence is fully serialized in the test, so the race outcome is
deterministic. The only non-determinism is in how quickly the bounce UPDATE runs after `Release`;
the test sleeps 200ms before inspecting `History` to give the bounce path time to attempt the
write (and fail the guard).
