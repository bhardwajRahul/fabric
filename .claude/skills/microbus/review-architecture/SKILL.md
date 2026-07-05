---
name: review-architecture
description: Performs an architectural review of a microservice-based system built on the Microbus framework. Examines service boundaries, dependencies, coupling, API design, resilience, data ownership, agentic workflow composition, observability, security, and operational concerns. Produces a structured report with findings and recommendations.
---

## Workflow

Copy this checklist and track your progress:

```
Architectural review:
- [ ] Step 1: Build the system map
- [ ] Step 2: Service boundaries and design
- [ ] Step 3: Dependencies and coupling
- [ ] Step 4: API design
- [ ] Step 5: Resilience and reliability
- [ ] Step 6: Data and consistency
- [ ] Step 7: Agentic workflows
- [ ] Step 8: Observability
- [ ] Step 9: Security
- [ ] Step 10: Operational concerns
- [ ] Step 11: Documentation
- [ ] Step 12: Produce the report
```

#### Step 1: Build the System Map

Read `main/main.go` to identify all microservices included in the application and their startup group ordering. Read the `manifest.yaml` and `CLAUDE.md` of each microservice to understand its features, downstream dependencies, design rationale, and general properties. Read `config.yaml` for runtime configuration context. If `main/topology.mmd` exists, read it to visualize the dependency graph.

Produce a written inventory: for each microservice, note its hostname, purpose (from `description`), downstream dependencies, outbound events, inbound event sinks, configs, `db` and `cloud` properties, and startup group. This inventory is the foundation for all subsequent steps.

#### Step 2: Service Boundaries and Design

Evaluate each microservice for:

- **Separation of concerns** - does each service own a single, well-defined domain? Flag services whose description suggests multiple unrelated responsibilities.
- **Right-sized services** - are services too granular (nano-services adding unnecessary network hops) or too broad (distributed monolith with many unrelated endpoints)?
- **Encapsulated persistence** - does any service access another service's database, share tables via foreign keys or JOINs, or make assumptions about another service's schema? Grep for SQL statements and check import paths to verify.
- **Cohesion** - is related functionality grouped together, or scattered across services? Are there endpoints that would be better placed in a different microservice?
- **Hostname conventions** - do hostnames follow the framework's dotted lowercase convention? Are core services on `.core` and application services on an appropriate domain?
- **Encapsulated external APIs** - is each external API (third-party web service, cloud provider, etc.) wrapped in its own dedicated microservice, as indicated by the `cloud` property in the manifest? Flag upstream services that call external APIs directly (grep for `http.NewRequest` or `http.Get` outside of a wrapping microservice) rather than going through an encapsulating microservice and the HTTP egress proxy.
- **Externalized strings** - are user-facing strings (error messages shown to end users, UI labels, etc.) externalized to `resources/text.yaml` and loaded via `svc.LoadResString`, rather than hardcoded in the source? Internal error messages (for logging or developer consumption) need not be externalized.

#### Step 3: Dependencies and Coupling

Analyze the dependency graph from the `downstream` sections of each `manifest.yaml`, and verify against actual code:

- **No circular dependencies** - is the dependency graph a DAG? Trace the graph from the inventory built in Step 1. Flag any cycles, including indirect cycles (A depends on B depends on C depends on A).
- **Loose coupling** - do services interact only through their `*api` client stubs? Grep import statements in each microservice for imports of other microservices' packages. Flag any import of a sibling service's internal package (anything other than its `*api` package).
- **Fan-out** - does any single request cascade through many services? Trace call chains from ingress-facing endpoints. The framework enforces a max call depth of 64, but even chains deeper than 4-5 levels warrant scrutiny for latency and fragility.
- **Dependency direction** - do higher-level, domain-specific services depend on lower-level, generic services - not the reverse? Flag infrastructure or utility services that depend on domain services.
- **Event pairing** - does every inbound event sink reference a valid outbound event from the source service? Check that the `source` in each `inboundEvents` entry matches an actual `outboundEvents` entry in the source service's manifest.
- **Appropriate use of events** - are events used for loose coupling where appropriate (e.g., notifications, cache invalidation, cross-domain reactions), rather than forcing direct downstream dependencies? Conversely, are events misused where a direct call would be simpler and more reliable (e.g., request/response flows shoehorned into events)?
- **Manifest accuracy** - does the `downstream` section of each manifest match the actual `*api` imports in the code? Flag missing or stale entries in either direction.

