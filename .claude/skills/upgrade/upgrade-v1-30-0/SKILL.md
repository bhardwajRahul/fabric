---
name: upgrade-v1-30-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.29.x to v1.30.0. Renames graph error/timeout transitions, switches workflow graphs to named nodes with explicit SetFanIn, replaces sub.Infra/Ultra with sub.Manual/Automatic (semantics changed), renames Subscription.Route to Path, changes the httpingress AllowedOrigins default, and regenerates every mock with the new cmd/genmock tool.
---

## Background

- `workflow.Graph.AddErrorTransition` renamed to `AddTransitionOnError`. Pure rename, semantics unchanged.
- New `workflow.Graph.AddTransitionOnTimeout(from, to)` fires only on HTTP 408 errors; wins over a catch-all `OnError` from the same node.
- Validator now rejects `OnError`/`OnTimeout` self-loops (`from == to`). Use `flow.Retry` / `flow.RetryOnTimeout` in the task body for retry semantics.
- New `flow.RetryOnTimeout(err, ...)` and `flow.RetryNowOnTimeout(err)` helpers retry only when the error carries HTTP 408.
- **Named graph nodes.** `AddTask(url)` becomes `AddTask(name, url)`; `AddSubgraph(name)` becomes `AddSubgraph(name, url)`. Single-string callers still compile via auto-registration (name == url), but new graphs should pass distinct names so Mermaid output, `URLOf`, and the lineage validator's aliasing escape hatch work.
- **Lineage fan-in via `SetFanIn`.** The old "fan-out siblings must converge to the same successors" rule is replaced with `graph.SetFanIn(name)` plus a lineage-stack validator. Graphs with no fan-out are unaffected.
- `sub.Infra()` / `sub.Ultra()` removed and replaced by `sub.Manual()` / `sub.Automatic()`. **Semantics changed**: `Infra` activated *before* `OnStartup`; `Manual` does not auto-activate at all. Caller code must invoke `Connector.ActivateSubscription(name)` once the backing resource is ready.
- New `sub.Tag(...)` / `sub.Untag(...)` options and `Connector.Subscriptions()` snapshot enable bulk operations like "activate every Python-backed sub."
- `Subscription.Route` field renamed to `Path`. `SubscriptionInfo.Path` is the snapshot equivalent.
- `coreservices/httpingress` `AllowedOrigins` default changed from `"*"` to `""`. Empty pins ACAO to the request's own scheme://host (same-origin only).
- New `cmd/genmock` tool emits both `mock.go` and a companion `mock_test.go`. Hand-written `TestXxx_Mock` blocks in `service_test.go` must be deleted to make room. Unmocked methods now return zero values (including `nil` error) instead of `501 Not Implemented`.
- New external module `github.com/microbus-io/pyvenv` provides Python integration; opt-in only.

## Workflow

```
Upgrade a Microbus project to v1.30.0:
- [ ] Step 1: Rename AddErrorTransition to AddTransitionOnError
- [ ] Step 2: Remove self-loop OnError transitions
- [ ] Step 3: Migrate sub.Infra/Ultra to sub.Manual/Automatic
- [ ] Step 4: Rename Subscription.Route reads to Path
- [ ] Step 5: Adopt named graph nodes and SetFanIn
- [ ] Step 6: Review httpingress AllowedOrigins
- [ ] Step 7: Delete legacy TestXxx_Mock from service_test.go
- [ ] Step 8: (mock + manifest regeneration deferred to the orchestrator)
```

#### Step 1: Rename `AddErrorTransition` to `AddTransitionOnError`

```bash
grep -rn "AddErrorTransition" --include="*.go" --include="*.md" .
```

Rewrite each call site. Signature and behavior are identical.

#### Step 2: Remove Self-Loop OnError Transitions

```bash
grep -rn "AddTransitionOnError\|AddTransitionOnTimeout" --include="*.go" .
```

