**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

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
  └─ IdentityVerification (subgraph) ──→ ReviewCredit
                                            │
                                    ReviewCredit → Decision → END
                                       ↕ (goto)
                                  RequestMoreInfo
```

- `SubmitCreditApplication` unpacks the applicant into individual state fields
- `VerifyCredit` checks credit score ≥ 550. On error, routes to `HandleCreditError`
- `HandleCreditError` receives the `TracedError` via `onErr`, sets `creditVerified = false`
- `VerifyEmployment` runs once per employer via forEach; failures summed with `ReducerAdd`
- `IdentityVerification` is a subgraph (see below)
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

## Fault Injection

The `CreditApproval` workflow accepts a `faultInjection string` as a second input parameter (alongside the `Applicant`). This value is set directly in the workflow state and read by tasks that support special test behaviors. Tasks check `strings.Contains(faultInjection, "...")` to trigger faults:

- **`"Error"`** - `VerifyCredit` returns an error, triggering the error transition to `HandleCreditError`
- **`"Retry"`** - `VerifyCredit` retries up to 3 times using `flow.BackoffRetry(3, 0, 0, 0)`
- **`"Subgraph"`** - `VerifyCredit` dynamically invokes `IdentityVerification` as a child workflow via `flow.Subgraph()`
- **`"MissingSSN"`** - `VerifySSN` interrupts with `{"request": "ssn"}` to request the SSN from the caller
- **`"Delay"`** - `VerifyPhoneNumber` sleeps 1.5s to exceed the 1s time budget, causing a timeout failure
- **`"Sleep"`** - `ReviewCredit` sleeps 200ms before approving, testing the `flow.Sleep` control signal
- **`"BadGoto"`** - `ReviewCredit` calls `flow.Goto` with a non-existent target, causing the flow to fail

Multiple faults can be combined in a single string (e.g. `"MissingSSN,Delay"`) since each task uses `strings.Contains`.