#### Step 4: API Design

Examine the `functions`, `webs`, and `outboundEvents` sections of each `manifest.yaml`, and review the corresponding handler signatures in `service.go`:

- **Consistent contracts** - are naming, HTTP methods, routes, and error handling patterns uniform across services? Flag inconsistencies in conventions (e.g., one service uses `Load` while another uses `Get` for the same semantic operation).
- **Appropriate endpoint type** - are functional endpoints used for typed request/response RPCs and web handlers used for HTML/file/streaming responses? Flag functional endpoints that should be web handlers (e.g., file downloads) or vice versa (e.g., web handlers that just marshal JSON).
- **Chatty APIs** - are there 1+N call patterns where a caller loops over singular operations instead of using a bulk endpoint? Read `service.go` of upstream services to check how they call downstream clients. Flag opportunities for bulk operations.
- **Route conventions** - do routes follow kebab-case? Do routes match their handler names (e.g., `MyHandler` maps to `/my-handler`)? Are port numbers used appropriately (`:443` default for HTTPS, `:888` for control endpoints, `:417` for events, `:0` for any-port)?
- **Method usage** - are HTTP methods used correctly (GET for reads, POST for creates/actions, PUT for full updates, DELETE for deletes)? Is `ANY` overused where a specific method would be more precise?
- **Load balancing** - is the `loadBalancing` setting appropriate for each endpoint? Should any unicast endpoint be multicast (`none`) or vice versa? Are there endpoints that should use a named queue for sticky routing?
- **Clean URLs** - are `//` route overrides used appropriately for user-facing pages, and not overused for internal APIs where the hostname prefix provides useful namespacing?
- **Function signatures** - are function signatures clean? Flag functions with excessive parameters that should use a struct, or functions that return too many values.
- **Error semantics** - are HTTP status codes used consistently and correctly across services? Is 404 always "not found"? Are 4xx (client errors) vs 5xx (server errors) distinguished properly? In Microbus, status codes are attached to errors via `errors.New` or `errors.Trace` - flag handlers that return generic 500s for what should be 400-level errors.
- **Pagination** - do `List`-style endpoints support pagination (limit/offset or cursor-based)? Flag endpoints that could return unbounded result sets without any paging mechanism.
- **Backward compatibility** - if a function signature has recently changed, will existing callers break? Flag signature changes that remove or reorder parameters without a migration path for consumers.

#### Step 5: Resilience and Reliability

Review `service.go` of each microservice:

- **Time budget awareness** - do long-running operations check the context for cancellation? Are there operations that could exceed the default 20-second time budget without using `pub.Timeout`? Flag any downstream calls in a loop without checking `ctx.Err()`.
- **Failure isolation** - when a downstream call fails, is the error handled gracefully or does it propagate unchecked and take down the caller's operation? Are there fallback paths for non-critical downstream failures?
- **Safe concurrency** - grep for bare `go ` and `go func` statements in `service.go` files. All goroutines must be launched via `svc.Go` (which handles panic recovery and lifecycle) rather than bare `go` statements. Is `svc.Parallel` used where multiple independent operations could run concurrently?
- **Idempotency** - are mutating operations safe to retry? This is especially important in Microbus because the framework may retry requests that fail due to ack timeout, meaning a handler could be invoked more than once for the same logical request.
- **Graceful degradation** - do services handle partial failures from downstream dependencies? For multicast calls (iterating over responses from `NewMulticastClient`), is the zero-response case handled appropriately?
- **Blocking operations** - are there synchronous operations in handlers that could block for extended periods without respecting context cancellation (e.g., file I/O, network calls without timeout, channel operations without select)?
- **Resource exhaustion** - are there maps, slices, or channels in the `Service` struct that grow without bound? Flag handlers that accumulate state without eviction or size limits. `DistribCache` and `lru.Cache` have built-in limits, but hand-rolled maps and slices do not.

#### Step 6: Data and Consistency

Evaluate data ownership and consistency strategies:

- **Ownership clarity** - does each piece of data have exactly one authoritative source (one microservice that owns the writes)?
- **Consistency strategy** - is there an explicit choice between strong and eventual consistency for each cross-service data flow? Flag cases where the consistency model is ambiguous or implicit.
- **Cross-service data** - when services need data owned by others, is the strategy appropriate? The three strategies are: on-demand querying via client stubs (strong consistency, higher latency), caching via `DistribCache` with event-driven invalidation (eventual consistency, volatile storage), or denormalization into local tables (eventual consistency, durable storage). Flag mismatches between the chosen strategy and the consistency requirements.
- **Cache discipline** - if `DistribCache` is used, are `MaxAge` and `MaxMemory` configured in `OnStartup`? Is the cache used only for data that can tolerate loss and staleness? The framework explicitly warns against using `DistribCache` to share state between peers - flag any such misuse.
- **No distributed transactions** - are there implicit distributed transactions where multiple services are mutated in sequence without compensation or saga logic? Flag operations that partially succeed across services with no rollback path.
- **Event ordering** - do event sinks assume events arrive in a particular order? In a distributed system, events can arrive out of order or be delivered more than once. Flag handlers that depend on sequencing (e.g., "created" before "updated") without explicit ordering logic or idempotency guards.
- **Schema migration safety** - for SQL CRUD services, are migration scripts additive and non-destructive (e.g., `ADD COLUMN`, not `DROP COLUMN` without a data migration step)? Are migrations compatible across all three supported database drivers (`mysql`, `pgx`, `mssql`)? Check for driver-specific statements prefixed with `-- DRIVER:`.

#### Step 7: Agentic Workflows

Skip this step if no microservice has `tasks` or `workflows` in its `manifest.yaml`. Otherwise, for each microservice that defines a workflow graph or task endpoint:

- **Foreman call sites (`foremanapi.NewClient`)** - grep every microservice for `foremanapi.NewClient`. The foreman is the execution engine, not a downstream dependency, so most call sites are one of three anti-patterns. **Flag each of these:**
  - A `define.Function` or `define.Web` handler that calls `foremanapi...Run` (or `Create` then `Await`) and returns the flow's result - a synchronous endpoint wrapping a workflow. It runs an open-ended flow on the caller's fixed request budget and couples an API provider to the foreman; wrong regardless of whether the workflow is this service's or another's. Fix: extract the logic into a plain Go helper the endpoint and the workflow's task body both call, or let the caller drive the foreman itself.
  - A `define.Task` handler that calls `foremanapi...Run`/`.Create` - composing a child workflow from a task must use `flow.Subgraph(otherflowapi.OtherFlow.URL(), input)` (return nil while `yield=true`, adopt `out` on re-entry). The foreman call spawns a detached flow that loses re-entry, cascading cancel, single-flow audit trail, and interrupt propagation.
  - A pass-through endpoint that just forwards to `foremanapi.Create/Run/Cancel/Snapshot/Await/Resume` (`Run<Workflow>`, `Cancel<Workflow>`, `<Workflow>Status`, ...). The code that triggers a workflow should call the foreman directly, not through a wrapper on the provider.
