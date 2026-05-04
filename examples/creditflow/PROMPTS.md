## Create CreditFlow Microservice

Create an example microservice at hostname `creditflow.example` that demonstrates advanced agentic workflow features of the Microbus framework: fan-out/fan-in, subgraphs, forEach, goto loops, error transitions, interrupts, retries, time budgets, and sleep signals.

## Applicant Type

Define an `Applicant` struct in the `creditflowapi` package with fields `ApplicantName string`, `SSN string`, `Address string`, `Phone string`, `Employers []string`, and `CreditScore int` (all with camelCase JSON tags, `omitzero`).

## CreditApproval Workflow Graph

The main workflow accepts inputs `applicant Applicant` and `faultInjection string`, and produces outputs `approved bool`, `creditVerified bool`, `employmentFailures int`, `identityVerified bool`.

Graph structure:

```
SubmitCreditApplication → (fan-out)
  ├─ VerifyCredit ──────────────────────────→ ReviewCredit
  │    └─ (onErr) HandleCreditError ────────→ ReviewCredit
  ├─ VerifyEmployment (forEach employers) ──→ ReviewCredit
  └─ IdentityVerification (subgraph) ───────→ ReviewCredit
                                                    │
                                             ReviewCredit → Decision → END
                                                  ↕ (goto)
                                             RequestMoreInfo
```

Construction details:
- `graph.AddSubgraph(identityVerification)` — marks IdentityVerification as a subgraph node
- `graph.AddErrorTransition(verifyCredit, handleCreditError)` — routes errors from VerifyCredit to HandleCreditError
- `graph.AddTransitionForEach(submitCreditApplication, verifyEmployment, "employers", "employerName")` — creates one VerifyEmployment invocation per element of the `employers` array, binding each element to `employerName`
- `graph.SetReducer("employmentFailures", workflow.ReducerAdd)` — sums `employmentFailures` across all forEach instances
- `graph.AddTransitionGoto(reviewCredit, requestMoreInfo)` — declares that ReviewCredit may dynamically goto RequestMoreInfo

## IdentityVerification Workflow Graph (Subgraph)

Accepts inputs `applicantName`, `ssn`, `address`, `phone`, `faultInjection`; produces output `identityVerified bool`.

```
InitIdentityVerification → (fan-out)
  ├─ VerifySSN
  ├─ VerifyAddress
  └─ VerifyPhoneNumber  [time budget: 1s]
       ↓ (fan-in)
  IdentityDecision → END
```

Set `graph.SetTimeBudget(verifyPhoneNumber, 1*time.Second)` on VerifyPhoneNumber to enforce the 1-second deadline.

## Task Endpoints

All task endpoints are on port `:428`. Each receives and returns typed state fields via function arguments and named return values.

- **SubmitCreditApplication** `:428/submit-credit-application` — unpacks `applicant Applicant` into individual state fields: returns `applicantName`, `ssn`, `address`, `phone`, `employers []string`, `creditScore int`.

- **VerifyCredit** `:428/verify-credit` — inputs `creditScore int`, `faultInjection string`; returns `creditVerified bool`. Normal logic: `creditScore >= 550`. Also handles fault injection (see below).

- **HandleCreditError** `:428/handle-credit-error` — inputs `onErr *errors.TracedError`; returns `creditVerified bool`. Logs the error as a warning and returns `false`.

- **VerifyEmployment** `:428/verify-employment` — inputs `applicantName string`, `employerName string`; returns `employmentFailures int`. Returns `1` if either argument is empty, otherwise `0`.

- **InitIdentityVerification** `:428/init-identity-verification` — inputs `applicantName`, `ssn`, `address`, `phone`; no outputs. Pass-through; inputs are already set in workflow state.

- **VerifySSN** `:428/verify-ssn` — inputs `ssn string`, `faultInjection string`; returns `ssnVerified bool`. Validates pattern `^\d{3}-\d{2}-\d{4}$` and rejects if last 4 digits are `0000`. Also handles `MissingSSN` fault.

- **VerifyAddress** `:428/verify-address` — inputs `address string`; returns `addressVerified bool`. Passes if address is non-empty and does not contain `"Nowhere"`.

