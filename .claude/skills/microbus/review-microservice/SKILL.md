---
name: review-microservice
user-invocable: true
description: Performs a thorough review of a single Microbus microservice. Checks for completeness, framework compliance, code quality, security, test coverage, documentation, API design, and data access performance. Produces a structured report with findings and recommendations.
---

**CRITICAL**: Scope this review strictly to the microservice directory you are asked to review. Do NOT explore or analyze other microservices.

**Naming convention used in this skill**: `service.go` refers collectively to all hand-written `.go` implementation files in the microservice directory (excluding `intermediate.go`, `mock.go`, and test files). `service_test.go` refers collectively to all `*_test.go` files. In most microservices these are single files, but they may be split across multiple files in rare cases. Step 1 identifies the actual files; subsequent steps use these shorthand names.

## Workflow

Copy this checklist and track your progress:

```
Microservice review:
- [ ] Step 1: Read the microservice
- [ ] Step 2: MARKER completeness
- [ ] Step 3: Framework compliance
- [ ] Step 4: Test quality
- [ ] Step 5: Manifest consistency
- [ ] Step 6: Code quality
- [ ] Step 7: Security
- [ ] Step 8: Documentation
- [ ] Step 9: API design
- [ ] Step 10: Performance and data access
- [ ] Step 11: Run vet and tests
- [ ] Step 12: Produce the report
```

#### Step 1: Read the Microservice

List the directory to identify all files. Read `manifest.yaml`, `intermediate.go`, `mock.go`, and all files in the `*api/` directory. Read all hand-written `.go` implementation files (`service.go` and any others) and all `*_test.go` files.

Build a feature inventory by collecting all `// MARKER: X` comments from `service.go`. This inventory is the ground truth for what features are implemented.

#### Step 2: MARKER Completeness

For each `// MARKER: X` in the feature inventory from Step 1, check the expected files based on feature type:

- **All features** — marker must appear in `intermediate.go` (subscription wiring)
- **Functions, webs, tasks, workflows** — marker must also appear in `mock.go` (mock implementation) and `service_test.go` (at least one test case)
- **Config `OnChanged` callbacks, inbound event sinks, outbound event triggers, tickers** — `mock.go` entry is not expected; `service_test.go` coverage is desirable but optional

Determine the feature type from context: `OnChangedXxx` functions are config callbacks; functions with no return value on a ticker interval are tickers; functions with a `*workflow.Flow` parameter are tasks or workflow graph builders; outbound events are identified by calls to `NewMulticastTrigger`.

Report any missing marker where it is expected.

Also check the reverse: are there MARKERs in `intermediate.go` or `mock.go` that have no corresponding handler in the feature inventory from Step 1? Flag these as stale wiring.

#### Step 3: Framework Compliance

Grep the hand-written `.go` files (same set as Step 1) for violations of Microbus framework conventions:

- **`time.Now()`** — must be `svc.Now(ctx)` in handlers and methods that receive a context
- **`http.Client`, `http.Get`, `http.Post`, `http.DefaultClient`** — outbound HTTP must go through the HTTP egress proxy (`httpegressapi`)
- **Bare goroutines** — `go func` or `go ` followed by a function call must use `svc.Go(ctx, func)` instead
- **`fmt.Errorf`** — must use `errors.New` instead
- **`return err`** without `errors.Trace(err)` — all non-nil error returns must be traced (allow exceptions for `nil` checks and `errors.Trace` already applied in the same line)
- **`fmt.Print`, `log.Print`, `println`** — structured logging must use `svc.LogDebug/Info/Warn/Error`
- **`context.Background()` or `context.TODO()`** — handlers must propagate the received context, not create a new root context
- **`errors.Newc`, `errors.Newf`, `errors.Newcf`** — deprecated constructors; use `errors.New` instead
- **`httpx.PathValues`** — deprecated; use `r.PathValue("name")` instead
- **Resource lifecycle** — resources opened or subscribed in `OnStartup` (connections, file handles, hooks) should have a corresponding release or unsubscribe in `OnShutdown`. Flag asymmetric lifecycle management.

#### Step 4: Test Quality

Review all `*_test.go` files in the microservice directory (commonly just `service_test.go`, but tests may be split across multiple files):