- **Legitimate launch sites - do not flag, but check duration handling** - a `foremanapi.Run`/`Create` call is correct in code that owns the *triggering event* and does not block a caller on the flow's result: a ticker, an inbound event sink, or a webhook/UI action that fires the flow and returns a handle or redirect. This launcher is ideally a microservice separate from the provider, but the same service is acceptable when it genuinely owns the trigger (its own ticker or UI). At these sites, verify duration is handled: an `Await` must handle the await timing out (poll-later handle, error, compensating cancel). There is no stop-notification event; for a follow-up that must happen reliably (a downstream call, a push, a compensation), the reaction belongs in the workflow as its own durable task - an orchestrating graph that runs the real work as a subgraph and routes success/failure to separate retryable tasks - not in a caller's `Await`. Flag a launch site that relies on a caller staying connected to observe the outcome of a genuinely long-running flow.
- **State hygiene at subgraph boundaries** - if a parent and a subgraph use different field names or one curates state at the boundary, look for a small upstream task that reshapes state with `flow.Delete` / `flow.Clear` / `flow.Transform`, or a small downstream task that drops internal scratch the parent doesn't want. A subgraph node with neither and a parent that obviously needs to reshape state (e.g., the parent stores `userQuestion` but the subgraph reads `prompt`) is a smell. The fix is one task at the boundary, not in-task state mutation scattered through the graph.
- **Reducer correctness at fan-in** - for every `graph.SetFanIn(node)`, check that every state field a branch writes is either replace-safe (last write wins is intended) or has an explicit `graph.SetReducer(field, reducer)` declared. The default is Replace; a branch that writes to a `messages` field expecting accumulation will silently lose siblings' contributions if no reducer is registered. Match the reducer to the semantic: `Append` for ordered arrays of deltas, `Add` for numeric sums, `Union` for sets, `Merge` for objects, `And`/`Or` for booleans, `Concat` for strings.
- **Task idempotency** - tasks may execute more than once for the same logical step: `flow.Retry`, foreman lease recovery, subgraph re-entry after the child completes, manual `Retry` via the foreman API. Tasks that fire external side effects (charging a card, sending an email, mutating a non-transactional store) must carry their own dedupe key or check first whether the effect has already happened. Pure-state tasks need no special treatment. Flag any task body that performs an external side effect without an obvious idempotency guard.
- **Long-running tasks free the worker** - any task that intrinsically takes more than the foreman's `TimeBudget` ceiling (default 2m, max 15m) must use the Interrupt/Resume pattern or the Sleep/Retry polling pattern, never block inside `time.Sleep` or a synchronous external wait. Grep tasks for long blocking calls without checking the return of `svc.Sleep` or for absence of `flow.Retry` / `flow.Interrupt` in polling shapes.
- **LLM tool exposure shape** - if the workflow uses `llmapi.ChatLoop` as a subgraph, the prepare task should produce `toolURLs []string` of canonical endpoint URLs (`calculatorapi.Arithmetic.URL()` style), not a pre-built `[]llmapi.Tool`. Pre-resolving `Tool` schemas in the caller duplicates what `InitChat` does internally and bypasses the actor-aware OpenAPI fetch that gates tools by `requiredClaims`. Flag any prepare task that constructs `llmapi.Tool` literals.
- **Naming convention** - microservices whose sole purpose is to host one workflow are conventionally named `<role>.agent` (e.g. `planner.agent`, `coordinator.agent`). Not a framework requirement, but flag drift from the convention if a project has adopted it elsewhere.

If the application includes a workflow that calls an LLM, also verify that `foreman.core`, `llm.core`, and an LLM provider (e.g. `claudellm.core`) are added to `main/main.go`. The workflow compiles fine without them, but execution fails at runtime with an `ack timeout` on whichever endpoint is missing.

#### Step 8: Observability

Check for adequate instrumentation by reviewing `manifest.yaml` metrics sections and `service.go`:

- **Metrics coverage** - are key business operations instrumented with custom metrics (counters, gauges, or histograms) defined in the manifest? Flag important operations (payments, user actions, external API calls) that lack metrics.
- **Dead metrics** - are metrics defined in the manifest actually recorded in the code? Grep for each metric's `Increment` or `Record` function in `service.go`. Flag metrics that are defined but never recorded.
- **Histogram buckets** - are `buckets` appropriate for histogram metrics? Flag buckets that don't cover the expected range of values (e.g., latency buckets that max out at 10ms for an operation that routinely takes seconds).
- **Logging** - is `svc.LogInfo/Warn/Error/Debug` used with structured `slog`-style name=value pairs? Is the `"error"` label used when logging errors? Grep for `fmt.Print`, `log.Print`, or `println` to flag non-framework logging.
- **Context propagation** - is `ctx` passed through the entire call chain, from handler to downstream calls? Flag functions that create a new `context.Background()` or `context.TODO()` instead of propagating the received context, as this breaks distributed tracing.
- **Error tracing** - are all returned errors wrapped with `errors.Trace`? Are new errors created with `errors.New` (not `fmt.Errorf`)? Grep for `return err` (without `errors.Trace`) and `fmt.Errorf` to find violations.

#### Step 9: Security

Review authentication, authorization, and secrets management:

- **Authentication at the edge** - is the HTTP ingress proxy configured with authorization middleware to validate and exchange tokens? Check for the authorization middleware in the HTTP ingress setup.
- **Authorization per endpoint** - do endpoints that handle sensitive operations (mutations, private data access, admin functions) specify `requiredClaims` in their manifest? Flag sensitive endpoints without `requiredClaims`.
- **Stale issuer predicates** - flag `iss=~"access.token.core"` (or `iss==`) predicates inside `requiredClaims` as redundant. Issuer verification is enforced at the framework layer via JWKS pinning - the connector rejects any token whose `iss` does not match a framework-pinned hostname before signature verification. The predicate is no longer load-bearing; recommend removal.
- **Claims expression correctness** - are `requiredClaims` boolean expressions syntactically correct and logically sound? Check for proper use of `&&`, `||`, `!`, comparison operators, and regex matching. Flag overly permissive expressions (e.g., `role=="admin" || true`).
- **Input validation** - are user-provided inputs validated before processing? Flag endpoints that pass user input directly into SQL queries (injection risk), file paths (traversal risk), or HTML output (XSS risk) without sanitization.
- **Secrets management** - are secret config properties marked `secret: true` in the manifest? Are secret values placed in `config.local.yaml` (git-ignored) and not in `config.yaml` or hardcoded in source? Grep for patterns that look like hardcoded credentials (API keys, passwords, connection strings).

