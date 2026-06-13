# dynamicsubgraphflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the **dynamic subgraph** control signal: `flow.Subgraph(url, input)`. A parent task calls `flow.Subgraph()` at runtime to launch a child workflow. The foreman parks the parent step, runs the child to completion, then **re-executes the parent task** - on re-entry the same `flow.Subgraph` call returns the child's `final_state` as `out` (with `yield=false`), which the parent adopts explicitly. The child output is *not* merged into the parent's state.

All subgraphs are invoked this way since v1.37.0; the static `graph.AddSubgraph` node type was retired. Other
subgraph fixtures (`subgraphflow`, `subgraphentryflow`, `subgraphfanoutflow`, `nestedfanoutflow`, `soakflow`) all use
the same `flow.Subgraph` pattern - this one is the minimal single-task focus case.

## Parent re-entry pattern

The parent task arms the subgraph on its first invocation and consumes the result on re-entry. The framework-managed `subgraph_done` flag (surfaced as the `yield` return) distinguishes the two passes - the parent does not encode a re-entry sentinel in state:

```go
func (svc *Service) Parent(ctx context.Context, flow *workflow.Flow, value int) (parentResult string, err error) {
    out, yield, err := flow.Subgraph(InnerURL, map[string]any{"value": value})
    if err != nil {
        return "", errors.Trace(err)
    }
    if yield {
        return "", nil // first pass: parked, child running
    }
    result := 0
    if v, ok := out["innerResult"].(float64); ok { // JSON-decoded: numbers are float64
        result = int(v)
    }
    return fmt.Sprintf("parent:%d", result), nil
}
```

## Patterns exercised

- `flow.Subgraph(url, input)` returning `(out, yield, err)`; the parent adopts fields from `out` explicitly
- Parent task re-execution after child completion (parked then re-enqueued)
- Child workflow registered as a normal Workflow endpoint (no static subgraph node on the parent graph)
- Re-entry detection via the framework's `subgraph_done` flag (the `yield` return), not a child-set state field