- Does each integration test function (`TestXxx_FeatureName`) have at least one `assert` call? Flag empty test bodies or tests that call the endpoint but make no assertions.
- Does each integration test initialize a proper `application.New()` + `RunInTest(t)` harness?
- Are downstream dependencies mocked? Flag tests that would fail if a real downstream service is unavailable.
- Does the mock test (`TestXxx_Mock`) cover all features with at least a smoke-test case?
- Are top-level test functions run with `t.Parallel()`? A missing `t.Parallel()` is acceptable if the same line is preceded by a comment of the form `// No parallel: {reason}` explaining why.
- **Mock handler quality** — do mock handler functions in `MockXxx` calls exercise the feature in a meaningful way, or do they return zero values unconditionally? Flag mocks that always return the same constant regardless of inputs, as these may mask bugs in the caller.
- **HINT scaffold comments** — `/* HINT: ... */` or `// HINT: ...` blocks in test files are intentional placeholders for future work. Do not flag their presence.

#### Step 5: Manifest Consistency

Compare the manifest against the feature inventory from Step 1:

- Are all features implemented in `service.go` (via MARKERs) also declared in `manifest.yaml`?
- Are all features declared in `manifest.yaml` implemented in `service.go`?
- Are all `description` fields non-empty and meaningful (not placeholder text)?
- Does the `downstream` section match the actual `*api` imports used in the code?
- Is `db: SQL` set if the code imports `database/sql` or `github.com/microbus-io/sequel`?
- Is `cloud` set if the code makes outbound HTTP requests via the egress proxy?
- **Config defaults in range** — for each config with both a `default` and a `validation` range, verify the default falls within the declared range.
- **Dead metrics** — for each metric declared in the `metrics` section, check that `svc.IncrementXxx` or `svc.RecordXxx` is called somewhere in the hand-written code. Flag metrics that are declared but never recorded.

#### Step 6: Code Quality

- **Route conventions** — are routes in kebab-case? Routes typically follow the handler name (e.g. `MyHandler` → `/my-handler`) but deviations are acceptable when intentional (e.g. REST-style paths, versioned routes). Flag routes that appear inconsistent without obvious reason.
- **Port conventions** — conventional port assignments are `:443` for standard endpoints, `:444` for internal-only, `:888` for management, `:417` for events, `:428` for tasks, but these are defaults not enforced rules. Flag deviations only when they appear unintentional or inconsistent with the endpoint's stated purpose.
- **Error strings** — do `errors.New` strings start with a lowercase letter and not end with punctuation?
- **HTTP status code on errors** — `errors.New` and `errors.Trace` accept an unpaired `int` argument as the HTTP status code. Errors caused by user input (validation failures, missing fields, unauthorized state) should attach a 4xx code (e.g. `http.StatusBadRequest`); errors caused by internal failures default to 500. Flag user-caused errors that return a generic 500.
- **Godoc** — do all exported functions and methods in `service.go` and `*api/` have godoc comments?
- **Description consistency** — for each feature, the `sub.Description(...)` string in `intermediate.go`, the `description:` field in `manifest.yaml`, the godoc on the handler in `service.go`, and the godoc on the corresponding client stub methods in `*api/client.go` should all describe the same behavior. Flag any that are meaningfully inconsistent (not just minor wording differences).
- **Magic HTTP arguments** — are `httpRequestBody`, `httpResponseBody`, and `httpStatusCode` used correctly in function signatures?
- **Struct tags** — do JSON struct tags in `*api/endpoints.go` use `omitzero` (not `omitempty`)?
- **`jsonschema` tags** — do structs used in functional endpoint signatures have `jsonschema:"description=..."` tags on their fields?
- **Concurrent map access** — are maps that are read or written from multiple goroutines (e.g. from ticker handlers, `svc.Go` calls, or `OnStartup` vs. request handlers) protected by a mutex or replaced with `sync.Map`? Flag unprotected shared maps.
- **Ticker concurrency** — if a ticker handler performs slow operations (DB queries, downstream calls), could overlapping ticker fires cause contention or double-processing? Flag tickers without a guard (mutex, flag, or atomic) when the body's duration might exceed the interval.
- **Actor context in tickers** — tickers run on a service-derived context with no inherited request frame or actor. If a ticker calls a downstream endpoint that requires `requiredClaims` or relies on `act.Of(ctx)` for tenant scoping, it must establish its actor context explicitly. Flag tickers that call authorized endpoints without setting up an actor.
- **Goroutine leaks in `svc.Go`** — callbacks passed to `svc.Go` that contain `for { ... }` loops or unbuffered channel reads must include a `case <-ctx.Done():` arm or another ctx check, otherwise they leak across service shutdown. Flag long-lived `svc.Go` callbacks with no ctx-cancellation path.
- **`svc.Parallel` opportunities** — look for sequential blocks of two or more independent downstream calls (each result unused by the next call before all complete). Flag these as candidates for `svc.Parallel` to reduce latency.
- **Context cancellation in long loops** — flag loops that iterate over large or unbounded collections without checking `ctx.Err()`. A cancelled request should not continue doing expensive work.
- **Distributed cache initialization** — [Info] if `svc.DistribCache` is used, is it initialized in `OnStartup` with `SetMaxAge` and/or `SetMaxMemory`? Reasonable defaults exist, but explicit initialization avoids surprises under load.
- **Cache invalidation** — when handlers mutate data that is also cached in `svc.DistribCache`, the corresponding cache entry must be invalidated (`Delete`) or overwritten (`Set`). Flag mutating handlers that change the canonical store without updating the cache, since subsequent reads will return stale data until the entry expires.
- **Cache stampede protection** — for `svc.DistribCache` miss paths that perform expensive work (DB query, downstream call, heavy computation), prefer `GetOrCompute`/`LoadOrCompute` over the `Get`+miss+compute+`Set` pattern. The compute methods deduplicate concurrent callers in the same process via singleflight, bounding backend load to one call per key per process instead of one per request. Flag heavy miss paths that use the read-then-write pattern.