#### Step 10: Operational Concerns

Evaluate deployment, configuration, and production readiness:

- **Startup group ordering** - in `main/main.go`, are microservices added to the application in the correct startup group order? Services must start after their dependencies. Verify against the dependency graph from Step 3.
- **Configuration externalization** - is behavior tunable via config properties without redeployment? Are config properties defined with appropriate `validation` rules in the manifest?
- **Appropriate defaults** - do config properties have sensible defaults? Flag configs with no default that could cause startup failures or unexpected behavior.
- **Resource lifecycle** - are resources (database connections, caches, external clients) initialized in `OnStartup` and cleaned up in `OnShutdown`? Flag resources that are initialized inline (e.g., in a handler) without cleanup.
- **Ticker intervals** - are ticker intervals appropriate for their purpose? Flag tickers that run too frequently (sub-second) or too infrequently for their stated goal.
- **Embedded resources** - are static files properly placed in `resources/` and accessed via `svc.ReadResFile` or `svc.ResFS`?
- **Test coverage** - does each microservice have integration tests in `service_test.go`? Do tests use `application.New()` with `RunInTest(t)`? Are mocks provided for downstream dependencies that are unavailable in the test environment?
- **No TODO leftovers** - grep for `TODO`, `FIXME`, `HACK`, and `XXX` comments across the codebase. Flag any that indicate unfinished work or known issues that should be resolved before production.
- **Version tracking** - is the `Version` const in each microservice's `myserviceapi/definition.go` being incremented when changes are made?

#### Step 11: Documentation

Evaluate whether the system is sufficiently documented for a new developer or agent to understand and safely evolve it:

- **CLAUDE.md quality** - does each microservice have an `CLAUDE.md` that explains its purpose, design rationale, tradeoffs, and the context needed to evolve it? Flag microservices with missing, empty, or boilerplate-only `CLAUDE.md` files. The file should capture *why* decisions were made, not just describe what the code does.
- **Manifest completeness** - does each `manifest.yaml` have meaningful `description` fields for the service, its endpoints, configs, events, and tickers? Flag entries with missing or placeholder descriptions.
- **Godoc comments** - are exported types, functions, and methods in `service.go` and `*api/client.go` documented with Godoc comments? Flag exported symbols that lack comments, especially public API surfaces in `*api` packages that downstream consumers rely on.
- **Complex logic** - are non-obvious algorithms, business rules, or workarounds explained with inline comments? Flag complex code blocks (deeply nested logic, regex patterns, SQL queries with many joins/conditions) that lack explanatory comments.
- **Config documentation** - are config properties documented with clear descriptions in the manifest? Would a system operator understand what each config does, what values are valid, and what the default means?

#### Step 12: Produce the Report

Compile findings into a structured report. For each category (Steps 2-11), list:

- **Findings** - specific observations, both positive and negative, with file paths and line numbers where applicable
- **Recommendations** - actionable suggestions for improvement, ordered by severity

Use severity levels:
- **Critical** - architectural issues that could cause system-wide failures, data loss, or security vulnerabilities
- **Warning** - design concerns that may lead to problems at scale or during evolution
- **Info** - minor improvements or deviations from best practices

Omit categories where no issues were found, but mention them in the summary as areas that passed review.

Format the report as follows:

```markdown
# Architectural Review

## Summary
Brief overview of the system, number of microservices reviewed, and high-level assessment.
Note categories that passed review without issues.

## Service Boundaries and Design
### Findings
- ...
### Recommendations
- [severity] ...

## Dependencies and Coupling
### Findings
- ...
### Recommendations
- [severity] ...

(repeat for each category that has findings)

## Conclusion
Overall assessment and prioritized action items.
```

Present the report to the user.
