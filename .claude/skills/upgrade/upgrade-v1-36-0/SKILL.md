---
name: upgrade-v1-36-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.35.x to v1.36.0. Removes graph.DeclareInputs / graph.DeclareOutputs and workflow.FilterState (subgraph boundaries are now pass-through). Workflow authors who need narrower contracts at a subgraph boundary bracket the subgraph with Before<NodeName> / After<NodeName> adapter tasks using the new flow.Delete / flow.Clear / flow.Keep / flow.Transform primitives (see .claude/rules/workflows.txt for the convention).
---

## Removed APIs

- `graph.DeclareInputs(...)`, `graph.DeclareOutputs(...)`, `graph.Inputs()`, `graph.Outputs()`
- `workflow.FilterState`

Stored graph JSON's `inputs` / `outputs` keys are silently ignored by the new unmarshaler. No schema migration.

## Workflow

```
Upgrade a Microbus project to v1.36.0:
- [ ] Step 1: Delete graph.DeclareInputs / graph.DeclareOutputs calls
- [ ] Step 2: Delete references to workflow.FilterState / graph.Inputs() / graph.Outputs()
- [ ] Step 3: For subgraphs whose old declarations were narrow (not "*"), add Before<NodeName> / After<NodeName> adapter tasks
- [ ] Step 4: Regenerate mocks (genmock)
- [ ] Step 5: Regenerate workflow mermaid diagrams (genworkflowmmd)
- [ ] Step 6: Regenerate manifests (genmanifest) and run go vet ./... && go test ./...
```

#### Step 1: Delete `DeclareInputs` / `DeclareOutputs` Calls

```bash
grep -rln "graph\.DeclareInputs\|graph\.DeclareOutputs\|g\.DeclareInputs\|g\.DeclareOutputs" --include="*.go" .
```

Mechanical sweep:

```bash
grep -l "DeclareInputs\|DeclareOutputs" --include="*.go" -r . \
    | xargs perl -i -ne 'print unless /^\s*(graph|g)\.Declare(Inputs|Outputs)\(/'
```

Existing `mock.go` files emit these too; Step 4 rewrites them.

#### Step 2: Delete References to Removed Helpers

```bash
grep -rn "workflow\.FilterState\|graph\.Inputs()\|graph\.Outputs()" --include="*.go" .
```

Delete each match. These were only used to plumb the removed declarations.

#### Step 3: Add `Before<NodeName>` / `After<NodeName>` Adapters Where Old Declarations Were Narrow

Skip this step for `DeclareInputs("*")` / `DeclareOutputs("*")` - pass-through was already the effective behavior.

For each subgraph whose old declaration named specific fields, decide whether the broader exchange is now fine. If not, scaffold a `Before<SubgraphNodeName>` task (upstream of the subgraph) and/or `After<SubgraphNodeName>` task (downstream) using `add-task`. The adapter body uses `flow.Keep` / `flow.Delete` / `flow.Transform` to reshape state. See "State Transformation Around a Subgraph" in `.claude/rules/workflows.txt` for the convention and full examples.

Wire the adapters into the graph around the subgraph node.

#### Step 4: Regenerate Mocks

```bash
for d in $(find . -name "mock.go" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genmock --path "$d"
done
```

`genmock` no longer emits `g.DeclareInputs("*")` / `g.DeclareOutputs("*")`. Idempotent.

#### Step 5: Regenerate Workflow Mermaid Diagrams

```bash
for d in $(find . -name "*.mmd" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genworkflowmmd -path "$d"
done
```

Any adapter tasks added in Step 3 appear as new nodes; expected.

#### Step 6: Regenerate Manifests and Verify the Build

From each microservice directory:

```bash
go run github.com/microbus-io/fabric/cmd/genmanifest --path .
```

`genmanifest` bumps `frameworkVersion` to `1.36.0`. Then from the project root:

```bash
go vet ./...
go test ./...
```

A subgraph-using test that previously asserted a field was filtered out (e.g. `assert.False(_, hasInput)`) is the behavior change landing. Flip the assertion to expect the field present, or add a Step 3 adapter to mimic the old filter.
