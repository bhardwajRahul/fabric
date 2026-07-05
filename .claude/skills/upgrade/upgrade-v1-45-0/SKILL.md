---
name: upgrade-v1-45-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.44.x to v1.45.0. One breaking change to the workflow/foreman surface - flow-stop notification is removed. The embedded dwarf engine no longer has a stop-notification callback, so foreman drops FlowOptions.NotifyOnStop (the option field) and the foreman OnFlowStopped outbound event and its NewHook/NewMulticastTrigger methods. Any code that set NotifyOnStop or subscribed to OnFlowStopped no longer compiles (loud errors). There is no safe mechanical fix - the replacement is a per-call-site design choice: block on foremanapi Await to learn a flow's outcome when a caller is standing by, or, for a follow-up that must happen reliably (fire-and-forget submit, push notification, downstream call, compensation), move the reaction into the workflow itself as an orchestrating graph that runs the real work as a subgraph and routes success/failure to separate durable tasks. Grep-guided manual migration; ask the user when the right shape is unclear. No genupgrade tool is involved; the orchestrator's single go vet/test pass surfaces every unmigrated site.
---

## What changed

v1.45.0 removes flow-stop notification from the workflow subsystem. The embedded dwarf engine dropped its
stop-notification callback entirely (notification is now ordinary workflow authoring, not an engine feature), and
the foreman adapter dropped the surface that fronted it:

- **`workflow.FlowOptions.NotifyOnStop` is removed.** The option field no longer exists, so a
  `&workflow.FlowOptions{NotifyOnStop: true}` literal is a compile error (unknown field).
- **The foreman `OnFlowStopped` outbound event is removed.** `foremanapi.OnFlowStopped`, the
  `NewHook(...).OnFlowStopped(...)` subscription, and the `NewMulticastTrigger(...).OnFlowStopped(...)` trigger are
  all gone. A subscriber no longer compiles.

Both are loud compile errors. There is **no mechanical rewrite**, because "opt into a fire-once event" had no
one-to-one replacement - the two supported ways to learn a flow's outcome are a design choice per call site:

1. **`foremanapi.Await`** - block a live caller until the flow stops and read the `*workflow.FlowOutcome`. Right
   when a caller is standing by for the result (a request/response handler that holds the flowKey, a test).
2. **Orchestration** - model the follow-up as tasks inside a workflow that runs the real work as a subgraph and
   routes success/failure to separate tasks, each a durable, independently retried step. Right when the reaction
   must happen reliably regardless of who is (or isn't) waiting: a fire-and-forget submit, a push notification, a
   downstream call, a compensation. This is strictly more reliable than the removed event, which reached only a
   receiver live on the bus at the instant of the stop.

The full rationale and the orchestrating-graph shape are in `.claude/rules/workflows.txt` under "Detecting Flow
Completion".

## Workflow

```
Upgrade a Microbus project to v1.45.0:
- [ ] Step 1: Remove FlowOptions.NotifyOnStop and choose how the caller learns the outcome (grep-guided)
- [ ] Step 2: Replace OnFlowStopped subscriptions and triggers (grep-guided)
```

Regeneration and verification are **not** part of this skill - the `upgrade-microbus` orchestrator runs
`genservice` and `go mod tidy && go vet ./... && go test ./...` once, after every numbered skill has applied its
source edits. The tree will not compile between steps; that is expected. The final `go vet` pass reports every
site this skill missed. A project that never set `NotifyOnStop` and never subscribed to `OnFlowStopped` needs no
change from this skill.

#### Step 1: Remove `FlowOptions.NotifyOnStop` and Choose How the Caller Learns the Outcome (Grep-Guided)

Find every site that set the flag:

```bash
grep -rn --include='*.go' --exclude-dir=vendor 'NotifyOnStop' .
```

For each hit, delete the `NotifyOnStop` field from the `FlowOptions` literal (if it was the only field, the whole
`&workflow.FlowOptions{...}` may become `nil`), then decide how that caller now learns the outcome. The right
choice depends on why it wanted the notification - do not guess; when it is unclear, ask the user.

- **A caller that was standing by for the result** (it holds, or can hold, the flowKey and can wait): switch to
  `Await`. `Create` auto-runs and returns the key, so:
  ```go
  // before
  flowKey, err := foremanapi.NewClient(svc).Create(ctx, url, initialState,
      &workflow.FlowOptions{NotifyOnStop: true})
  // ... later, react in an OnFlowStopped handler ...

  // after
  flowKey, err := foremanapi.NewClient(svc).Create(ctx, url, initialState, nil)
  outcome, err := foremanapi.NewClient(svc).Await(ctx, flowKey)
  // react to outcome.Status / outcome.State / outcome.Error / outcome.InterruptPayload here
  ```
  `Await` returns without the outcome if the caller's context deadline fires first while the flow keeps running;
  a caller that may outlive its request budget must be prepared to `Await` again (it still holds the key).

- **A follow-up that must happen reliably** regardless of who is waiting (fire-and-forget submit, a push
  notification, a downstream call, a compensation): the reaction belongs in the workflow, not in a caller. Author
  an orchestrating graph whose entry task launches the real work as a subgraph and whose success/failure tasks do
  the follow-up as their own durable, retryable steps (`AddTransition` for success, `AddTransitionOnError` for
  failure). The `add-workflow` and `add-task` skills scaffold this; the shape is in `.claude/rules/workflows.txt`
  under "Detecting Flow Completion". This is the guaranteed-delivery replacement for the removed event.

#### Step 2: Replace `OnFlowStopped` Subscriptions and Triggers (Grep-Guided)

Find every reference to the removed event:

```bash
grep -rn --include='*.go' --exclude-dir=vendor 'OnFlowStopped' .
```

Each hit is one of:

- **A subscription** - `foremanapi.NewHook(svc).ForHost(...).OnFlowStopped(func(ctx, flowKey string, outcome
  *workflow.FlowOutcome) error { ... })`. Delete the subscription and relocate its handler body to wherever the
  outcome is now learned in Step 1: inline after the `Await` call (standing-by caller), or into the workflow's
  success/failure task (reliable follow-up). The handler's logic is preserved; only its trigger moves.
- **A trigger** - `foremanapi.NewMulticastTrigger(svc).ForHost(...).OnFlowStopped(...)`. Only the foreman itself
  fired this event, so a downstream project should not have one; if it does, it was reaching into foreman
  internals and the call must be removed.
- **A leftover handler method or hook type** left unused once its subscription is gone. Remove the dead code; the
  orchestrator's `go vet` flags an unused function only if it is unexported and otherwise unreferenced.

After both steps the project still will not compile until the orchestrator regenerates boilerplate, re-resolves
the dependency, and runs its verification pass. Any `NotifyOnStop` or `OnFlowStopped` reference the greps missed
surfaces there as an unknown-field or undefined-symbol error.