For each call where `from == to`, drop the transition and move retry semantics into the task body using `flow.Retry(...)` or `flow.RetryOnTimeout(err, ...)`. The validator now rejects self-loops.

#### Step 3: Migrate `sub.Infra` / `sub.Ultra` to `sub.Manual` / `sub.Automatic`

```bash
grep -rn "sub\.Infra\b\|sub\.Ultra\b\|\.Infra\b" --include="*.go" --include="*.md" --include="*.txt" .
```

Replace `sub.Infra()` with `sub.Manual()` and `sub.Ultra()` with `sub.Automatic()`. Replace `Subscription.Infra` reads with `Subscription.Manual`.

The behavior is **not** the same. `Infra` activated the sub before `OnStartup` ran; `Manual` does not activate it at all. For each migrated sub, decide whether the caller now needs to invoke `svc.ActivateSubscription(name)` from `OnStartup` (or wherever the backing resource becomes ready). For grouped activation, attach a `sub.Tag("groupname")` to each related subscription and iterate `svc.Subscriptions()` filtered by tag.

#### Step 4: Rename `Subscription.Route` Reads to `Path`

```bash
grep -rn "\.Route\b" --include="*.go" . | grep -i "sub\.\|Subscription\|SubscriptionInfo"
```

Replace each `s.Route` read on a `*sub.Subscription` or `SubscriptionInfo` with `s.Path`. The wire route inside `sub.At(method, route)` and `sub.Route(route)` option constructors is unchanged.

#### Step 5: Adopt Named Graph Nodes and `SetFanIn`

Existing graphs that pass URLs as the sole argument to `AddTask`/`AddSubgraph` and as transition endpoints continue to compile via auto-registration. No action required for those graphs to keep running.

For graphs that use fan-out (multiple non-goto/non-error transitions out of one node, or any `AddTransitionForEach`):

1. Convert each `graph.AddTask(url)` to `graph.AddTask(name, url)` with a short camelCase name; rewrite transitions to use the names.
2. Mark every fan-in node with `graph.SetFanIn(name)`. Required for: empty-cohort handling on `forEach`, multi-stage per-element pipelines, and aliased re-entry. Once the graph declares any `SetFanIn` it opts into the lineage validator, which rejects branches that don't converge through their fan-in.
3. To reuse the same task at multiple positions, call `AddTask("posA", url)` and `AddTask("posB", url)` - both names dispatch to the same URL but the validator treats them as distinct positions.

Worked examples live in `examples/creditflow` and the `verify/*flow` microservices.

#### Step 6: Review httpingress `AllowedOrigins`

The default changed from `"*"` to `""` (empty pins ACAO to the request's own origin; same-origin only). Concrete origins and `"*"` still work as before.

Check `config.yaml` / `config.local.yaml` and `env.yaml` / `env.local.yaml` for an explicit `AllowedOrigins` value on `http.ingress.core`. If unset and the project relied on `"*"`, list the affected files and ask the developer whether to restore `"*"` explicitly or accept same-origin only. Do not auto-edit configuration.

#### Step 7: Delete Legacy `TestXxx_Mock` From `service_test.go`

`cmd/genmock` now emits `mock_test.go` containing the per-microservice mock smoke test. The hand-written function in `service_test.go` would collide.

Delete every top-level `Test.*_Mock` function (no receiver) from each `service_test.go`. Run `goimports -w` on the modified files to drop newly-unused imports.

#### Step 8: Defer Mock and Manifest Regeneration

The named-node and `mock_test.go` changes require regenerating each microservice's `mock.go`, `mock_test.go`, and
`manifest.yaml`. Do **not** run a generator here - the `upgrade-microbus` orchestrator regenerates every
microservice's boilerplate from source and verifies the whole project once, after every numbered skill has run.
Step 7 (deleting the legacy `TestXxx_Mock`) is still done here, by hand, so the regenerated `mock_test.go` does not
collide.
