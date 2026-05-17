# fairnessflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the foreman's two-level weighted fairness. The graph is the single task
`tally -> END`. The test pins the foreman to one worker, creates many flows across two fairness keys
with different `FairnessWeight` values (all at the same priority) while a long-running holder flow
occupies the lone worker, then drains them and inspects the dispatch order.

## Patterns exercised

- `*workflow.FlowOptions{FairnessKey, FairnessWeight}` supplied at `Create`
- Two-level selection: pick a fairness key (weighted-random, Efraimidis-Spirakis), then the key's oldest
  step. Re-picked per dispatch so share is proportional to weight and independent of backlog depth
- A heavy key does not stop a light key from making progress

## Determinism caveat

By construction, the only probabilistic step of the foreman's selection is the weighted-random pick
among candidate keys, which fires only when more than one key is in the candidate set. So this fixture's cross-key dispatch *share* is asserted statistically: counts within a
generous tolerance band around the weight ratio, over enough flows that the law of large numbers holds.
The exactly-assertable properties are kept exact and must not flake: intra-key order is FIFO by
`step_id` (creation order), every flow eventually completes (liveness), and the light key makes progress
well before the heavy key is exhausted. Only the heavy:light ratio is a tolerance assertion.
