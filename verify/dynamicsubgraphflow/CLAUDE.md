# dynamicsubgraphflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **dynamic subgraph** control signal: `flow.Subgraph(url, input)`. A parent task calls `flow.Subgraph()` at runtime to launch a child workflow. The foreman parks the parent step, runs the child to completion, merges the child's `DeclareOutputs` fields into the parent step's state, then **re-executes the parent task** with the merged state in scope.

This is distinct from `subgraphflow.verify`, which uses **`graph.AddSubgraph`** (the static form, where the child workflow is a declared node in the parent graph and the parent task never re-executes).

## Parent re-entry pattern

The parent task must idempotently distinguish its first invocation from the re-entry after the subgraph completes. It does so by reading a state field that only the subgraph sets:

```go
func (svc *Service) Parent(ctx context.Context, flow *workflow.Flow, value int, innerDone bool, innerResult int) (parentResult string, err error) {
    if !innerDone {
        flow.Subgraph(InnerURL, map[string]any{"value": value})
        return "", nil
    }
    return fmt.Sprintf("parent:%d", innerResult), nil
}
```

The child's `DeclareOutputs("innerDone", "innerResult")` makes those fields cross back into the parent's state on completion.

## Patterns exercised

- `flow.Subgraph(url, input)` as a control signal returned via the response payload
- Parent task re-execution after child completion (parked then re-enqueued)
- Child workflow registered as a normal Workflow endpoint (no `AddSubgraph` on the parent graph)
- Child outputs filtered through child's `DeclareOutputs` before merging into parent state
- Idempotent re-entry detection via a child-set state field