#### Step 7: Security

- **Missing `requiredClaims`** — do endpoints on `:443` or `:444` that handle sensitive operations (mutations, private data, admin actions) specify `requiredClaims`? Flag endpoints that appear sensitive but lack authorization.
- **Issuer verification** — do `requiredClaims` expressions that check roles or identity also include `iss=~"access.token.core"` to ensure a properly exchanged token?
- **Unmarked secrets** — do config properties whose names suggest a secret value (e.g. `Key`, `Password`, `Token`, `Secret`, `APIKey`) have `secret: true` in the manifest?
- **Input handling** — are user-supplied path/query/body values validated before use? Flag direct use of user input in file paths, SQL strings, or HTML output without sanitization.
- **Multi-tenant isolation** — for SQL services that scope data by tenant, every `WHERE` and `JOIN ON` clause on tenant-scoped tables must include the tenant predicate. Bare queries without a tenant filter are cross-tenant data leaks. `svc.DistribCache` keys for tenant-scoped data must include the tenant ID in the key, otherwise cache hits cross tenants.
- **Sensitive data in logs** — grep `LogInfo`/`LogWarn`/`LogError`/`LogDebug` calls for arguments named or containing `password`, `token`, `secret`, `apiKey`, `authorization`, `email`, `ssn`. Flag direct logging of values from config properties marked `secret: true` in the manifest.

#### Step 8: Documentation

- **`CLAUDE.md`** — does the file contain meaningful content beyond the H1 hostname heading? Is the design rationale captured (the *why*, not just the *what*)?
- **`PROMPTS.md`** — if present, does it accurately describe how to reproduce the microservice in its current form?
- **Complex logic** — are non-obvious algorithms, business rules, or workarounds explained with inline comments? Flag complex blocks that are hard to follow without explanation.
- **TODO/FIXME** — grep for `TODO`, `FIXME`, `HACK`, `XXX` comments. Flag any that indicate unfinished work.

#### Step 9: API Design

Review the public interface of the microservice — its functions, web handlers, events, and configs — for design quality:

