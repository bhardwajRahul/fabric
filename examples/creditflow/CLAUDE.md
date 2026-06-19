# creditflow.example

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Overview

The creditflow example implements a multi-step credit approval workflow that exercises most workflow features: fan-out/fan-in, subgraphs, forEach, goto loops, error transitions, interrupts, retries, sleep, and time budgets. It serves as both an example and the foreman's primary integration test fixture.

## Workflows

### CreditApproval

The main workflow. Accepts an `Applicant` and produces an `approved` boolean.

```
SubmitCreditApplication → (fan-out)
  ├─ VerifyCredit ─────────→ ReviewCredit
  │    └─ (on err) HandleCreditError → ReviewCredit
  ├─ VerifyEmployment (forEach employer) → ReviewCredit
  └─ RunIdentityVerification (flow.Subgraph) → ReviewCredit
                                            │
                                    ReviewCredit → Decision → END
                                       ↕ (goto)
                                  RequestMoreInfo
```

- `SubmitCreditApplication` unpacks the applicant into individual state fields
- `VerifyCredit` checks credit score ≥ 550. On error, routes to `HandleCreditError`
- `HandleCreditError` receives the `TracedError` via `onErr`, sets `creditVerified = false`
- `VerifyEmployment` runs once per employer via forEach; failures summed with `ReducerAdd`
- `RunIdentityVerification` calls `flow.Subgraph` to run the `IdentityVerification` child workflow (see below) and adopts its `identityVerified` output
- `ReviewCredit` is the fan-in point. Passes through for good scores (650+), approves borderline (580-649), uses goto to `RequestMoreInfo` for very borderline (550-579), rejects below 550
- `Decision` ANDs all verification results

### IdentityVerification (subgraph)

```
InitIdentityVerification → (fan-out)
  ├─ VerifySSN
  ├─ VerifyAddress
  └─ VerifyPhoneNumber (1s time budget)
       ↓ (fan-in)
  IdentityDecision → END
```

## Why no fault injection

creditflow deliberately has no fault-injection hooks (no `faultInjection`-style field that tasks inspect to force error/retry/sleep/interrupt/timeout/goto behaviors). Those workflow mechanisms are stress-tested in isolation by the dwarf workflow engine's own test suite, so this example stays a clean illustration of a *real* workflow shape, without test-only branches.