- **VerifyPhoneNumber** `:428/verify-phone-number` — inputs `phone string`, `faultInjection string`; returns `phoneVerified bool`. Validates pattern `^\d{3}-\d{3}-\d{4}$` or `^\(\d{3}\) \d{3}-\d{4}$`. Also handles `Delay` fault.

- **IdentityDecision** `:428/identity-decision` — inputs `ssnVerified`, `addressVerified`, `phoneVerified`; returns `identityVerified bool`. Returns `true` only if all three are true.

- **RequestMoreInfo** `:428/request-more-info` — inputs `reviewAttempts int`; returns `reviewAttemptsOut int` (note: output argument name has `Out` suffix). Returns `reviewAttempts + 1`.

- **ReviewCredit** `:428/review-credit` — inputs `creditScore int`, `creditVerified bool`, `reviewAttempts int`, `faultInjection string`; returns `creditVerifiedOut bool`. Logic:
  - Score >= 650: pass through unchanged
  - Score 580-649: approve (`return true`)
  - Score 550-579 and `reviewAttempts < 2`: call `flow.Goto(requestMoreInfo.URL())`, return current value
  - Score 550-579 and `reviewAttempts >= 2`: approve
  - Score < 550: return unchanged (reject)
  Also handles `BadGoto` and `Sleep` faults.

- **Decision** `:428/decision` — inputs `creditVerified bool`, `employmentFailures int`, `identityVerified bool`; returns `approved bool`. Returns `creditVerified && employmentFailures == 0 && identityVerified`.

## Fault Injection

The `faultInjection string` field is read by tasks using `strings.Contains`. Multiple faults can be combined in one string. Supported faults:

- **`"Error"`** — `VerifyCredit` returns `errors.New("credit bureau unavailable", http.StatusServiceUnavailable)`, triggering the error transition to `HandleCreditError`.
- **`"Retry"`** — `VerifyCredit` calls `flow.Retry(3, 0, 0, 0)`. On the first three invocations this returns `true` (retry); on the fourth it returns `false` (proceed), and the task returns `true`.
- **`"Subgraph"`** — `VerifyCredit` dynamically invokes `IdentityVerification` as a child workflow via `flow.Subgraph(...)`. On first run, sets `subgraphPending = true` in state and calls `flow.Subgraph(identityVerification.URL(), map[string]any{...})`, then returns `false`. On re-entry (subgraph completed), reads `identityVerified` from state and computes `creditScore >= 550 && identityVerified`.
- **`"MissingSSN"`** — `VerifySSN` calls `flow.Interrupt(map[string]any{"request": "ssn"})` and returns `false`.
- **`"Delay"`** — `VerifyPhoneNumber` sleeps `1500ms`, exceeding the 1s time budget and causing a timeout.
- **`"Sleep"`** — `ReviewCredit` calls `flow.Sleep(200 * time.Millisecond)` before returning `true`.
- **`"BadGoto"`** — `ReviewCredit` calls `flow.Goto("https://credit.flow.example:428/non-existent-task")`, causing the flow to fail.

## Demo Web Endpoint

- `Demo` on `ANY /demo` — an interactive HTML page backed by `resources/demo.html`. On GET with no params, pre-fills defaults (name "Alice", SSN "123-45-6789", address "123 Main St", phone "555-123-4567", employer "Acme Corp", score 750). On POST, builds a `CreditApprovalIn` from form fields, then calls a `runWorkflow` helper that:
  1. `foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), initialState)` to create the flow
  2. `foremanClient.Start(ctx, flowKey)` to start it
  3. `foremanClient.AwaitAndParse(ctx, flowKey, &out)` to wait and parse results
  4. Fetches history (`foremanClient.History`) and Mermaid diagram (`foremanClient.HistoryMermaid`) in parallel using `svc.Parallel`
  Renders the template with status, approval result, step history, and Mermaid diagram.

A `flattenSteps` helper recursively flattens `[]foremanapi.FlowStep` into `[]demoStep` structs (with `TaskName`, `Status`, `Changes` as JSON string, `Indent bool`), indenting subgraph steps.

## Downstream Dependency

Imports `foremanapi` for workflow lifecycle management (`Create`, `Start`, `AwaitAndParse`, `History`, `HistoryMermaid`).
