---
name: upgrade-v1-42-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.41.x to v1.42.0. Enforces one principle - a task is not an independent call unit; it is only ever a node in a graph, and the independently-invocable engine-backed unit is the workflow. Three things are removed and one client is renamed. (1) The generated subflow client is renamed Subflow -> Subgraph and NewSubflow -> NewSubgraph, and it now carries ONLY workflow methods - the per-task methods are gone, since a task can no longer be invoked as an isolated child flow. The mechanical rename NewSubflow( -> NewSubgraph( fixes the common case (calls that targeted a workflow); regeneration (orchestrator Step 6) rebuilds each *api/client.go to match. (2) flow.Subtask is removed from the dwarf workflow package. (3) The foreman CreateTask endpoint/client is removed. The migration is a pure-shell find/replace for the rename, plus grep-guided manual rewrites for the three removed call shapes (a subflow call that targeted a task, flow.Subtask, foreman CreateTask) - each migrates to a one-node workflow invoked via Subgraph/Run, or to graph composition. No genupgrade tool is involved. All breaks are loud compile errors surfaced by the orchestrator's single go vet pass.
---

## What changed

v1.42.0 enforces a single principle: **a task is not an independent call unit.** A task is only ever a node in a
graph; the independently-invocable, engine-backed unit is the workflow. Every mechanism that ran a bare task as a
standalone unit is removed.

- **The generated subflow client is renamed and loses its task methods.** `Subflow` becomes `Subgraph` and
  `NewSubflow` becomes `NewSubgraph`. The client now carries **only workflow methods** - the per-task methods are
  gone, because a task can no longer be invoked as an isolated child flow. The common call site
  `otherapi.NewSubflow(flow).SomeWorkflow(ctx, ...)` becomes `otherapi.NewSubgraph(flow).SomeWorkflow(ctx, ...)`.
- **`flow.Subtask` is removed** from the dwarf `workflow` package. There is no longer a way to run a single task as
  a synthesized one-node child flow.
- **The foreman `CreateTask` endpoint and client are removed** (`foremanapi.CreateTask` and the
  `Client`/`MulticastClient` `CreateTask` methods).

The replacement for all three is the same: to run one unit of work in isolation, declare a **one-node workflow**
and invoke it (as a subgraph from inside a task body, or via the foreman's `Run`/`Create` from a caller). When the
callee is your own microservice and the work belongs to the calling flow, make the task a **node in the calling
graph** instead.

Nothing changes at runtime for code that already composes via graphs and workflow subgraphs - this is an API
removal plus a client rename.

## Workflow

```
Upgrade a Microbus project to v1.42.0:
- [ ] Step 1: Rename the subflow client (mechanical)
- [ ] Step 2: Migrate the removed task-as-unit call shapes (grep-guided)
```

Regeneration and verification are **not** part of this skill - the `upgrade-microbus` orchestrator runs
`genservice` and `go mod tidy && go vet ./... && go test ./...` once, after every numbered skill has applied its
source transformation. That regeneration rebuilds each `*api/client.go` so its generated `Subflow` type becomes
`Subgraph` with workflow-only methods; the `go vet` pass surfaces any call site this skill's manual step missed.

#### Step 1: Rename the Subflow Client (Mechanical)

Replace the constructor call across all hand-written Go source. This fixes every subflow call that targeted a
**workflow** (the common case):

```bash
grep -rl --include='*.go' --exclude-dir=vendor 'NewSubflow(' . \
  | while read -r f; do
      perl -i -pe 's/\bNewSubflow\(/NewSubgraph(/g' "$f"
    done
```

A bare reference to the type itself (e.g. `var sf otherapi.Subflow` - rare) renames the same way, `Subflow` ->
`Subgraph`. These are uncommon; if any exist they surface as a compile error in the orchestrator's `go vet` and
rename mechanically. Do not blindly sed the bare word `Subflow` repo-wide - it can appear in unrelated identifiers
and comments; rename only a confirmed `otherapi.Subflow` type reference.

The generated `*api/client.go` files also contain `Subflow`/`NewSubflow`; leave them to the orchestrator's
regeneration in Step 6 (it re-emits them as `Subgraph` from `definition.go`).

#### Step 2: Migrate the Removed Task-as-Unit Call Shapes (Grep-Guided)

Three call shapes are removed and have no mechanical rewrite, because the correct replacement depends on intent.
Find each and migrate it; when the right shape is unclear, ask the user rather than guessing.

**(a) A subflow call that targeted a task.** After Step 1, `otherapi.NewSubgraph(flow).SomeTask(ctx, ...)` does not
compile - the renamed client has no task methods. Find candidates (a subgraph-client call whose method is a task,
not a workflow); the orchestrator's `go vet` also reports these as `NewSubgraph(...).SomeTask undefined`:

```bash
grep -rn --include='*.go' --exclude-dir=vendor 'NewSubgraph(' .
```

Migrate by making the work a workflow: the callee microservice exposes the task as the single node of a one-node
workflow, and the caller invokes `otherapi.NewSubgraph(flow).ThatWorkflow(ctx, ...)`. If instead the task belongs
to the calling flow, add it as a node in the calling graph and route to it with a transition - no subgraph at all.

**(b) `flow.Subtask`.** Removed. Find every call:

```bash
grep -rn --include='*.go' --exclude-dir=vendor 'Subtask(' .
```

Migrate to a subgraph of a one-node workflow: declare a workflow whose graph binds the single task as its only
node, then call `flow.Subgraph(thatWorkflowURL, in, &out)` (or the typed `NewSubgraph(flow).ThatWorkflow(...)`).
If the task is part of the same workflow, make it a graph node instead.

**(c) Foreman `CreateTask`.** Removed. Find every call:

```bash
grep -rn --include='*.go' --exclude-dir=vendor 'CreateTask(' .
```

Migrate to a one-node workflow run through the foreman: declare a workflow that runs the task as its only node,
then `foremanapi.NewClient(svc).Run(ctx, thatWorkflow.URL(), initialState, opts)` (or `Create` + `Start` +
`Await` for fine-grained control). The `name` argument that `CreateTask` took becomes the node/graph name in the
workflow definition.

After Step 2 the project still does not compile (generated boilerplate is stale and the dwarf dependency has not
been re-resolved); that is expected. The orchestrator's final step regenerates each microservice's boilerplate,
runs `go mod tidy`, and verifies the whole project with `go vet ./...` and `go test ./...`.