- **Cohesion** — does each endpoint do exactly one clearly defined thing? Flag endpoints that combine unrelated concerns or whose names suggest they do two things (e.g. `GetAndValidate`, `CreateOrUpdate` without a clear reason).
- **Function signatures** — are signatures clean? Flag functions with more than 4–5 parameters that should use an input struct, or functions that return an unusually large number of values.
- **Separation of concerns** — does this microservice mix UI rendering (HTML templates, static assets, browser-facing pages) with data or business logic endpoints? UI concerns belong in a dedicated microservice; data and business logic belong in another. Flag services that do both.
- **Event vs direct call** — are events used for loose coupling where appropriate (notifications, cross-domain reactions, cache invalidation) and direct calls used where a synchronous response is needed? Flag cases where a direct call is used where an event would decouple better, or where an event is used where the caller clearly needs a response.
- **Missing operations** — given the data model and described purpose, are there obviously missing endpoints (e.g. a service that creates but never lists or deletes, without a clear reason)?
- **Output size** — do list-style endpoints or response structs return unbounded or excessively large payloads? Flag endpoints that could return large result sets without pagination.
- **Naming consistency** — are endpoint, event, and config names consistent in style and vocabulary across the microservice (e.g. not mixing `Get`/`Fetch`/`Load` for the same semantic)?
- **Idempotency** — are mutating endpoints naturally idempotent or do they use optimistic concurrency (e.g. revision checks)? Clients may choose to retry on timeout; endpoints that produce irreversible side effects on every call (e.g. sending emails, charging a payment method, firing a webhook) should note this in their godoc so callers know not to retry blindly.
- **Task idempotency under retry** — Foreman may retry failed tasks, so each task invocation must be safe to repeat. Flag tasks that perform irreversible side effects (sending emails, charging payments, firing webhooks) without an idempotency guard, or tasks that increment counters or append to lists in a way that double-counts on retry.
- **Backward compatibility** — if the project is in a git repository, compare `*api/endpoints.go` and `*api/client.go` against the `main` branch. Flag removed or renamed exported types, fields, JSON tag names, endpoint definitions, and method signatures — all of these break downstream services that import the package. Skip this check if the project is not source-controlled or if the microservice was just created on this branch.

#### Step 10: Performance and Data Access

This step applies primarily to SQL CRUD microservices. Skip checks that are not relevant if the service does not use a database.

- **N+1 queries** — look for loops that iterate over a result set and perform a database query or call another microservice per row. This is a common and expensive pattern. Flag any loop where the body contains a SQL query, a call to `svc.*` client, or a call to a downstream `*api` client.
- **Expensive operations inside DB loops** — flag any loop over database rows where the body performs HTTP requests, sleeps, or other slow operations. Database connections should be released quickly; long-held result sets block connection pool slots.
- **Unindexed query conditions** — read the database schema from migration files (typically in `resources/` or a `migrations/` subdirectory). Extract all indexed columns (from `PRIMARY KEY`, `UNIQUE`, `CREATE INDEX`, and `INDEX` declarations). Then examine hardcoded SQL `WHERE`, `JOIN ON`, and `ORDER BY` clauses in the code and flag conditions on columns that have no index and are likely to cause full table scans.
- **Unbounded queries** — flag SQL `SELECT` statements without a `LIMIT` clause that could return arbitrarily large result sets.
- **Transaction scope** — flag database transactions that span expensive operations such as downstream service calls or HTTP requests. Transactions should be kept as short as possible.
- **Migration safety** — SQL migration files should be append-only. Flag edits to existing migration files. Destructive operations (`DROP TABLE`, `DROP COLUMN`, type-narrowing `ALTER COLUMN`) should be flagged for review even if intentional. New `NOT NULL` columns without a default break in-flight rollouts where old replicas still write rows that omit the column.

#### Step 11: Run Vet and Tests

Run the following and report any failures:

```shell
go vet ./main/...
go test -coverprofile=/tmp/coverage.out ./path/to/microservice/...
go tool cover -func=/tmp/coverage.out
```

Flag compilation errors, vet warnings, and test failures with their full output.

From the coverage output, read the coverage percentage reported for the main microservice package (e.g. `github.com/.../myservice`). Ignore the `*api/` subpackage (generated client code) and the `resources/` subpackage (embed-only). Flag as a Warning if the main package coverage is below 70%.

#### Step 12: Produce the Report

Compile findings into a structured report. For each category (Steps 2–11), list specific findings with file paths and line numbers where applicable, followed by actionable recommendations.

Use severity levels:
- **Critical** — missing tests, security gaps, or framework violations that could cause data loss, security vulnerabilities, or silent misbehavior
- **Warning** — code quality or consistency issues that may cause problems during evolution
- **Info** — minor style or documentation gaps

Omit categories with no findings but mention them as passing in the summary.

```md
# Microservice Review: {hostname}

## Summary
Brief description of the microservice, what was reviewed, and overall assessment.
Note categories that passed without issues.

## MARKER Completeness
### Findings
- ...
### Recommendations
- [severity] ...

(repeat for each category that has findings)

## Conclusion
Prioritized action items.
```
